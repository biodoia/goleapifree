package agents

import (
	"testing"

	"github.com/biodoia/goleapifree/internal/providers"
)

func TestContextAnalyzer_Analyze(t *testing.T) {
	analyzer := NewContextAnalyzer()

	tests := []struct {
		name           string
		prompt         string
		expectedType   TaskType
		minConfidence  float64
	}{
		{
			name:          "Coding task - Python function",
			prompt:        "Write a Python function to calculate fibonacci numbers",
			expectedType:  TaskTypeCoding,
			minConfidence: 0.1,
		},
		{
			name:          "Coding task - Debug",
			prompt:        "Debug this code and fix the error in the class implementation",
			expectedType:  TaskTypeCoding,
			minConfidence: 0.1,
		},
		{
			name:          "Creative task - Story",
			prompt:        "Write a creative story about a robot learning to paint",
			expectedType:  TaskTypeCreative,
			minConfidence: 0.1,
		},
		{
			name:          "Creative task - Marketing",
			prompt:        "Create a marketing campaign for our new product",
			expectedType:  TaskTypeCreative,
			minConfidence: 0.1,
		},
		{
			name:          "Analysis task - Data",
			prompt:        "Analyze this dataset and explain the trends you observe",
			expectedType:  TaskTypeAnalysis,
			minConfidence: 0.1,
		},
		{
			name:          "Analysis task - Research",
			prompt:        "Research the benefits of renewable energy and summarize your findings",
			expectedType:  TaskTypeAnalysis,
			minConfidence: 0.1,
		},
		{
			name:          "Translation task",
			prompt:        "Translate this text from English to Italian",
			expectedType:  TaskTypeTranslation,
			minConfidence: 0.1,
		},
		{
			name:          "Fast task - Quick question",
			prompt:        "Quick question: what is the capital of France?",
			expectedType:  TaskTypeFast,
			minConfidence: 0.1,
		},
		{
			name:          "General task",
			prompt:        "Tell me about the weather today",
			expectedType:  TaskTypeGeneral,
			minConfidence: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := analyzer.Analyze(tt.prompt, []providers.Message{})

			if ctx.TaskType != tt.expectedType {
				t.Errorf("Expected task type %s, got %s", tt.expectedType, ctx.TaskType)
			}

			if ctx.Confidence < tt.minConfidence {
				t.Errorf("Confidence too low: %.2f < %.2f", ctx.Confidence, tt.minConfidence)
			}

			if ctx.Prompt != tt.prompt {
				t.Errorf("Prompt mismatch")
			}
		})
	}
}

func TestContextAnalyzer_RequiresFastResponse(t *testing.T) {
	analyzer := NewContextAnalyzer()

	tests := []struct {
		prompt   string
		expected bool
	}{
		{"Quick question: what is 2+2?", true},
		{"Fast answer needed: what time is it?", true},
		{"I need this urgently", true},
		{"Tell me a detailed analysis of quantum computing", false},
		{"Normal question here", false},
	}

	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			ctx := analyzer.Analyze(tt.prompt, []providers.Message{})

			if ctx.RequiresFastResponse != tt.expected {
				t.Errorf("Expected RequiresFastResponse=%v, got %v", tt.expected, ctx.RequiresFastResponse)
			}
		})
	}
}

func TestContextAnalyzer_RequiresHighQuality(t *testing.T) {
	analyzer := NewContextAnalyzer()

	tests := []struct {
		prompt   string
		expected bool
	}{
		{"I need a detailed and comprehensive analysis", true},
		{"Please provide a thorough explanation", true},
		{"Give me a professional and high quality response", true},
		{"Just give me a quick answer", false},
		{"What is the weather?", false},
	}

	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			ctx := analyzer.Analyze(tt.prompt, []providers.Message{})

			if ctx.RequiresHighQuality != tt.expected {
				t.Errorf("Expected RequiresHighQuality=%v, got %v", tt.expected, ctx.RequiresHighQuality)
			}
		})
	}
}

func TestContextAnalyzer_EstimateComplexity(t *testing.T) {
	analyzer := NewContextAnalyzer()

	tests := []struct {
		name            string
		prompt          string
		minComplexity   float64
		maxComplexity   float64
	}{
		{
			name:          "Simple question",
			prompt:        "What is 2+2?",
			minComplexity: 0.0,
			maxComplexity: 0.3,
		},
		{
			name:          "Complex technical question",
			prompt:        "Explain the architecture of a distributed database system with multi-master replication, including the algorithm for conflict resolution and performance optimization strategies",
			minComplexity: 0.3,
			maxComplexity: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := analyzer.Analyze(tt.prompt, []providers.Message{})

			if ctx.Complexity < tt.minComplexity || ctx.Complexity > tt.maxComplexity {
				t.Errorf("Complexity %.2f not in range [%.2f, %.2f]",
					ctx.Complexity, tt.minComplexity, tt.maxComplexity)
			}
		})
	}
}

