package agents

import (
	"strings"

	"github.com/biodoia/goleapifree/internal/providers"
)

// TaskType rappresenta il tipo di task
type TaskType string

const (
	TaskTypeCoding      TaskType = "coding"
	TaskTypeCreative    TaskType = "creative"
	TaskTypeAnalysis    TaskType = "analysis"
	TaskTypeTranslation TaskType = "translation"
	TaskTypeFast        TaskType = "fast"
	TaskTypeGeneral     TaskType = "general"
)

// TaskContext contiene informazioni sul contesto del task
type TaskContext struct {
	// Tipo di task identificato
	TaskType TaskType

	// Prompt originale
	Prompt string

	// Keywords trovate nel prompt
	Keywords []string

	// Richiede risposta veloce
	RequiresFastResponse bool

	// Richiede alta qualità
	RequiresHighQuality bool

	// Lingua rilevata
	Language string

	// Complessità stimata (0.0-1.0)
	Complexity float64

	// Score di confidenza per il task type (0.0-1.0)
	Confidence float64

	// Metadata aggiuntivi
	Metadata map[string]interface{}
}

// Task rappresenta un task da eseguire
type Task struct {
	// Tipo di task
	Type TaskType

	// Modello da utilizzare
	Model string

	// Messaggi della conversazione
	Messages []providers.Message

	// Parametri
	Temperature *float64
	MaxTokens   *int

	// Metadata
	Metadata map[string]interface{}
}

// TaskResult rappresenta il risultato di un task
type TaskResult struct {
	// Tipo di agente che ha eseguito il task
	AgentType AgentType

	// Contenuto della risposta
	Content string

	// Modello utilizzato
	Model string

	// Usage statistics
	Usage providers.Usage

	// Metadata
	Metadata map[string]interface{}
}

// ContextAnalyzer analizza il contesto per determinare il tipo di task
type ContextAnalyzer struct {
	// Keyword mappings per ogni tipo di task
	keywordMappings map[TaskType][]string

	// Threshold per confidenza minima
	confidenceThreshold float64
}

// NewContextAnalyzer crea un nuovo ContextAnalyzer
func NewContextAnalyzer() *ContextAnalyzer {
	return &ContextAnalyzer{
		keywordMappings: map[TaskType][]string{
			TaskTypeCoding: {
				"code", "function", "class", "method", "variable",
				"debug", "error", "bug", "implement", "algorithm",
				"programming", "syntax", "compile", "refactor",
				"api", "library", "framework", "import", "export",
				"test", "unittest", "python", "javascript", "go",
				"java", "rust", "c++", "typescript", "sql",
			},
			TaskTypeCreative: {
				"write", "story", "creative", "imagine", "brainstorm",
				"content", "marketing", "blog", "article", "essay",
				"poem", "narrative", "character", "plot", "scene",
				"describe", "invent", "create", "design", "generate",
				"slogan", "tagline", "copy", "advertisement",
			},
			TaskTypeAnalysis: {
				"analyze", "analysis", "research", "study", "investigate",
				"explain", "understand", "reason", "why", "how",
				"compare", "evaluate", "assess", "examine", "review",
				"summarize", "summary", "extract", "insight", "conclusion",
				"data", "statistics", "trend", "pattern", "correlation",
			},
			TaskTypeTranslation: {
				"translate", "translation", "language", "italian", "english",
				"spanish", "french", "german", "chinese", "japanese",
				"localize", "localization", "multilingual", "interpreter",
				"convert language", "from english to", "to italian",
			},
			TaskTypeFast: {
				"quick", "fast", "brief", "short", "simple",
				"rapid", "immediate", "instant", "quickly",
				"in few words", "concise", "summary", "tldr",
			},
		},
		confidenceThreshold: 0.3,
	}
}

// Analyze analizza il prompt e determina il tipo di task
func (ca *ContextAnalyzer) Analyze(prompt string, messages []providers.Message) *TaskContext {
	ctx := &TaskContext{
		Prompt:   prompt,
		Keywords: []string{},
		Metadata: make(map[string]interface{}),
	}

	// Combina tutti i messaggi per l'analisi
	fullText := prompt
	for _, msg := range messages {
		if content, ok := msg.Content.(string); ok {
			fullText += " " + content
		}
	}
	fullText = strings.ToLower(fullText)

	// Calcola score per ogni tipo di task
	scores := make(map[TaskType]float64)
	maxScore := 0.0
	var bestTaskType TaskType

	for taskType, keywords := range ca.keywordMappings {
		score := ca.calculateScore(fullText, keywords)
		scores[taskType] = score

		if score > maxScore {
			maxScore = score
			bestTaskType = taskType
		}
	}

	// Determina il task type basato sullo score più alto
	if maxScore >= ca.confidenceThreshold {
		ctx.TaskType = bestTaskType
		ctx.Confidence = maxScore
	} else {
		ctx.TaskType = TaskTypeGeneral
		ctx.Confidence = 1.0 // Generale è sempre sicuro
	}

	// Rileva keywords specifiche
	ctx.Keywords = ca.extractKeywords(fullText, ca.keywordMappings[ctx.TaskType])

	// Determina se richiede risposta veloce
	ctx.RequiresFastResponse = ca.requiresFastResponse(fullText)

	// Determina se richiede alta qualità
	ctx.RequiresHighQuality = ca.requiresHighQuality(fullText)

	// Stima complessità
	ctx.Complexity = ca.estimateComplexity(fullText)

	// Rileva lingua
	ctx.Language = ca.detectLanguage(fullText)

	// Aggiungi scores a metadata
	ctx.Metadata["task_scores"] = scores

	return ctx
}

