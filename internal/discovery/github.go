package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/rs/zerolog"
)

// GitHubDiscovery gestisce la ricerca di API su GitHub
type GitHubDiscovery struct {
	token      string
	httpClient *http.Client
	logger     zerolog.Logger
}

// GitHubRepo rappresenta un repository GitHub
type GitHubRepo struct {
	Name            string    `json:"name"`
	FullName        string    `json:"full_name"`
	Description     string    `json:"description"`
	HTMLURL         string    `json:"html_url"`
	StargazersCount int       `json:"stargazers_count"`
	Language        string    `json:"language"`
	UpdatedAt       time.Time `json:"updated_at"`
	DefaultBranch   string    `json:"default_branch"`
}

// GitHubSearchResponse rappresenta la risposta della GitHub Search API
type GitHubSearchResponse struct {
	TotalCount int          `json:"total_count"`
	Items      []GitHubRepo `json:"items"`
}

// NewGitHubDiscovery crea un nuovo GitHub discovery client
func NewGitHubDiscovery(token string, logger zerolog.Logger) *GitHubDiscovery {
	return &GitHubDiscovery{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger.With().Str("component", "github_discovery").Logger(),
	}
}

// DiscoverAPIs cerca repository su GitHub con API gratuite
func (g *GitHubDiscovery) DiscoverAPIs(ctx context.Context, searchTerms []string) ([]Candidate, error) {
	candidates := make([]Candidate, 0)

	for _, term := range searchTerms {
		repos, err := g.searchRepositories(ctx, term)
		if err != nil {
			g.logger.Error().Err(err).Str("term", term).Msg("GitHub search failed")
			continue
		}

		g.logger.Info().
			Str("term", term).
			Int("repos_found", len(repos)).
			Msg("GitHub search completed")

		for _, repo := range repos {
			// Filtra repository spam o non rilevanti
			if !g.isRelevantRepo(repo) {
				continue
			}

			// Scarica e analizza il README
			readme, err := g.fetchReadme(ctx, repo)
			if err != nil {
				g.logger.Debug().
					Err(err).
					Str("repo", repo.FullName).
					Msg("Failed to fetch README")
				continue
			}

			// Estrai endpoint dal README
			endpoints := g.extractEndpoints(readme)
			if len(endpoints) == 0 {
				continue
			}

			// Crea candidati dai risultati
			for _, endpoint := range endpoints {
				candidate := Candidate{
					Name:        repo.Name,
					BaseURL:     endpoint.URL,
					AuthType:    endpoint.AuthType,
					Source:      "github",
					RepoURL:     repo.HTMLURL,
					Description: repo.Description,
					Stars:       repo.StargazersCount,
					Language:    repo.Language,
					LastUpdate:  repo.UpdatedAt,
					Models:      endpoint.Models,
				}
				candidates = append(candidates, candidate)
			}
		}
	}

	return candidates, nil
}

// searchRepositories cerca repository su GitHub
func (g *GitHubDiscovery) searchRepositories(ctx context.Context, query string) ([]GitHubRepo, error) {
	// Costruisci query con filtri
	searchQuery := fmt.Sprintf("%s language:go stars:>10 pushed:>%s",
		query,
		time.Now().AddDate(0, -6, 0).Format("2006-01-02"),
	)

	apiURL := fmt.Sprintf("https://api.github.com/search/repositories?q=%s&sort=stars&order=desc&per_page=30",
		url.QueryEscape(searchQuery))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	// Aggiungi headers
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if g.token != "" {
		req.Header.Set("Authorization", "token "+g.token)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API error: %d - %s", resp.StatusCode, string(body))
	}

	var searchResp GitHubSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	return searchResp.Items, nil
}

// fetchReadme scarica il README di un repository
func (g *GitHubDiscovery) fetchReadme(ctx context.Context, repo GitHubRepo) (string, error) {
	// Prova vari formati di README
	readmeNames := []string{"README.md", "README", "readme.md", "Readme.md"}

	for _, name := range readmeNames {
		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s",
			repo.FullName, repo.DefaultBranch, name)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}

		resp, err := g.httpClient.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				continue
			}
			return string(body), nil
		}
		resp.Body.Close()
	}

	return "", fmt.Errorf("README not found")
}

// EndpointInfo contiene informazioni su un endpoint estratto
type EndpointInfo struct {
	URL      string
	AuthType models.AuthType
	Models   []string
}

