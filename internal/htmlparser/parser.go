package htmlparser

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"bullwler/internal/helpers"
	"bullwler/internal/report"

	"golang.org/x/net/html"
)

// AnalyzeNode - функция для анализа DOM элемента (ноды)
func AnalyzeNode(n *html.Node, r *report.SEOReport, labelForMap map[string]bool) {
	if n.Type == html.ElementNode {
		processElement(n, r, labelForMap)
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		AnalyzeNode(c, r, labelForMap)
	}
}

func processElement(n *html.Node, r *report.SEOReport, labelForMap map[string]bool) {
	tag := n.Data

	switch tag {
	case "meta":
		handleMeta(n, r)
	case "title":
		if text := helpers.GetText(n); text != "" {
			r.Title = strings.TrimSpace(text)
		}
	case "link":
		if rel := helpers.GetAttr(n, "rel"); rel == "canonical" {
			r.HasCanonical = true
		}
	case "script":
		handleScript(n, r)
	case "header":
		r.HasHeader = true
	case "nav":
		r.HasNav = true
	case "main":
		r.HasMain = true
	case "article":
		r.HasArticle = true
	case "section":
		r.HasSection = true
	case "footer":
		r.HasFooter = true
	case "img":
		handleImage(n, r)
	case "button":
		if helpers.GetAttr(n, "type") == "" {
			r.InvalidButtons++
		}
	case "a":
		handleLink(n, r)
		if href := helpers.GetAttr(n, "href"); href != "" {
			if absURL := resolveURL(r.URL, href); absURL != "" {
				r.AllLinks = append(r.AllLinks, absURL)
			}
		}
	case "form":
		handleForm(n, r)
	case "input", "textarea", "select":
		handleInput(n, r, labelForMap)
	case "label":
		if forAttr := helpers.GetAttr(n, "for"); forAttr == "" {
			r.LabelsWithoutFor++
		}
	}

	// ARIA
	for _, attr := range n.Attr {
		switch attr.Key {
		case "aria-label":
			r.AriaLabels++
		case "aria-labelledby":
			r.AriaLabelledBy++
		case "role":
			r.Roles++
		}
	}

	// Headings
	if len(tag) == 2 && tag[0] == 'h' && tag[1] >= '1' && tag[1] <= '6' {
		r.HeadingCounts[tag]++
		if text := helpers.GetText(n); text != "" {
			cleanText := strings.TrimSpace(text)
			if cleanText != "" {
				r.HeadingTexts[tag] = append(r.HeadingTexts[tag], cleanText)
			}
		}
		r.HeadingsSequence = append(r.HeadingsSequence, tag)
	}

	// Microdata / RDFa
	if _, exists := helpers.GetAttrExists(n, "itemscope"); exists {
		r.HasMicrodata = true
		if itemType, exists := helpers.GetAttrExists(n, "itemtype"); exists && itemType != "" {
			r.MicrodataTypes = append(r.MicrodataTypes, extractSchemaTypes(itemType)...)
		}
	}
	if vocab, exists := helpers.GetAttrExists(n, "vocab"); exists && vocab != "" {
		r.HasRDFa = true
		r.RDFaVocabularies = append(r.RDFaVocabularies, vocab)
	} else if _, exists := helpers.GetAttrExists(n, "typeof"); exists {
		r.HasRDFa = true
	}
}

func handleMeta(n *html.Node, r *report.SEOReport) {
	name, prop, content := helpers.GetMetaAttrsFull(n)
	if strings.HasPrefix(prop, "og:") {
		r.OG[strings.TrimPrefix(prop, "og:")] = content
	}
	if strings.HasPrefix(name, "twitter:") {
		r.Twitter[strings.TrimPrefix(name, "twitter:")] = content
	}
	if (name == "description" || prop == "description") && r.Description == "" {
		r.Description = content
	}
	if name == "viewport" {
		r.HasViewport = true
	}
}

