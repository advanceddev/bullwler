package crawler

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"bullwler/internal/analyzer"
	"bullwler/internal/report"
)

// Crawler — структура краулера
type Crawler struct {
	robots      *RobotsClient
	maxDepth    int
	maxPages    int
	concurrency int
	userAgent   string
}

// NewCrawler — создаёт новый инстанс краулера
func NewCrawler(opts ...Option) *Crawler {
	c := &Crawler{
		robots:      NewRobotsClient(),
		maxDepth:    2,
		maxPages:    30,
		concurrency: 5,
		userAgent:   "BullwlerBot/1.0",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Option — функциональная опция
type Option func(*Crawler)

// WithMaxDepth — задаёт максимальную глубину сканирования
func WithMaxDepth(d int) Option { return func(c *Crawler) { c.maxDepth = d } }

// WithMaxPages — задаёт максимальное количество страниц
func WithMaxPages(n int) Option { return func(c *Crawler) { c.maxPages = n } }

// WithConcurrency — задаёт количество параллельных горутин
func WithConcurrency(n int) Option { return func(c *Crawler) { c.concurrency = n } }

type crawlTask struct {
	URL   string
	Depth int
}

// Crawl — рекурсивно сканирует сайт
func (c *Crawler) Crawl(startURL string) ([]report.CrawlResult, error) {
	base, err := url.Parse(startURL)
	if err != nil {
		return nil, fmt.Errorf("некорректный стартовый URL: %w", err)
	}
	allowedHost := base.Hostname()

	var mu sync.Mutex
	seen := make(map[string]bool)
	var results []report.CrawlResult

	taskQueue := make(chan crawlTask, c.maxPages)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	for i := 0; i < c.concurrency; i++ {
		g.Go(func() error {
			for {
				select {
				case <-gCtx.Done():
					return nil
				case task, ok := <-taskQueue:
					if !ok {
						return nil
					}

					if task.Depth > c.maxDepth {
						continue
					}

					normalizedURL := normalizeURL(task.URL)

					mu.Lock()
					if seen[normalizedURL] || len(seen) >= c.maxPages {
						mu.Unlock()
						continue
					}
					seen[normalizedURL] = true
					currentCount := len(seen)
					mu.Unlock()

					log.Printf("➤ Анализ %s (%d/%d)", task.URL, currentCount, c.maxPages)

					var res report.CrawlResult
					if !c.robots.Allowed(c.userAgent, task.URL) {
						res = report.CrawlResult{
							URL:   task.URL,
							Error: fmt.Errorf("запрещено robots.txt"),
						}
					} else {
						rep := analyzer.AnalyzeURL(task.URL)
						res = report.CrawlResult{URL: task.URL, Report: rep}
					}

					mu.Lock()
					results = append(results, res)
					mu.Unlock()

					if task.Depth < c.maxDepth && res.Report != nil && res.Report.StatusCode == 200 {
						newURLs := c.extractInternalLinks(res.Report, allowedHost)
						mu.Lock()
						for _, nextURL := range newURLs {
							normalizedNext := normalizeURL(nextURL)
							if !seen[normalizedNext] && len(seen) < c.maxPages {
								select {
								case taskQueue <- crawlTask{URL: nextURL, Depth: task.Depth + 1}:
								case <-gCtx.Done():
									mu.Unlock()
									return nil
								default:
								}
							}
						}
						mu.Unlock()
					}

					time.Sleep(200 * time.Millisecond)
				}
			}
		})
	}

	select {
	case taskQueue <- crawlTask{URL: startURL, Depth: 0}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	_ = g.Wait()

	close(taskQueue)

	log.Printf("Сканирование завершено. Обработано %d страниц", len(results))
	return results, nil
}

// CrawlSite — формирует сводный отчёт
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

func normalizeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Fragment = ""
	u.RawQuery = ""
	if u.Path == "/" {
		u.Path = ""
	}

	return strings.TrimRight(u.String(), "/")
}

func (c *Crawler) extractInternalLinks(rep *report.SEOReport, host string) []string {
	var internal []string
	seen := make(map[string]bool)

	for _, link := range rep.AllLinks {
		u, err := url.Parse(link)
		if err != nil || u.Hostname() != host {
			continue
		}
		normalized := normalizeURL(u.String())

		if !seen[normalized] {
			seen[normalized] = true
			internal = append(internal, normalized)
		}
	}
	return internal
}
