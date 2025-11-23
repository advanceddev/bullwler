package report

import "strings"

// SEOReport - структура отчета по странице
type SEOReport struct {
	URL string

	// Производительность
	ResponseTimeMs int64
	StatusCode     int
	IsHTTPS        bool
	Redirects      []string
	HasRobotsTxt   bool
	HasSitemap     bool

	// Мета
	Title             string
	TitleLength       int
	Description       string
	DescriptionLength int
	HasViewport       bool
	HasCanonical      bool

	// Open Graph / Twitter
	OG      map[string]string
	Twitter map[string]string

	// Структурированные данные
	JSONLD                []map[string]interface{}
	MicrodataTypes        []string
	RDFaVocabularies      []string
	HasJSONLD             bool
	HasMicrodata          bool
	HasRDFa               bool
	SchemaOrgValidationOK bool
	SchemaOrgErrors       []string
	SchemaTypes           map[string]bool

	// Семантика
	HasHeader  bool
	HasNav     bool
	HasMain    bool
	HasArticle bool
	HasSection bool
	HasFooter  bool

	// Заголовки
	HeadingCounts    map[string]int
	HeadingTexts     map[string][]string
	HeadingsSequence []string
	HeadingsValid    bool

	// Доступность
	ImageCount        int
	ImageWithoutAlt   int
	ImageWithEmptyAlt int
	AriaLabels        int
	AriaLabelledBy    int
	Roles             int
	InvalidButtons    int
	InvalidLinks      int

	// Формы
	FormCount            int
	InputWithoutLabel    int
	InputWithoutName     int
	RequiredWithoutLabel int
	LabelsWithoutFor     int

	// Безопасность
	InsecureExternalLinks  int
	InsecureResources      int
	MissingSecurityHeaders []string
	FormsWithGetMethod     int
	InsecureFormActions    int

	// AI-дружелюбность
	TextBytes          int
	HTMLBytes          int
	TextToHTMLRatio    float64
	HasDatePublished   bool
	ParagraphCount     int
	AvgParagraphLength int
	AIScore            int
	HasDateModified    bool
	HasAuthor          bool
	HasAuthorWithName  bool
	ListCount          int
	TableCount         int
	HTMLLang           string
	CanonicalHost      string
	Host               string

	// Для краулера
	AllLinks []string

	// Сообщения
	Errors   []string
	Warnings []string
	Info     []string
}

// New - возвращает новый отчет
func New(rawURL string, schemaTypes map[string]bool) *SEOReport {
	return &SEOReport{
		URL:                    rawURL,
		IsHTTPS:                strings.HasPrefix(rawURL, "https://"),
		OG:                     make(map[string]string),
		Twitter:                make(map[string]string),
		JSONLD:                 []map[string]interface{}{},
		HeadingCounts:          make(map[string]int),
		HeadingTexts:           make(map[string][]string),
		Errors:                 []string{},
		Warnings:               []string{},
		Info:                   []string{},
		SchemaTypes:            schemaTypes,
		MissingSecurityHeaders: []string{},
		AllLinks:               []string{},
	}
}