func handleScript(n *html.Node, r *report.SEOReport) {
	if typ := helpers.GetAttr(n, "type"); typ == "application/ld+json" {
		r.HasJSONLD = true
		content := strings.Join(helpers.CollectText(n), "")
		trimmed := strings.TrimSpace(content)
		if trimmed == "" {
			r.SchemaOrgErrors = append(r.SchemaOrgErrors, "Пустой JSON-LD блок")
			return
		}

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
			r.SchemaOrgErrors = append(r.SchemaOrgErrors, "Некорректный JSON: "+err.Error())
			return
		}

		context, hasContext := data["@context"]
		if !hasContext {
			r.SchemaOrgErrors = append(r.SchemaOrgErrors, "Отсутствует @context")
		} else {
			ctxStr := ""
			switch v := context.(type) {
			case string:
				ctxStr = v
			case []interface{}:
				if len(v) > 0 {
					if s, ok := v[0].(string); ok {
						ctxStr = s
					}
				}
			}
			if ctxStr != "https://schema.org" && ctxStr != "http://schema.org" {
				r.SchemaOrgErrors = append(r.SchemaOrgErrors, "@context должен быть 'https://schema.org'")
			}
		}

		if typeVal, hasType := data["@type"]; hasType {
			types := helpers.ExtractTypes(typeVal)
			for _, t := range types {
				if !r.SchemaTypes[t] {
					r.SchemaOrgErrors = append(r.SchemaOrgErrors, fmt.Sprintf("Неизвестный тип Schema.org: %s", t))
				}
			}
		} else {
			r.SchemaOrgErrors = append(r.SchemaOrgErrors, "Отсутствует @type")
		}

		r.JSONLD = append(r.JSONLD, data)
	}
}

func handleImage(n *html.Node, r *report.SEOReport) {
	r.ImageCount++
	if alt, ok := helpers.GetAttrExists(n, "alt"); !ok {
		r.ImageWithoutAlt++
	} else if alt == "" {
		r.ImageWithEmptyAlt++
	}
}

func handleLink(n *html.Node, r *report.SEOReport) {
	href := helpers.GetAttr(n, "href")
	target := helpers.GetAttr(n, "target")
	rel := helpers.GetAttr(n, "rel")

	if target == "_blank" {
		parts := strings.Fields(strings.ToLower(rel))
		hasNoopener := false
		hasNoreferrer := false
		for _, p := range parts {
			if p == "noopener" {
				hasNoopener = true
			}
			if p == "noreferrer" {
				hasNoreferrer = true
			}
		}
		if !hasNoopener || !hasNoreferrer {
			r.InsecureExternalLinks++
		}
	}

	if r.IsHTTPS && strings.HasPrefix(href, "http://") {
		r.InsecureResources++
	}
}

func handleForm(n *html.Node, r *report.SEOReport) {
	r.FormCount++
	action := helpers.GetAttr(n, "action")
	method := strings.ToLower(helpers.GetAttr(n, "method"))
	if r.IsHTTPS && strings.HasPrefix(action, "http://") {
		r.InsecureFormActions++
	}
	if method == "get" {
		r.FormsWithGetMethod++
	}
}

func handleInput(n *html.Node, r *report.SEOReport, labelForMap map[string]bool) {
	id, _ := helpers.GetAttrExists(n, "id")
	name, nameExists := helpers.GetAttrExists(n, "name")
	required, _ := helpers.GetAttrExists(n, "required")

	if !nameExists || name == "" {
		r.InputWithoutName++
	}

	hasLabel := false
	if id != "" && labelForMap[id] {
		hasLabel = true
	}
	if !hasLabel {
		parent := n.Parent
		for parent != nil {
			if parent.Type == html.ElementNode && parent.Data == "label" {
				hasLabel = true
				break
			}
			parent = parent.Parent
		}
	}

	if !hasLabel {
		r.InputWithoutLabel++
		if required != "" {
			r.RequiredWithoutLabel++
		}
	}
}

func extractSchemaTypes(itemtype string) []string {
	var types []string
	for _, part := range strings.Split(itemtype, " ") {
		if strings.Contains(part, "schema.org/") {
			if i := strings.LastIndex(part, "/"); i != -1 {
				types = append(types, part[i+1:])
			}
		}
	}
	return types
}

func resolveURL(baseURL, href string) string {
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
		return ""
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	abs, err := base.Parse(href)
	if err != nil {
		return ""
	}
	return abs.String()
}

// ValidateHeadings - функция проверки заголовков их структуры на странице
func ValidateHeadings(r *report.SEOReport) bool {
	if len(r.HeadingsSequence) == 0 {
		return true
	}
	var levels []int
	for _, tag := range r.HeadingsSequence {
		if len(tag) == 2 && tag[0] == 'h' {
			level := int(tag[1] - '0')
			if level >= 1 && level <= 6 {
				levels = append(levels, level)
			}
		}
	}
	if len(levels) == 0 || levels[0] != 1 {
		return false
	}
	prev := levels[0]
	for _, lvl := range levels[1:] {
		if lvl > prev+1 {
			return false
		}
		prev = lvl
	}
	return true
}

// CheckAIFeatures - функция для анализа на ИИ-дружелюбность
func CheckAIFeatures(r *report.SEOReport) {
	for _, ld := range r.JSONLD {
		if _, has := ld["datePublished"]; has {
			r.HasDatePublished = true
			break
		}
	}

	score := 0
	if r.HasMain {
		score++
	}
	if r.HasJSONLD && len(r.SchemaOrgErrors) == 0 {
		score++
	}
	if r.TextToHTMLRatio > 0.1 {
		score++
	}
	if r.HasDatePublished {
		score++
	}
	if r.Description != "" || len(r.HeadingTexts["h1"]) > 0 {
		score++
	}
	r.AIScore = score
}

