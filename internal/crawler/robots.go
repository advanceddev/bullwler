package crawler

import (
	"net/http"
	"net/url"
	"time"

	"github.com/temoto/robotstxt"
)

// RobotsClient - управляет загрузкой и кэшированием robots.txt
type RobotsClient struct {
	client *http.Client
	cache  map[string]*robotstxt.RobotsData
}

// NewRobotsClient - создает новый инстанс клиента для работы с robots.txt
func NewRobotsClient() *RobotsClient {
	return &RobotsClient{
		client: &http.Client{Timeout: 10 * time.Second},
		cache:  make(map[string]*robotstxt.RobotsData),
	}
}

// Allowed - метод определения доступности
func (rc *RobotsClient) Allowed(userAgent, targetURL string) bool {
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return false
	}

	host := parsed.Scheme + "://" + parsed.Host
	if _, ok := rc.cache[host]; !ok {
		robotsURL := host + "/robots.txt"
		resp, err := rc.client.Get(robotsURL)
		if err != nil {
			rc.cache[host] = nil
			return true
		}
		defer resp.Body.Close()

		robots, err := robotstxt.FromResponse(resp)
		if err != nil {
			rc.cache[host] = nil
			return true
		}
		rc.cache[host] = robots
	}

	if rc.cache[host] == nil {
		return true
	}

	return rc.cache[host].TestAgent(targetURL, userAgent)
}
