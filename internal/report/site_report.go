package report

// CrawlResult — результат анализа одной страницы в рамках краулинга
type CrawlResult struct {
	URL    string
	Report *SEOReport
	Error  error
}

// SiteReport — сводный отчёт по всему сайту
type SiteReport struct {
	MainURL    string
	MainReport *SEOReport
	SubReports []CrawlResult
}
