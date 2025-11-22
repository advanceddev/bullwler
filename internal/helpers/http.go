package helpers

import (
	"net/http"
	"time"
)

// CheckResourceExists - функция проверки целевого ресурса на доступность
func CheckResourceExists(u string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}
