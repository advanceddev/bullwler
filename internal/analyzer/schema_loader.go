package analyzer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	schemaURL   = "https://schema.org/version/latest/schemaorg-all-http.jsonld"
	schemaFile  = "schemaorg-types.json"
	maxAgeHours = 24
)

// LoadSchemaTypes - функция загрузки типов schema.org в файл
func LoadSchemaTypes() (map[string]bool, error) {
	if info, err := os.Stat(schemaFile); err == nil {
		if time.Since(info.ModTime()) < maxAgeHours*time.Hour {
			data, err := os.ReadFile(schemaFile)
			if err != nil {
				return nil, err
			}
			var types map[string]bool
			if err := json.Unmarshal(data, &types); err != nil {
				return nil, err
			}
			return types, nil
		}
	}

	fmt.Fprintf(os.Stderr, "⏳ Загрузка актуальных типов Schema.org...\n")
	resp, err := http.Get(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("не удалось скачать schema.org: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ошибка HTTP %d", resp.StatusCode)
	}

	var container struct {
		Graph []json.RawMessage `json:"@graph"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&container); err != nil {
		return nil, fmt.Errorf("ошибка парсинга корневого JSON: %w", err)
	}

	types := make(map[string]bool)
	for _, rawNode := range container.Graph {
		var node map[string]interface{}
		if err := json.Unmarshal(rawNode, &node); err != nil {
			continue
		}

		id, ok := node["@id"].(string)
		if !ok {
			continue
		}

		if !isSchemaClass(node) {
			continue
		}

		typeName := extractTypeName(id)
		if typeName != "" {
			types[typeName] = true
		}
	}

	data, _ := json.Marshal(types)
	os.WriteFile(schemaFile, data, 0644)
	fmt.Fprintf(os.Stderr, "✅ Загружено %d типов Schema.org\n", len(types))
	return types, nil
}

func isSchemaClass(node map[string]interface{}) bool {
	typ, exists := node["@type"]
	if !exists {
		return false
	}

	switch v := typ.(type) {
	case string:
		return v == "rdfs:Class"
	case []interface{}:
		for _, t := range v {
			if tStr, ok := t.(string); ok && tStr == "rdfs:Class" {
				return true
			}
		}
	}
	return false
}

func extractTypeName(id string) string {
	if strings.HasPrefix(id, "https://schema.org/") {
		typeName := strings.TrimPrefix(id, "https://schema.org/")
		if idx := strings.Index(typeName, "#"); idx != -1 {
			typeName = typeName[idx+1:]
		}
		return typeName
	} else if strings.HasPrefix(id, "schema:") {
		return strings.TrimPrefix(id, "schema:")
	}
	return ""
}

// GetFallbackSchemaTypes - возвращает фоллбэк данные, если отсутствуют типы schema.org
func GetFallbackSchemaTypes() map[string]bool {
	return map[string]bool{
		"Thing": true, "CreativeWork": true, "Article": true, "BlogPosting": true,
		"WebPage": true, "WebSite": true, "Organization": true, "Person": true,
		"Product": true, "Offer": true, "Event": true, "LocalBusiness": true,
	}
}
