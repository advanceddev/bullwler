package analyzer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"bullwler/internal/helpers"
	"bullwler/internal/htmlparser"
	"bullwler/internal/report"

	"golang.org/x/net/html"
)

// HasScheme - проверяет наличие протокола
func HasScheme(u string) bool {
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}

// AnalyzeURL - функция анализа ресурса по ссылке
func AnalyzeURL(rawURL string) *report.SEOReport {
	schemaTypes, err := LoadSchemaTypes()
	if err != nil {
		schemaTypes = GetFallbackSchemaTypes()
		fmt.Fprintf(os.Stderr, "⚠️  Используется fallback-список типов: %v\n", err)
	}

	rep := report.New(rawURL, schemaTypes)

	base, err := url.Parse(rawURL)
	if err != nil {
		rep.Errors = append(rep.Errors, "Некорректный URL")
		return rep
	}

	rep.HasRobotsTxt = helpers.CheckResourceExists(base.Scheme + "://" + base.Host + "/robots.txt")
	rep.HasSitemap = helpers.CheckResourceExists(base.Scheme + "://" + base.Host + "/sitemap.xml")

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			rep.Redirects = append(rep.Redirects, req.URL.String())
			return nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Bullwler/1.0)")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		rep.Errors = append(rep.Errors, "Не удалось загрузить страницу: "+err.Error())
		return rep
	}
	defer resp.Body.Close()

	rep.StatusCode = resp.StatusCode
	rep.ResponseTimeMs = time.Since(start).Milliseconds()

	// Security headers
	rep.MissingSecurityHeaders = checkSecurityHeaders(resp.Header, rep.IsHTTPS)

	if resp.StatusCode != 200 {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("HTTP статус: %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		rep.Errors = append(rep.Errors, "Ошибка чтения тела")
		return rep
	}

	htmlStr := string(body)
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		rep.Errors = append(rep.Errors, "Ошибка парсинга HTML")
		return rep
	}

	rep.HTMLBytes = len(htmlStr)
	rep.TextBytes = len(helpers.GetText(doc))
	if rep.HTMLBytes > 0 {
		rep.TextToHTMLRatio = float64(rep.TextBytes) / float64(rep.HTMLBytes)
	}

	labelForMap := make(map[string]bool)
	helpers.CollectLabelFor(doc, labelForMap)
	htmlparser.AnalyzeNode(doc, rep, labelForMap)

	rep.TitleLength = len(rep.Title)
	rep.DescriptionLength = len(rep.Description)
	rep.HeadingsValid = htmlparser.ValidateHeadings(rep)

	htmlparser.CheckAIFeatures(rep)
	htmlparser.AddWarnings(rep)

	return rep
}

func checkSecurityHeaders(headers http.Header, isHTTPS bool) []string {
	var missing []string
	if headers.Get("Content-Security-Policy") == "" {
		missing = append(missing, "Content-Security-Policy")
	}
	if headers.Get("X-Frame-Options") == "" {
		missing = append(missing, "X-Frame-Options")
	}
	if headers.Get("X-Content-Type-Options") != "nosniff" {
		missing = append(missing, "X-Content-Type-Options")
	}
	if isHTTPS && headers.Get("Strict-Transport-Security") == "" {
		missing = append(missing, "Strict-Transport-Security")
	}
	return missing
}