// AddWarnings - функция для добавления предупреждений в отчет
func AddWarnings(r *report.SEOReport) {
	if r.TitleLength == 0 {
		r.Warnings = append(r.Warnings, "Отсутствует <title>")
	} else if r.TitleLength > 60 {
		r.Warnings = append(r.Warnings, "Title слишком длинный (>60 символов)")
	}
	if r.DescriptionLength == 0 {
		r.Warnings = append(r.Warnings, "Отсутствует meta description")
	} else if r.DescriptionLength > 160 {
		r.Warnings = append(r.Warnings, "Description слишком длинный (>160 символов)")
	}
	if !r.HasViewport {
		r.Warnings = append(r.Warnings, "Отсутствует <meta name=\"viewport\">")
	}
	if r.HeadingCounts["h1"] == 0 {
		r.Warnings = append(r.Warnings, "Отсутствует <h1>")
	} else if r.HeadingCounts["h1"] > 1 {
		r.Warnings = append(r.Warnings, "Несколько <h1>")
	}
	if !r.HasMain {
		r.Warnings = append(r.Warnings, "Отсутствует <main>")
	}

	if len(r.OG) == 0 {
		r.Info = append(r.Info, "Отсутствует Open Graph разметка")
	} else {
		missing := []string{}
		for _, k := range []string{"title", "description", "image"} {
			if r.OG[k] == "" {
				missing = append(missing, k)
			}
		}
		if len(missing) > 0 {
			r.Info = append(r.Info, "Open Graph: отсутствуют поля "+strings.Join(missing, ", "))
		}
	}

	if len(r.Twitter) == 0 {
		r.Info = append(r.Info, "Отсутствует Twitter Card разметка")
	} else if r.Twitter["card"] == "" {
		r.Info = append(r.Info, "Twitter Card: отсутствует twitter:card")
	}

	if !r.HasJSONLD && !r.HasMicrodata && !r.HasRDFa {
		r.Warnings = append(r.Warnings, "Отсутствуют структурированные данные (Schema.org)")
	}

	if len(r.SchemaOrgErrors) > 0 {
		r.Warnings = append(r.Warnings, "Ошибки Schema.org: "+strings.Join(r.SchemaOrgErrors, "; "))
	} else if r.HasJSONLD {
		r.SchemaOrgValidationOK = true
	}

	if r.ImageWithoutAlt > 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("%d изображений без alt-атрибута", r.ImageWithoutAlt))
	}
	if r.InputWithoutLabel > 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("%d полей без <label>", r.InputWithoutLabel))
	}
	if r.InputWithoutName > 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("%d полей без name", r.InputWithoutName))
	}

	if !r.IsHTTPS {
		r.Warnings = append(r.Warnings, "Сайт не использует HTTPS")
	}
	if r.ResponseTimeMs > 3000 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("Медленная загрузка: %d мс", r.ResponseTimeMs))
	}
	if !r.HasRobotsTxt {
		r.Info = append(r.Info, "Отсутствует robots.txt")
	}
	if !r.HasSitemap {
		r.Info = append(r.Info, "Отсутствует sitemap.xml")
	}
	if len(r.Redirects) > 0 {
		r.Info = append(r.Info, fmt.Sprintf("Цепочка редиректов: %d шагов", len(r.Redirects)))
	}

	if r.InsecureExternalLinks > 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("%d ссылок с target=\"_blank\" без rel=\"noopener noreferrer\"", r.InsecureExternalLinks))
	}
	if r.InsecureResources > 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("%d небезопасных ресурсов (HTTP) на HTTPS-странице", r.InsecureResources))
	}
	if len(r.MissingSecurityHeaders) > 0 {
		r.Warnings = append(r.Warnings, "Отсутствуют заголовки безопасности: "+strings.Join(r.MissingSecurityHeaders, ", "))
	}
	if r.FormsWithGetMethod > 0 {
		r.Info = append(r.Info, fmt.Sprintf("%d форм используют method=\"get\"", r.FormsWithGetMethod))
	}
	if r.InsecureFormActions > 0 {
		r.Warnings = append(r.Warnings, "Формы отправляют данные по HTTP на HTTPS-сайте")
	}

	if r.TextToHTMLRatio < 0.05 {
		r.Warnings = append(r.Warnings, "Низкое соотношение текста к HTML (<5%) — ИИ может не распознать основной контент")
	}
	if !r.HasDatePublished {
		r.Info = append(r.Info, "Отсутствует datePublished — ИИ не сможет определить актуальность")
	}
}
