package report

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

// Print для SEOReport (одна страница)
func (r *SEOReport) Print() {
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	white := color.New(color.FgWhite).SprintFunc()

	fmt.Println(cyan("\n🔍 РЕЗУЛЬТАТ АУДИТА"), r.URL)
	fmt.Println(strings.Repeat("─", 65))

	fmt.Printf("🌐 URL: %s\n", white(r.URL))
	fmt.Printf("⏱️  Загрузка: %s мс", white(r.ResponseTimeMs))
	if r.ResponseTimeMs > 3000 {
		fmt.Print(" " + red("(!)"))
	}
	fmt.Println()
	fmt.Printf("🔒 HTTPS: %s\n", boolIcon(r.IsHTTPS))

	fmt.Println("\n" + cyan("🤖 ИИ-ГОТОВНОСТЬ (AI Readiness)"))
	fmt.Printf("  Соотношение текста: %.1f%%", r.TextToHTMLRatio*100)
	if r.TextToHTMLRatio < 0.05 {
		fmt.Print(" " + red("(!)"))
	}
	fmt.Println()
	fmt.Printf("  Основной контент в <main>: %s\n", boolIcon(r.HasMain))
	fmt.Printf("  Дата публикации: %s\n", boolIcon(r.HasDatePublished))
	fmt.Printf("  Структурированные данные: %s\n", boolIcon(r.SchemaOrgValidationOK))
	fmt.Printf("  AI Readiness Score: %s/5\n", white(strconv.Itoa(r.AIScore)))

	fmt.Println("\n" + cyan("📄 SEO"))
	fmt.Printf("  Title: %s %s\n", white(strconvEllipsis(r.Title, 50)), grayf("(%d)", r.TitleLength))
	fmt.Printf("  Desc:  %s %s\n", white(strconvEllipsis(r.Description, 50)), grayf("(%d)", r.DescriptionLength))
	fmt.Printf("  Viewport: %s | Canonical: %s\n", boolIcon(r.HasViewport), boolIcon(r.HasCanonical))

	if len(r.OG) > 0 {
		fmt.Println("\n" + cyan("🖼️  OPEN GRAPH"))
		for _, k := range []string{"title", "description", "image", "url", "type"} {
			if v, ok := r.OG[k]; ok && v != "" {
				fmt.Printf("  og:%-12s: %s\n", k, white(strconvEllipsis(v, 40)))
			}
		}
	}

	if len(r.Twitter) > 0 {
		fmt.Println("\n" + cyan("🐦 TWITTER CARDS"))
		for _, k := range []string{"card", "title", "description", "image"} {
			if v, ok := r.Twitter[k]; ok && v != "" {
				fmt.Printf("  twitter:%-8s: %s\n", k, white(strconvEllipsis(v, 40)))
			}
		}
	}

	fmt.Println("\n" + cyan("🧩 СТРУКТУРИРОВАННЫЕ ДАННЫЕ"))
	if r.HasJSONLD {
		fmt.Printf("  JSON-LD: %s", boolIcon(r.SchemaOrgValidationOK))
		if len(r.JSONLD) > 0 {
			types := []string{}
			for _, ld := range r.JSONLD {
				types = append(types, extractTypes(ld["@type"])...)
			}
			if len(types) > 0 {
				fmt.Printf(" → %s", white(strings.Join(types, ", ")))
			}
		}
		if !r.SchemaOrgValidationOK && len(r.SchemaOrgErrors) > 0 {
			fmt.Printf("%s", " "+red("(!)"))
		}
		fmt.Println()
	}
	if r.HasMicrodata {
		fmt.Printf("  Micro %s", green("найден"))
		if len(r.MicrodataTypes) > 0 {
			fmt.Printf(" → %s", white(strings.Join(r.MicrodataTypes, ", ")))
		}
		fmt.Println()
	}
	if r.HasRDFa {
		fmt.Printf("  RDFa: %s", green("найден"))
		if len(r.RDFaVocabularies) > 0 {
			fmt.Printf(" → vocab=%s", white(r.RDFaVocabularies[0]))
		}
		fmt.Println()
	}
	if !r.HasJSONLD && !r.HasMicrodata && !r.HasRDFa {
		fmt.Printf("  Структурированные данные: %s\n", red("отсутствуют"))
	}

	fmt.Println("\n" + cyan("🧱 СЕМАНТИЧЕСКАЯ РАЗМЕТКА"))
	fmt.Printf("  <header>: %s, <nav>: %s, <main>: %s\n",
		boolIcon(r.HasHeader), boolIcon(r.HasNav), boolIcon(r.HasMain))
	fmt.Printf("  <article>: %s, <section>: %s, <footer>: %s\n",
		boolIcon(r.HasArticle), boolIcon(r.HasSection), boolIcon(r.HasFooter))

	fmt.Println("\n" + cyan("📑 ЗАГОЛОВКИ"))
	counts := []string{}
	for _, level := range []string{"h1", "h2", "h3", "h4", "h5", "h6"} {
		if cnt := r.HeadingCounts[level]; cnt > 0 {
			counts = append(counts, fmt.Sprintf("%s: %s", level, white(strconv.Itoa(cnt))))
		}
	}
	if len(counts) > 0 {
		fmt.Printf("  %s\n", strings.Join(counts, ", "))
	} else {
		fmt.Println("  Нет заголовков h1–h6")
	}
	fmt.Printf("  Иерархия: %s\n", boolIcon(r.HeadingsValid))

	for _, level := range []string{"h1", "h2", "h3", "h4", "h5", "h6"} {
		if texts, exists := r.HeadingTexts[level]; exists && len(texts) > 0 {
			fmt.Printf("    %s: ", level)
			for i, text := range texts {
				if i >= 3 {
					fmt.Print(grayf("(+%d)", len(texts)-3))
					break
				}
				if i > 0 {
					fmt.Print("; ")
				}
				fmt.Print(white(strconvEllipsis(text, 30)))
			}
			fmt.Println()
		}
	}

	fmt.Println("\n" + cyan("♿ ДОСТУПНОСТЬ (a11y)"))
	fmt.Printf("  Изображений: %s | Без alt: %s | alt=\"\": %s\n",
		white(strconv.Itoa(r.ImageCount)), warnCount(r.ImageWithoutAlt), warnCount(r.ImageWithEmptyAlt))
	fmt.Printf("  ARIA: label=%s, labelledby=%s, role=%s\n",
		white(strconv.Itoa(r.AriaLabels)), white(strconv.Itoa(r.AriaLabelledBy)), white(strconv.Itoa(r.Roles)))
	fmt.Printf("  Кнопок без type: %s | Ссылок без href: %s\n",
		warnCount(r.InvalidButtons), warnCount(r.InvalidLinks))

	if r.FormCount > 0 {
		fmt.Println("\n" + cyan("📋 ФОРМЫ"))
		fmt.Printf("  Форм: %s\n", white(strconv.Itoa(r.FormCount)))
		fmt.Printf("  Полей без <label>: %s\n", warnCount(r.InputWithoutLabel))
		fmt.Printf("  Полей без name: %s\n", warnCount(r.InputWithoutName))
		fmt.Printf("  Обязательных без описания: %s\n", warnCount(r.RequiredWithoutLabel))
	}

	if r.InsecureExternalLinks > 0 || r.InsecureResources > 0 || len(r.MissingSecurityHeaders) > 0 {
		fmt.Println("\n" + cyan("🔐 БЕЗОПАСНОСТЬ"))
		if r.InsecureExternalLinks > 0 {
			fmt.Printf("  Ссылок без noopener/noreferrer: %s\n", warnCount(r.InsecureExternalLinks))
		}
		if r.InsecureResources > 0 {
			fmt.Printf("  Небезопасных ресурсов (HTTP): %s\n", warnCount(r.InsecureResources))
		}
		if len(r.MissingSecurityHeaders) > 0 {
			fmt.Printf("  Отсутствующие заголовки: %s\n", white(strings.Join(r.MissingSecurityHeaders, ", ")))
		}
		if r.FormsWithGetMethod > 0 {
			fmt.Printf("  Форм с method=\"get\": %s\n", warnCount(r.FormsWithGetMethod))
		}
	}

	if len(r.Warnings) > 0 {
		fmt.Println("\n" + red("⚠️  ПРОБЛЕМЫ (требуют исправления):"))
		for _, w := range r.Warnings {
			fmt.Printf("  • %s\n", w)
		}
	}

	if len(r.Info) > 0 {
		fmt.Println("\n" + yellow("ℹ️  ЗАМЕЧАНИЯ:"))
		for _, i := range r.Info {
			fmt.Printf("  • %s\n", i)
		}
	}

	if len(r.Errors) > 0 {
		fmt.Println("\n" + red("❌ КРИТИЧЕСКИЕ ОШИБКИ:"))
		for _, e := range r.Errors {
			fmt.Printf("  • %s\n", e)
		}
	} else if len(r.Warnings) == 0 {
		fmt.Println("\n" + green("✅ ВСЁ В ПОРЯДКЕ!"))
	}

	fmt.Println("\n" + strings.Repeat("─", 65))
}