// extractEndpoints estrae endpoint API dal README
func (g *GitHubDiscovery) extractEndpoints(readme string) []EndpointInfo {
	endpoints := make([]EndpointInfo, 0)

	// Pattern per trovare URL di API
	urlPatterns := []*regexp.Regexp{
		// API endpoints comuni
		regexp.MustCompile(`https?://[a-zA-Z0-9.-]+(?:\:[0-9]+)?/v[0-9]+(?:/[a-zA-Z0-9/_-]*)?`),
		regexp.MustCompile(`https?://api\.[a-zA-Z0-9.-]+(?:\:[0-9]+)?(?:/[a-zA-Z0-9/_-]*)?`),
		regexp.MustCompile(`(?:base[-_]?url|endpoint|api[-_]?url):\s*(https?://[^\s\)]+)`),
	}

	// Pattern per determinare il tipo di auth
	authPatterns := map[models.AuthType]*regexp.Regexp{
		models.AuthTypeNone:   regexp.MustCompile(`(?i)no\s+auth|without\s+auth|free\s+access|no\s+api\s+key`),
		models.AuthTypeAPIKey: regexp.MustCompile(`(?i)api[-_]?key|x-api-key`),
		models.AuthTypeBearer: regexp.MustCompile(`(?i)bearer\s+token|authorization:\s*bearer`),
	}

	// Pattern per trovare modelli supportati
	modelPatterns := regexp.MustCompile(`(?i)(?:gpt-[34][-.0-9]*|claude-[a-z0-9-]+|gemini-[a-z0-9-]+|llama-[0-9]+)`)

	readmeLower := strings.ToLower(readme)

	// Estrai URL
	urlSet := make(map[string]bool)
	for _, pattern := range urlPatterns {
		matches := pattern.FindAllString(readme, -1)
		for _, match := range matches {
			// Pulisci l'URL
			match = strings.TrimRight(match, ".,;)")

			// Filtra URL non validi
			if !g.isValidAPIURL(match) {
				continue
			}

			urlSet[match] = true
		}
	}

	// Per ogni URL trovato, determina auth e modelli
	for urlStr := range urlSet {
		authType := models.AuthTypeAPIKey // default

		// Determina auth type dal contesto
		for auth, pattern := range authPatterns {
			if pattern.MatchString(readmeLower) {
				authType = auth
				break
			}
		}

		// Estrai modelli supportati
		models := make([]string, 0)
		modelMatches := modelPatterns.FindAllString(readme, -1)
		modelSet := make(map[string]bool)
		for _, model := range modelMatches {
			modelLower := strings.ToLower(model)
			if !modelSet[modelLower] {
				modelSet[modelLower] = true
				models = append(models, modelLower)
			}
		}

		endpoints = append(endpoints, EndpointInfo{
			URL:      urlStr,
			AuthType: authType,
			Models:   models,
		})
	}

	return endpoints
}

// isValidAPIURL verifica se un URL è un endpoint API valido
func (g *GitHubDiscovery) isValidAPIURL(urlStr string) bool {
	// Filtra URL non API
	invalidPatterns := []string{
		"github.com",
		"githubusercontent.com",
		"shields.io",
		"badge",
		"travis-ci",
		"circleci",
		"codecov",
		"twitter.com",
		"facebook.com",
		"linkedin.com",
		"youtube.com",
		"google.com",
		"example.com",
		"localhost",
		"127.0.0.1",
		".png",
		".jpg",
		".gif",
		".svg",
	}

	urlLower := strings.ToLower(urlStr)
	for _, pattern := range invalidPatterns {
		if strings.Contains(urlLower, pattern) {
			return false
		}
	}

	// Verifica che sia un URL valido
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Deve avere schema http/https e host
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	if parsed.Host == "" {
		return false
	}

	// Parole chiave positive per API
	positiveKeywords := []string{
		"api",
		"v1",
		"v2",
		"chat",
		"completions",
		"generate",
		"inference",
		"proxy",
		"gateway",
	}

	for _, keyword := range positiveKeywords {
		if strings.Contains(urlLower, keyword) {
			return true
		}
	}

	return false
}

// isRelevantRepo verifica se il repository è rilevante
func (g *GitHubDiscovery) isRelevantRepo(repo GitHubRepo) bool {
	// Filtri minimi
	if repo.StargazersCount < 5 {
		return false
	}

	// Deve essere aggiornato negli ultimi 12 mesi
	if time.Since(repo.UpdatedAt) > 365*24*time.Hour {
		return false
	}

	// Filtra repository spam
	descLower := strings.ToLower(repo.Description)
	nameLower := strings.ToLower(repo.Name)

	spamKeywords := []string{
		"test",
		"demo",
		"example",
		"tutorial",
		"course",
		"homework",
		"assignment",
	}

	for _, keyword := range spamKeywords {
		if strings.Contains(nameLower, keyword) || strings.Contains(descLower, keyword) {
			return false
		}
	}

	// Parole chiave positive
	relevantKeywords := []string{
		"api",
		"proxy",
		"gateway",
		"llm",
		"ai",
		"gpt",
		"claude",
		"gemini",
		"openai",
		"anthropic",
		"free",
	}

	for _, keyword := range relevantKeywords {
		if strings.Contains(nameLower, keyword) || strings.Contains(descLower, keyword) {
			return true
		}
	}

	return false
}
