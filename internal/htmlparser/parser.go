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
	case "html":
		r.HTMLLang = helpers.GetAttr(n, "lang")
		if u, err := url.Parse(r.URL); err == nil {
			r.Host = u.Host
		}
	case "ol", "ul":
		r.ListCount++
	case "table":
		r.TableCount++
	case "meta":
		handleMeta(n, r)
	case "title":
		if text := helpers.GetText(n); text != "" {
			r.Title = strings.TrimSpace(text)
		}
	case "link":
		if rel := helpers.GetAttr(n, "rel"); rel == "canonical" {
			r.HasCanonical = true
			if u, err := url.Parse(helpers.GetAttr(n, "href")); err == nil {
				r.CanonicalHost = u.Host
			}
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
	case "p":
		if text := helpers.GetText(n); text != "" {
			cleanText := strings.TrimSpace(text)
			if cleanText != "" {
				r.Paragraphs = append(r.Paragraphs, cleanText)
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

	if helpers.HasAttr(n, "onclick") && !helpers.HasAttr(n, "tabindex") {
		r.A11yWarnings = append(r.A11yWarnings, "<div> с onclick должен иметь tabindex=\"0\" для клавиатурной навигации")
	}

	if idVal, hasID := helpers.GetAttrExists(n, "id"); hasID && idVal != "" {
		r.AllIDs = append(r.AllIDs, idVal)
	}

	// ARIA
	for _, attr := range n.Attr {
		switch attr.Key {
		case "aria-label":
			r.AriaLabels++
		case "aria-labelledby":
			r.AriaLabelledBy++

			targets := strings.Fields(attr.Val)
			for _, targetID := range targets {
				if !idExists(r.AllIDs, targetID) {
					r.A11yErrors = append(r.A11yErrors, fmt.Sprintf("aria-labelledby='%s' ссылается на несуществующий id", targetID))
				}
			}
		case "role":
			r.Roles++
			if !isValidRoleForElement(tag, attr.Val) {
				r.A11yWarnings = append(r.A11yWarnings, fmt.Sprintf("Недопустимая роль '%s' для <%s>", attr.Val, tag))
			}
			required := requiredAriaAttrs(attr.Val)
			for _, reqAttr := range required {
				if !helpers.HasAttr(n, reqAttr) {
					r.A11yErrors = append(r.A11yErrors, fmt.Sprintf("Роль '%s' требует атрибут %s", attr.Val, reqAttr))
				}
			}
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
	alt, hasAlt := helpers.GetAttrExists(n, "alt")

	if !hasAlt {
		r.ImageWithoutAlt++
		r.A11yErrors = append(r.A11yErrors, "Изображение без alt-атрибута")
	} else if alt == "" {
		r.ImageWithEmptyAlt++
	} else {
		lower := strings.ToLower(alt)
		if strings.Contains(lower, "изображение") || strings.Contains(lower, "image") || strings.Contains(lower, "img") {
			r.A11yWarnings = append(r.A11yWarnings, fmt.Sprintf("Бесполезный alt: '%s'", alt))
		}
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
		if !helpers.HasAttr(n, "aria-label") && !helpers.HasAttr(n, "aria-labelledby") {
			r.A11yErrors = append(r.A11yErrors, "Поле ввода не имеет доступной метки (ни <label>, ни aria-label)")
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

func hasDirectAnswer(r *report.SEOReport) bool {
	if len(r.Paragraphs) == 0 || r.Title == "" {
		return false
	}

	title := strings.ToLower(r.Title)
	firstParagraphs := ""
	for i := 0; i < 2 && i < len(r.Paragraphs); i++ {
		firstParagraphs += " " + strings.ToLower(r.Paragraphs[i])
	}

	if strings.HasSuffix(strings.TrimSpace(title), "?") {
		answerWords := []string{"это", "означает", "является", "можно", "следует", "важно", "необходимо"}
		for _, word := range answerWords {
			if strings.Contains(firstParagraphs, word) {
				return true
			}
		}
	}
	return false
}

func calculateTextDensity(r *report.SEOReport) float64 {
	if len(r.Paragraphs) == 0 {
		return 0
	}

	totalWords := 0
	uniqueWords := make(map[string]bool)

	for _, p := range r.Paragraphs {
		words := strings.Fields(strings.ToLower(p))
		totalWords += len(words)
		for _, w := range words {
			clean := strings.Trim(w, ".,!?;:")
			if len(clean) > 2 {
				uniqueWords[clean] = true
			}
		}
	}

	if totalWords == 0 {
		return 0
	}

	return float64(len(uniqueWords)) / float64(totalWords)
}

func idExists(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func isValidRoleForElement(tag, role string) bool {

	if role == "presentation" || role == "none" {
		return true
	}

	switch role {
	case "heading":
		return tag == "h1" || tag == "h2" || tag == "h3" || tag == "h4" || tag == "h5" || tag == "h6"
	case "button":
		return tag == "button" || tag == "summary" || tag == "a" || tag == "input"
	case "checkbox", "radio":
		return tag == "input"
	case "textbox", "searchbox", "combobox":
		return tag == "input" || tag == "textarea"
	case "list":
		return tag == "ul" || tag == "ol"
	case "listitem":
		return tag == "li"
	case "img":
		return tag == "img"
	case "link":
		return tag == "a"
	case "navigation", "banner", "main", "complementary", "contentinfo", "region", "form", "search",
		"article", "section", "aside", "figure", "application", "document", "feed", "log", "marquee", "status", "timer":
		return true
	}

	return true
}

func requiredAriaAttrs(role string) []string {
	switch role {
	case "checkbox":
		return []string{"aria-checked"}
	case "radio":
		return []string{"aria-checked"}
	case "slider":
		return []string{"aria-valuenow", "aria-valuemin", "aria-valuemax"}
	case "spinbutton":
		return []string{"aria-valuenow"}
	case "progressbar":
		return []string{"aria-valuenow"}
	default:
		return nil
	}
}

// CheckAIDeepFeatures — расширенный анализ для ИИ-индексации
func CheckAIDeepFeatures(r *report.SEOReport) {
	for _, ld := range r.JSONLD {
		if t, ok := ld["@type"].(string); ok {
			switch t {
			case "FAQPage":
				r.HasFAQStructured = true
			case "HowTo":
				r.HasHowToStructured = true
			}
		}
	}

	r.HasDirectAnswer = hasDirectAnswer(r)

	r.TextDensityScore = calculateTextDensity(r)
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

// CheckAIFeatures - расширенный анализ на ИИ-дружелюбность
func CheckAIFeatures(r *report.SEOReport) {
	for _, ld := range r.JSONLD {
		if _, has := ld["datePublished"]; has {
			r.HasDatePublished = true
		}
		if _, has := ld["dateModified"]; has {
			r.HasDateModified = true
		}
		if author, has := ld["author"]; has {
			r.HasAuthor = true
			if authorMap, ok := author.(map[string]interface{}); ok {
				if _, hasName := authorMap["name"]; hasName {
					r.HasAuthorWithName = true
				}
			}
		}
	}

	score := 0

	if r.HasMain {
		score++
	}
	if r.HasJSONLD && len(r.SchemaOrgErrors) == 0 {
		score++
	}
	if r.Description != "" || len(r.HeadingTexts["h1"]) > 0 {
		score++
	}
	if r.HasCanonical {
		score++
	}

	if r.HasDatePublished || r.HasDateModified {
		score++
	}
	if r.HasAuthorWithName {
		score++
	}

	if r.TextBytes > 500 {
		score++
	}
	if r.TextToHTMLRatio > 0.1 {
		score++
	}
	if r.TextToHTMLRatio > 0.2 {
		score++
	}

	if r.ListCount > 0 || r.TableCount > 0 {
		score++
	}

	if r.HTMLLang != "" {
		score++
	}
	if r.HasDirectAnswer {
		score++
	}
	if r.TextDensityScore > 0.6 {
		score++
	}
	if r.HasFAQStructured || r.HasHowToStructured {
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
	if !r.HasDirectAnswer && strings.HasSuffix(strings.TrimSpace(r.Title), "?") {
		r.Info = append(r.Info, "Заголовок-вопрос не содержит прямого ответа в тексте")
	}
	if r.TextDensityScore < 0.4 {
		r.Warnings = append(r.Warnings, "Высокая доля 'воды' в тексте — ИИ может проигнорировать")
	}
}