// Print для SiteReport (сайт целиком)
func (sr *SiteReport) Print() {

	sr.MainReport.Print()

	if len(sr.SubReports) <= 1 {
		return
	}

	cyan := color.New(color.FgCyan).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	white := color.New(color.FgWhite).SprintFunc()

	fmt.Println("\n" + cyan("🕷️ СВОДКА ПО САЙТУ"))
	fmt.Printf("Просканировано: %s страниц\n", white(strconv.Itoa(len(sr.SubReports))))

	var totalErrors, totalWarnings int
	var missingTitles, missingH1, brokenPages int

	for _, res := range sr.SubReports {
		if res.Error != nil {
			totalErrors++
			continue
		}
		rep := res.Report
		totalErrors += len(rep.Errors)
		totalWarnings += len(rep.Warnings)
		if rep.Title == "" {
			missingTitles++
		}
		if rep.HeadingCounts["h1"] == 0 {
			missingH1++
		}
		if rep.StatusCode >= 400 {
			brokenPages++
		}
	}

	fmt.Printf("  Ошибок: %s, Предупреждений: %s\n",
		red(strconv.Itoa(totalErrors)),
		yellow(strconv.Itoa(totalWarnings)),
	)

	if missingTitles > 0 {
		fmt.Printf("  ❗ %d страниц без <title>\n", missingTitles)
	}
	if missingH1 > 0 {
		fmt.Printf("  ❗ %d страниц без <h1>\n", missingH1)
	}
	if brokenPages > 0 {
		fmt.Printf("  ❌ %d битых страниц (код ≥ 400)\n", brokenPages)
	}

	type slowPage struct {
		URL  string
		Time int64
	}
	var slow []slowPage
	for _, res := range sr.SubReports {
		if res.Report != nil {
			slow = append(slow, slowPage{res.URL, res.Report.ResponseTimeMs})
		}
	}
	sort.Slice(slow, func(i, j int) bool {
		return slow[i].Time > slow[j].Time
	})
	if len(slow) > 0 && slow[0].Time > 2000 {
		fmt.Print("\n  🐌 Медленные страницы (самые долгие):\n")
		for i := 0; i < 3 && i < len(slow); i++ {
			if slow[i].Time > 2000 {
				fmt.Printf("    %s — %s мс\n",
					strconvEllipsis(slow[i].URL, 40),
					white(strconv.FormatInt(slow[i].Time, 10)),
				)
			}
		}
	}

	warnFreq := make(map[string]int)
	for _, res := range sr.SubReports {
		if res.Report != nil {
			for _, w := range res.Report.Warnings {
				warnFreq[w]++
			}
		}
	}

	if len(warnFreq) > 0 {
		fmt.Println("\n  📉 Самые частые предупреждения:")

		type warnCount struct {
			Text  string
			Count int
		}
		var sorted []warnCount
		for text, count := range warnFreq {
			sorted = append(sorted, warnCount{text, count})
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Count > sorted[j].Count
		})

		for i := 0; i < 5 && i < len(sorted); i++ {
			fmt.Printf("    • %s (%dx)\n", sorted[i].Text, sorted[i].Count)
		}
	}

	fmt.Println(strings.Repeat("─", 65))

}

func boolIcon(ok bool) string {
	if ok {
		return color.GreenString("✅")
	}
	return color.RedString("❌")
}

func warnCount(n int) string {
	if n == 0 {
		return color.GreenString("0")
	}
	return color.RedString("%d", n)
}

func grayf(format string, args ...interface{}) string {
	return color.New(color.FgHiBlack).Sprintf(format, args...)
}

func strconvEllipsis(s string, maximum int) string {
	if len(s) <= maximum {
		return s
	}
	return s[:maximum-3] + "..."
}

func extractTypes(v any) []string {
	var types []string
	switch val := v.(type) {
	case string:
		types = append(types, val)
	case []any:
		for _, item := range val {
			if s, ok := item.(string); ok {
				types = append(types, s)
			}
		}
	}
	return types
}
