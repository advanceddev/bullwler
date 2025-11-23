package helpers

import (
	"golang.org/x/net/html"
)

// GetText - функция получения текстового содержимого
func GetText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var result string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		result += GetText(c)
	}
	return result
}

// CollectText - функция сбора текстового содержимого
func CollectText(n *html.Node) []string {
	if n.Type == html.TextNode {
		return []string{n.Data}
	}
	var result []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		result = append(result, CollectText(c)...)
	}
	return result
}

// CollectLabelFor - функция сбора тегов label for=
func CollectLabelFor(n *html.Node, m map[string]bool) {
	if n.Type == html.ElementNode && n.Data == "label" {
		if forAttr, exists := GetAttrExists(n, "for"); exists && forAttr != "" {
			m[forAttr] = true
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		CollectLabelFor(c, m)
	}
}

// GetAttrExists - функция проверки наличия атрибута у DOM-элемента
func GetAttrExists(n *html.Node, key string) (string, bool) {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val, true
		}
	}
	return "", false
}

// GetAttr - функция получения атрибута и его значения у DOM-элемента
func GetAttr(n *html.Node, key string) string {
	if val, exists := GetAttrExists(n, key); exists {
		return val
	}
	return ""
}

// HasAttr - проверяет наличие атрибуда у DOM элемента
func HasAttr(n *html.Node, key string) bool {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return true
		}
	}
	return false
}

// GetMetaAttrsFull - функция получения meta-элементов документа
func GetMetaAttrsFull(n *html.Node) (name, prop, content string) {
	for _, attr := range n.Attr {
		switch attr.Key {
		case "name":
			name = attr.Val
		case "property":
			prop = attr.Val
		case "content":
			content = attr.Val
		}
	}
	return
}

// ExtractTypes - функция извлечения типов
func ExtractTypes(v any) []string {
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