// calculateScore calcola lo score per un set di keywords
func (ca *ContextAnalyzer) calculateScore(text string, keywords []string) float64 {
	matchCount := 0
	totalKeywords := len(keywords)

	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			matchCount++
		}
	}

	// Score normalizzato (0.0-1.0)
	if totalKeywords == 0 {
		return 0.0
	}

	return float64(matchCount) / float64(totalKeywords)
}

// extractKeywords estrae le keywords trovate nel testo
func (ca *ContextAnalyzer) extractKeywords(text string, keywords []string) []string {
	found := []string{}
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			found = append(found, keyword)
		}
	}
	return found
}

// requiresFastResponse determina se il task richiede risposta veloce
func (ca *ContextAnalyzer) requiresFastResponse(text string) bool {
	fastIndicators := []string{
		"quick", "fast", "brief", "short", "urgent",
		"immediately", "asap", "right now", "hurry",
	}

	for _, indicator := range fastIndicators {
		if strings.Contains(text, indicator) {
			return true
		}
	}

	return false
}

// requiresHighQuality determina se il task richiede alta qualità
func (ca *ContextAnalyzer) requiresHighQuality(text string) bool {
	qualityIndicators := []string{
		"detailed", "comprehensive", "thorough", "in-depth",
		"complete", "extensive", "elaborate", "professional",
		"high quality", "best", "perfect", "excellent",
	}

	for _, indicator := range qualityIndicators {
		if strings.Contains(text, indicator) {
			return true
		}
	}

	return false
}

// estimateComplexity stima la complessità del task
func (ca *ContextAnalyzer) estimateComplexity(text string) float64 {
	// Fattori che indicano complessità
	length := len(text)
	words := len(strings.Fields(text))

	complexityScore := 0.0

	// Lunghezza del testo
	if length > 1000 {
		complexityScore += 0.3
	} else if length > 500 {
		complexityScore += 0.2
	} else if length > 200 {
		complexityScore += 0.1
	}

	// Numero di parole
	if words > 150 {
		complexityScore += 0.2
	} else if words > 75 {
		complexityScore += 0.1
	}

	// Presenza di termini tecnici
	technicalTerms := []string{
		"algorithm", "architecture", "optimization", "implementation",
		"infrastructure", "distributed", "scalable", "concurrent",
		"asynchronous", "multithreaded", "database", "performance",
	}

	technicalCount := 0
	for _, term := range technicalTerms {
		if strings.Contains(text, term) {
			technicalCount++
		}
	}

	if technicalCount > 5 {
		complexityScore += 0.3
	} else if technicalCount > 2 {
		complexityScore += 0.2
	}

	// Presenza di richieste multiple
	if strings.Count(text, "?") > 2 {
		complexityScore += 0.1
	}

	if strings.Contains(text, "step by step") ||
	   strings.Contains(text, "explain in detail") {
		complexityScore += 0.1
	}

	// Normalizza tra 0.0 e 1.0
	if complexityScore > 1.0 {
		complexityScore = 1.0
	}

	return complexityScore
}

// detectLanguage rileva la lingua del testo
func (ca *ContextAnalyzer) detectLanguage(text string) string {
	// Semplice rilevamento basato su parole comuni
	// In produzione si userebbe una libreria più sofisticata

	italianWords := []string{"il", "la", "di", "da", "per", "con", "sono", "è", "che"}
	englishWords := []string{"the", "is", "are", "and", "or", "to", "from", "for", "with"}
	spanishWords := []string{"el", "la", "de", "para", "con", "es", "que", "por"}
	frenchWords := []string{"le", "la", "de", "pour", "avec", "est", "que", "dans"}

	italianCount := 0
	englishCount := 0
	spanishCount := 0
	frenchCount := 0

	words := strings.Fields(text)
	for _, word := range words {
		word = strings.ToLower(word)

		for _, iw := range italianWords {
			if word == iw {
				italianCount++
			}
		}
		for _, ew := range englishWords {
			if word == ew {
				englishCount++
			}
		}
		for _, sw := range spanishWords {
			if word == sw {
				spanishCount++
			}
		}
		for _, fw := range frenchWords {
			if word == fw {
				frenchCount++
			}
		}
	}

	// Determina la lingua più probabile
	maxCount := italianCount
	lang := "it"

	if englishCount > maxCount {
		maxCount = englishCount
		lang = "en"
	}
	if spanishCount > maxCount {
		maxCount = spanishCount
		lang = "es"
	}
	if frenchCount > maxCount {
		lang = "fr"
	}

	return lang
}

// HasKeywords verifica se il contesto contiene almeno una delle keywords
func (tc *TaskContext) HasKeywords(keywords []string) bool {
	text := strings.ToLower(tc.Prompt)

	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}
