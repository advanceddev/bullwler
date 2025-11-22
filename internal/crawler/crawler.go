package crawler

import (
	"fmt"
	"net/url"
	"time"

	"bullwler/internal/analyzer"
	"bullwler/internal/report"
)

// Crawler - структура краулера
type Crawler struct {
	robots      *RobotsClient
	maxDepth    int
	maxPages    int
	concurrency int
	userAgent   string
}

// NewCrawler - создает новый инстанс краулера
func NewCrawler(opts ...Option) *Crawler {
	c := &Crawler{
		robots:      NewRobotsClient(),
		maxDepth:    2,
		maxPages:    50,
		concurrency: 5,
		userAgent:   "BullwlerBot/1.0",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Option - структура опции краулера
type Option func(*Crawler)

// WithMaxDepth - задает максимальную глубину сканирования
func WithMaxDepth(d int) Option { return func(c *Crawler) { c.maxDepth = d } }

// WithMaxPages - задает максимальное количество страниц для сканирования
func WithMaxPages(n int) Option { return func(c *Crawler) { c.maxPages = n } }

// WithConcurrency - задает максимальное колисество горутин
func WithConcurrency(n int) Option { return func(c *Crawler) { c.concurrency = n } }

// Crawl - рекурсивно сканирует ресурс (итеративная реализация)
func (c *Crawler) Crawl(startURL string) ([]report.CrawlResult, error) {
	base, err := url.Parse(startURL)
	if err != nil {
		return nil, fmt.Errorf("некорректный стартовый URL: %w", err)
	}
	allowedHost := base.Hostname()

	seen := make(map[string]bool)
	results := make([]report.CrawlResult, 0, c.maxPages)

	queue := []crawlTask{{URL: startURL, Depth: 0}}

	for len(queue) > 0 && len(seen) < c.maxPages {

		task := queue[0]
		queue = queue[1:]

		if task.Depth > c.maxDepth {
			continue
		}

		if seen[task.URL] {
			continue
		}
		seen[task.URL] = true

		if !c.robots.Allowed(c.userAgent, task.URL) {
			results = append(results, report.CrawlResult{
				URL:   task.URL,
				Error: fmt.Errorf("запрещено robots.txt"),
			})
			continue
		}

		rep := analyzer.AnalyzeURL(task.URL)
		results = append(results, report.CrawlResult{URL: task.URL, Report: rep})

		if task.Depth < c.maxDepth && rep.StatusCode == 200 {
			newURLs := c.extractInternalLinks(rep, allowedHost)
			for _, nextURL := range newURLs {
				if !seen[nextURL] && len(seen) < c.maxPages {
					queue = append(queue, crawlTask{URL: nextURL, Depth: task.Depth + 1})
				}
			}
		}

		time.Sleep(300 * time.Millisecond)
	}

	return results, nil
}

// CrawlSite - формирует отчет о просканированном ресурсе
func (c *Crawler) CrawlSite(startURL string) (*report.SiteReport, error) {
	results, err := c.Crawl(startURL)
	if err != nil {
		return nil, err
	}

	var mainRep *report.SEOReport
	for _, res := range results {
		if res.Report != nil && res.URL == startURL {
			mainRep = res.Report
			break
		}
	}

	if mainRep == nil {
		mainRep = analyzer.AnalyzeURL(startURL)
	}

	return &report.SiteReport{
		MainURL:    startURL,
		MainReport: mainRep,
		SubReports: results,
	}, nil
}

type crawlTask struct {
	URL   string
	Depth int
}

func (c *Crawler) extractInternalLinks(rep *report.SEOReport, host string) []string {
	var internal []string
	for _, link := range rep.AllLinks {
		u, err := url.Parse(link)
		if err == nil && u.Hostname() == host {
			internal = append(internal, link)
		}
	}
	return internal
}
