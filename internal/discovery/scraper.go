package discovery

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/rs/zerolog"
)

// WebScraper gestisce lo scraping di awesome lists e altre risorse
type WebScraper struct {
	httpClient *http.Client
	logger     zerolog.Logger
}

// AwesomeList rappresenta una lista awesome da scrapare
type AwesomeList struct {
	Name string
	URL  string
}

// NewWebScraper crea un nuovo web scraper
func NewWebScraper(logger zerolog.Logger) *WebScraper {
	return &WebScraper{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger.With().Str("component", "scraper").Logger(),
	}
}

// ScrapeAwesomeLists scrape le awesome lists per trovare API
func (s *WebScraper) ScrapeAwesomeLists(ctx context.Context) ([]Candidate, error) {
	lists := []AwesomeList{
		{
			Name: "awesome-chatgpt-api",
			URL:  "https://raw.githubusercontent.com/reorx/awesome-chatgpt-api/master/README.md",
		},
		{
			Name: "awesome-free-chatgpt",
			URL:  "https://raw.githubusercontent.com/LiLittleCat/awesome-free-chatgpt/main/README.md",
		},
		{
			Name: "awesome-ai-services",
			URL:  "https://raw.githubusercontent.com/mahseema/awesome-ai-c2-prompts/main/README.md",
		},
		{
			Name: "free-gpt-api-list",
			URL:  "https://raw.githubusercontent.com/xxxily/hello-ai/main/home/FreeChatGPTAPI.md",
		},
	}

	candidates := make([]Candidate, 0)

	for _, list := range lists {
		s.logger.Info().
			Str("name", list.Name).
			Str("url", list.URL).
			Msg("Scraping awesome list")

		listCandidates, err := s.scrapeList(ctx, list)
		if err != nil {
			s.logger.Warn().
				Err(err).
				Str("name", list.Name).
				Msg("Failed to scrape list")
			continue
		}

		candidates = append(candidates, listCandidates...)
		s.logger.Info().
			Str("name", list.Name).
			Int("found", len(listCandidates)).
			Msg("List scraped successfully")
	}

	return candidates, nil
}

// scrapeList scrape una singola awesome list
func (s *WebScraper) scrapeList(ctx context.Context, list AwesomeList) ([]Candidate, error) {
	content, err := s.fetchContent(ctx, list.URL)
	if err != nil {
		return nil, err
	}

	return s.parseMarkdownForAPIs(content, list.Name), nil
}

// fetchContent scarica il contenuto di una URL
func (s *WebScraper) fetchContent(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// parseMarkdownForAPIs estrae API dal contenuto markdown
func (s *WebScraper) parseMarkdownForAPIs(content, source string) []Candidate {
	candidates := make([]Candidate, 0)

	// Pattern per trovare link markdown: [testo](url)
	linkPattern := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	matches := linkPattern.FindAllStringSubmatch(content, -1)

	// Pattern per URL API
	apiURLPattern := regexp.MustCompile(`https?://[a-zA-Z0-9.-]+(?:\:[0-9]+)?(?:/[a-zA-Z0-9/_-]*)?`)

	// Pattern per tabelle markdown
	tablePattern := regexp.MustCompile(`(?m)^\|([^|]+)\|([^|]+)\|`)

	// Estrai da link markdown
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		text := match[1]
		url := match[2]

		// Filtra URL non API
		if !s.looksLikeAPI(url) {
			continue
		}

		candidate := s.createCandidateFromURL(url, text, source)
		if candidate != nil {
			candidates = append(candidates, *candidate)
		}
	}

	// Estrai da tabelle
	tableMatches := tablePattern.FindAllStringSubmatch(content, -1)
	for _, match := range tableMatches {
		if len(match) < 3 {
			continue
		}

		row := match[0]
		urls := apiURLPattern.FindAllString(row, -1)

		for _, url := range urls {
			if s.looksLikeAPI(url) {
				candidate := s.createCandidateFromURL(url, "", source)
				if candidate != nil {
					candidates = append(candidates, *candidate)
				}
			}
		}
	}

	// Estrai URL diretti dal contenuto
	directURLs := apiURLPattern.FindAllString(content, -1)
	for _, url := range directURLs {
		if s.looksLikeAPI(url) {
			// Controlla se non è già stato trovato
			found := false
			for _, c := range candidates {
				if c.BaseURL == url {
					found = true
					break
				}
			}

			if !found {
				candidate := s.createCandidateFromURL(url, "", source)
				if candidate != nil {
					candidates = append(candidates, *candidate)
				}
			}
		}
	}

	return candidates
}

// looksLikeAPI verifica se una URL sembra un endpoint API
func (s *WebScraper) looksLikeAPI(url string) bool {
	urlLower := strings.ToLower(url)

	// Filtra URL non API
	excludePatterns := []string{
		"github.com",
		"youtube.com",
		"twitter.com",
		"facebook.com",
		"discord.gg",
		"t.me",
		"reddit.com",
		"medium.com",
		"shields.io",
		"badge",
		".png",
		".jpg",
		".gif",
		".svg",
		".pdf",
		"wiki",
		"docs.python",
		"docs.rs",
		"doc.rust",
	}

	for _, pattern := range excludePatterns {
		if strings.Contains(urlLower, pattern) {
			return false
		}
	}

	// Pattern positivi per API
	apiKeywords := []string{
		"api",
		"/v1/",
		"/v2/",
		"chat",
		"completions",
		"generate",
		"inference",
		"proxy",
		"gateway",
		"openai",
		"anthropic",
		"claude",
		"gpt",
	}

	for _, keyword := range apiKeywords {
		if strings.Contains(urlLower, keyword) {
			return true
		}
	}

	return false
}