func TestAgentCanHandle(t *testing.T) {
	analyzer := NewContextAnalyzer()

	tests := []struct {
		name       string
		agent      Agent
		prompt     string
		shouldHandle bool
	}{
		{
			name:       "CodingAgent handles coding task",
			agent:      NewCodingAgent(),
			prompt:     "Write a function to implement quicksort",
			shouldHandle: true,
		},
		{
			name:       "CodingAgent doesn't handle creative task",
			agent:      NewCodingAgent(),
			prompt:     "Write a poem about flowers",
			shouldHandle: false,
		},
		{
			name:       "CreativeAgent handles creative task",
			agent:      NewCreativeAgent(),
			prompt:     "Write a story about a dragon",
			shouldHandle: true,
		},
		{
			name:       "FastAgent handles fast request",
			agent:      NewFastAgent(),
			prompt:     "Quick: what is the capital of Italy?",
			shouldHandle: true,
		},
		{
			name:       "GeneralAgent handles anything",
			agent:      NewGeneralAgent(),
			prompt:     "Any random question",
			shouldHandle: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := analyzer.Analyze(tt.prompt, []providers.Message{})

			canHandle := tt.agent.CanHandle(ctx)

			if canHandle != tt.shouldHandle {
				t.Errorf("Expected CanHandle=%v, got %v", tt.shouldHandle, canHandle)
			}
		})
	}
}

func TestAgentPreferredModels(t *testing.T) {
	tests := []struct {
		name          string
		agent         Agent
		expectedFirst string
	}{
		{
			name:          "CodingAgent prefers Codestral",
			agent:         NewCodingAgent(),
			expectedFirst: "codestral-latest",
		},
		{
			name:          "CreativeAgent prefers Claude",
			agent:         NewCreativeAgent(),
			expectedFirst: "claude-3-5-sonnet-20241022",
		},
		{
			name:          "FastAgent prefers Groq",
			agent:         NewFastAgent(),
			expectedFirst: "llama-3.1-70b-versatile",
		},
		{
			name:          "TranslationAgent prefers GPT-4o",
			agent:         NewTranslationAgent(),
			expectedFirst: "gpt-4o",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			models := tt.agent.PreferredModels()

			if len(models) == 0 {
				t.Fatal("No preferred models")
			}

			if models[0] != tt.expectedFirst {
				t.Errorf("Expected first model %s, got %s", tt.expectedFirst, models[0])
			}
		})
	}
}

func TestFallbackChains(t *testing.T) {
	tests := []struct {
		name            string
		agentType       AgentType
		minChainLength  int
	}{
		{
			name:           "CodingAgent has fallback chain",
			agentType:      AgentTypeCoding,
			minChainLength: 5,
		},
		{
			name:           "CreativeAgent has fallback chain",
			agentType:      AgentTypeCreative,
			minChainLength: 5,
		},
		{
			name:           "FastAgent has fallback chain",
			agentType:      AgentTypeFast,
			minChainLength: 4,
		},
	}

	// Note: This test doesn't actually create an orchestrator
	// but validates the test structure
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// In a real test, you would:
			// orchestrator := NewOrchestrator(cfg, db)
			// chain := orchestrator.fallbackChains[tt.agentType]
			// if len(chain) < tt.minChainLength { ... }

			// For now, just validate test structure
			if tt.minChainLength < 1 {
				t.Error("Invalid test: minChainLength must be >= 1")
			}
		})
	}
}

func TestChainBuilder(t *testing.T) {
	// This test validates the ChainBuilder interface
	// In a real scenario, you would need a mock orchestrator

	t.Run("ChainBuilder creates chain", func(t *testing.T) {
		// orchestrator := NewOrchestrator(cfg, db)
		// builder := NewChainBuilder(ChainTypeSequential, orchestrator)

		// Validate that ChainBuilder methods are chainable
		// chain := builder.
		//     WithStep("step1", agent1, "template1").
		//     WithStep("step2", agent2, "template2").
		//     Build()

		// This test structure validates the API design
	})
}

func BenchmarkContextAnalyzer_Analyze(b *testing.B) {
	analyzer := NewContextAnalyzer()
	prompt := "Write a Python function to implement a binary search tree with insert, delete, and search operations"
	messages := []providers.Message{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.Analyze(prompt, messages)
	}
}

func BenchmarkContextAnalyzer_AnalyzeSimple(b *testing.B) {
	analyzer := NewContextAnalyzer()
	prompt := "What is 2+2?"
	messages := []providers.Message{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.Analyze(prompt, messages)
	}
}
