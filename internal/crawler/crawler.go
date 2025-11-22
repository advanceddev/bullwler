package crawler

import (
	"fmt"
	"net/url"
	"sync"
	"time"

	"bullwler/internal/analyzer"
	"bullwler/internal/report"

	"github.com/cheggaaa/pb/v3"
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

// Crawl - рекурсивно сканирует ресурс
func (c *Crawler) Crawl(startURL string) ([]report.CrawlResult, error) {
	base, err := url.Parse(startURL)
	if err != nil {
		return nil, fmt.Errorf("некорректный стартовый URL: %w", err)
	}
	allowedHost := base.Hostname()

	bar := pb.Simple.Start64(int64(c.maxPages))
	bar.Start()

	var mu sync.Mutex
	seen := make(map[string]bool)
	results := make([]report.CrawlResult, 0, c.maxPages)
	queue := make(chan crawlTask, c.maxPages*2)
	var wg sync.WaitGroup

	for i := 0; i < c.concurrency; i++ {
		wg.Add(1)   // ← ДОБАВЛЕНО
		go func() { // ← ИСПРАВЛЕНО: go func(), а не wg.Go()
			defer wg.Done() // ← ДОБАВЛЕНО
			for task := range queue {
				if task.Depth > c.maxDepth {
					continue
				}

				mu.Lock()
				if seen[task.URL] || len(seen) >= c.maxPages {
					mu.Unlock()
					continue
				}
				seen[task.URL] = true
				mu.Unlock()

				if !c.robots.Allowed(c.userAgent, task.URL) {
					mu.Lock()
					results = append(results, report.CrawlResult{
						URL:   task.URL,
						Error: fmt.Errorf("запрещено robots.txt"),
					})
					mu.Unlock()
					bar.Increment()
					continue
				}

				rep := analyzer.AnalyzeURL(task.URL)

				mu.Lock()
				results = append(results, report.CrawlResult{URL: task.URL, Report: rep})
				if task.Depth < c.maxDepth && rep.StatusCode == 200 {
					newURLs := c.extractInternalLinks(rep, allowedHost)
					for _, nextURL := range newURLs {
						if !seen[nextURL] {
							queue <- crawlTask{URL: nextURL, Depth: task.Depth + 1}
						}
					}
				}
				mu.Unlock()

				bar.Increment()
				time.Sleep(500 * time.Millisecond)
			}
		}()
	}

	queue <- crawlTask{URL: startURL, Depth: 0}

	go func() {
		time.Sleep(15 * time.Second)
		close(queue)
	}()

	wg.Wait()
	bar.Finish()
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
