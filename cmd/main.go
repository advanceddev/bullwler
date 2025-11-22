package main

import (
	"net/url"
	"os"
	"strings"

	"bullwler/internal/analyzer"
	"bullwler/internal/crawler"

	"github.com/fatih/color"
)

func main() {
	if len(os.Args) < 2 {
		color.Red("Использование: bullwler <URL>")
		os.Exit(1)
	}

	targetURL := os.Args[1]
	if !analyzer.HasScheme(targetURL) {
		targetURL = "https://" + targetURL
	}

	isRoot := isSiteRoot(targetURL)

	if isRoot {
		c := crawler.NewCrawler(
			crawler.WithMaxDepth(3),
			crawler.WithMaxPages(30),
			crawler.WithConcurrency(5),
		)
		siteRep, err := c.CrawlSite(targetURL)
		if err != nil {
			color.Red("Ошибка сканирования сайта: %v", err)
			os.Exit(1)
		}
		siteRep.Print()
	} else {
		rep := analyzer.AnalyzeURL(targetURL)
		rep.Print()
	}
}

func isSiteRoot(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	path := strings.TrimRight(u.Path, "/")
	return path == "" || path == "/index.html" || path == "/index.htm"
}