// createCandidateFromURL crea un candidato da una URL
func (s *WebScraper) createCandidateFromURL(url, description, source string) *Candidate {
	// Pulisci l'URL
	url = strings.TrimSpace(url)
	url = strings.TrimRight(url, "/.,;")

	// Estrai nome dall'URL o dalla descrizione
	name := description
	if name == "" {
		// Estrai dal dominio
		parts := strings.Split(url, "/")
		if len(parts) >= 3 {
			domain := parts[2]
			name = strings.Split(domain, ".")[0]
		}
	}

	// Determina tipo di auth (euristica)
	authType := models.AuthTypeAPIKey // default
	descLower := strings.ToLower(description)

	if strings.Contains(descLower, "no auth") ||
		strings.Contains(descLower, "free") ||
		strings.Contains(descLower, "without key") {
		authType = models.AuthTypeNone
	}

	// Normalizza l'URL
	baseURL := s.normalizeURL(url)

	return &Candidate{
		Name:        name,
		BaseURL:     baseURL,
		AuthType:    authType,
		Source:      "scraper:" + source,
		Description: description,
	}
}

// normalizeURL normalizza una URL API
func (s *WebScraper) normalizeURL(url string) string {
	// Rimuovi trailing slash
	url = strings.TrimSuffix(url, "/")

	// Se termina con endpoint specifici, rimuovili per ottenere la base URL
	endpoints := []string{
		"/chat/completions",
		"/v1/chat/completions",
		"/v1/completions",
		"/v1/messages",
		"/completions",
		"/messages",
	}

	for _, endpoint := range endpoints {
		if strings.HasSuffix(url, endpoint) {
			// Rimuovi l'endpoint specifico
			url = strings.TrimSuffix(url, endpoint)
			break
		}
	}

	return url
}

// ScrapeGitHubProjects cerca progetti su GitHub tramite topics
func (s *WebScraper) ScrapeGitHubProjects(ctx context.Context) ([]Candidate, error) {
	// URL del topic GitHub
	topics := []string{
		"https://github.com/topics/free-gpt",
		"https://github.com/topics/chatgpt-api",
		"https://github.com/topics/ai-proxy",
		"https://github.com/topics/llm-api",
	}

	candidates := make([]Candidate, 0)

	for _, topicURL := range topics {
		s.logger.Debug().Str("url", topicURL).Msg("Scraping GitHub topic")

		content, err := s.fetchContent(ctx, topicURL)
		if err != nil {
			s.logger.Warn().Err(err).Str("url", topicURL).Msg("Failed to scrape topic")
			continue
		}

		// Estrai repository URL
		repoPattern := regexp.MustCompile(`https://github\.com/([^/]+)/([^"/\s]+)`)
		matches := repoPattern.FindAllStringSubmatch(content, -1)

		seen := make(map[string]bool)
		for _, match := range matches {
			if len(match) < 3 {
				continue
			}

			repoURL := fmt.Sprintf("https://github.com/%s/%s", match[1], match[2])
			if seen[repoURL] {
				continue
			}
			seen[repoURL] = true

			// Nota: qui si potrebbe fare fetch del README e analisi
			// Ma per evitare troppe richieste, lo lasciamo al GitHub discovery engine
		}
	}

	return candidates, nil
}

// ScrapeDocumentation cerca nelle documentazioni comuni
func (s *WebScraper) ScrapeDocumentation(ctx context.Context, urls []string) ([]Candidate, error) {
	candidates := make([]Candidate, 0)

	for _, url := range urls {
		s.logger.Debug().Str("url", url).Msg("Scraping documentation")

		content, err := s.fetchContent(ctx, url)
		if err != nil {
			s.logger.Warn().Err(err).Str("url", url).Msg("Failed to fetch documentation")
			continue
		}

		// Cerca pattern di configurazione
		configPatterns := []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:base[-_]?url|api[-_]?url|endpoint):\s*["']?(https?://[^"'\s]+)`),
			regexp.MustCompile(`(?i)OPENAI_API_BASE\s*=\s*["']?(https?://[^"'\s]+)`),
			regexp.MustCompile(`(?i)ANTHROPIC_API_URL\s*=\s*["']?(https?://[^"'\s]+)`),
		}

		for _, pattern := range configPatterns {
			matches := pattern.FindAllStringSubmatch(content, -1)
			for _, match := range matches {
				if len(match) < 2 {
					continue
				}

				apiURL := match[1]
				if s.looksLikeAPI(apiURL) {
					candidate := s.createCandidateFromURL(apiURL, "", "documentation")
					if candidate != nil {
						candidates = append(candidates, *candidate)
					}
				}
			}
		}
	}

	return candidates, nil
}
