# Multi-Agent Orchestration System

Sistema di orchestrazione multi-agent per GoLeapAI che fornisce selezione intelligente di modelli LLM basata sul tipo di task.

## Caratteristiche

### Agenti Specializzati

1. **CodingAgent** - Coding tasks
   - Preferenze: Codestral, DeepSeek Coder, Claude, GPT-4
   - Keywords: code, function, class, debug, refactor, implement
   - Temperature: 0.2 (deterministico)

2. **CreativeAgent** - Creative writing
   - Preferenze: Claude, GPT-4, Gemini
   - Keywords: write, story, creative, brainstorm, content
   - Temperature: 0.8 (creativo)

3. **AnalysisAgent** - Analysis & reasoning
   - Preferenze: Claude, GPT-4, Gemini
   - Keywords: analyze, research, summarize, reason, explain
   - Temperature: 0.3 (accurato)

4. **TranslationAgent** - Translation tasks
   - Preferenze: GPT-4, Claude, modelli multilingua
   - Keywords: translate, language, localize
   - Temperature: 0.3 (preciso)

5. **FastAgent** - Quick responses
   - Preferenze: Groq, Cerebras, Gemini Flash
   - Keywords: quick, fast, brief, simple
   - Low latency, high throughput

6. **GeneralAgent** - General purpose
   - Fallback per tutti i task non specializzati

### Context Analyzer

Analizza automaticamente il prompt per determinare:
- **Task Type**: Identifica il tipo di task basandosi su keywords
- **Confidence Score**: Score di confidenza per la classificazione
- **Complexity**: Stima della complessità (0.0-1.0)
- **Language**: Rilevamento automatico della lingua
- **Requirements**: Fast response, high quality, etc.

### Orchestrator

Gestisce:
- **Registry di agenti**: Tutti gli agenti specializzati
- **Provider registry**: Gestione dei provider LLM
- **Model selection**: Selezione automatica del miglior modello
- **Fallback chains**: Catene di fallback per ogni tipo di agente
- **Model availability caching**: Cache per performance

### Agent Chains

Pipeline multi-agent per task complessi:

#### 1. Sequential Chain
Esegue agenti in sequenza, output → input.

```go
chain := NewChainBuilder(ChainTypeSequential, orchestrator).
    WithStep("analyze", analysisAgent, "%s").
    WithStep("implement", codingAgent, "%s").
    Build()
```

#### 2. Parallel Chain
Esegue agenti in parallelo e aggrega risultati.

```go
chain := NewChainBuilder(ChainTypeParallel, orchestrator).
    WithStep("agent1", agent1, "%s").
    WithStep("agent2", agent2, "%s").
    Build()
```

#### 3. Draft-Refine Pattern
Prima bozza veloce, poi raffinamento di qualità.

```go
chain := NewChainBuilder(ChainTypeDraftRefine, orchestrator).
    WithStep("draft", fastAgent, "%s").
    WithStep("refine", creativeAgent, "%s").
    Build()
```

#### 4. Multi-Step Reasoning
Decomposizione del problema + soluzione step-by-step.

```go
chain := NewChain(ChainTypeMultiStep, orchestrator)
result, _ := chain.Execute(ctx, complexProblem, messages)
```

#### 5. Consensus Chain
Multiple agents votano e sintetizzano la risposta migliore.

```go
chain := NewChainBuilder(ChainTypeConsensus, orchestrator).
    WithStep("expert1", agent1, "%s").
    WithStep("expert2", agent2, "%s").
    WithStep("expert3", agent3, "%s").
    Build()
```

## Utilizzo Base

### 1. Setup Orchestrator

```go
import (
    "github.com/biodoia/goleapifree/internal/agents"
    "github.com/biodoia/goleapifree/pkg/config"
    "github.com/biodoia/goleapifree/pkg/database"
)

// Carica config
cfg, _ := config.Load("")
db, _ := database.New(&cfg.Database)

// Crea orchestrator
orchestrator := agents.NewOrchestrator(cfg, db)

// Registra providers
orchestrator.RegisterProvider(openaiProvider)
orchestrator.RegisterProvider(anthropicProvider)
orchestrator.RegisterProvider(groqProvider)
```

### 2. Esecuzione Automatica

```go
ctx := context.Background()

messages := []providers.Message{
    {Role: "user", Content: "Write a Python function to sort a list"},
}

// L'orchestrator seleziona automaticamente:
// - CodingAgent (perché rileva keywords "function", "python")
// - Codestral o DeepSeek Coder (preferiti per coding)
result, err := orchestrator.Execute(ctx,
    "Write a Python function to sort a list",
    messages)

fmt.Printf("Agent: %s\n", result.AgentType)      // coding
fmt.Printf("Model: %s\n", result.Model)           // codestral-latest
fmt.Printf("Content: %s\n", result.Content)
```

### 3. Routing Avanzato

```go
agentType := agents.AgentTypeCoding
model := "deepseek-coder"

opts := &agents.AdvancedRoutingOptions{
    ForceAgentType:      &agentType,
    ForceModel:          &model,
    RequireHighQuality:  true,
}

req := &providers.ChatRequest{
    Messages: messages,
}

result, _ := agents.RouteWithOptions(ctx, orchestrator, req, opts)
```

### 4. Chain Execution

```go
// Draft-Refine pattern per content creation
chain := agents.NewChainBuilder(agents.ChainTypeDraftRefine, orchestrator).
    WithStep("draft", agents.NewFastAgent(), "Create draft: %s").
    WithStep("refine", agents.NewCreativeAgent(), "Improve: %s").
    Build()

messages := []providers.Message{
    {Role: "user", Content: "Write a blog post about AI"},
}

result, _ := chain.Execute(ctx, "Write a blog post about AI", messages)

// Accedi ai risultati intermedi
draftModel := result.IntermediateResults["draft"].Model
refineModel := result.IntermediateResults["refined"].Model
```

## Fallback Chains

Ogni agente ha una fallback chain configurata. Se il modello preferito non è disponibile, prova automaticamente i successivi.

### Esempio: CodingAgent Fallback Chain

1. codestral-latest
2. deepseek-coder
3. deepseek-chat
4. claude-3-5-sonnet-20241022
5. gpt-4o
6. gemini-1.5-pro
7. llama-3.1-70b

```go
// Se codestral-latest fallisce, prova deepseek-coder
// Se anche quello fallisce, prova deepseek-chat
// E così via...
result, err := orchestrator.Execute(ctx, codingPrompt, messages)
```

## Context Analysis

Il ContextAnalyzer analizza il prompt automaticamente:

```go
analyzer := agents.NewContextAnalyzer()

ctx := analyzer.Analyze("Write a function to implement quicksort", messages)

fmt.Printf("Task Type: %s\n", ctx.TaskType)              // coding
fmt.Printf("Confidence: %.2f\n", ctx.Confidence)         // 0.85
fmt.Printf("Complexity: %.2f\n", ctx.Complexity)         // 0.4
fmt.Printf("Keywords: %v\n", ctx.Keywords)               // [write, function, implement]
fmt.Printf("Fast Response: %v\n", ctx.RequiresFastResponse)  // false
fmt.Printf("High Quality: %v\n", ctx.RequiresHighQuality)    // false
```

## Integrazione con Router

```go
import "github.com/biodoia/goleapifree/internal/router"

// Crea router con agent routing strategy
strategy := agents.NewAgentRoutingStrategy(orchestrator)

// Il router ora usa l'orchestrator per selection intelligente
selection, _ := strategy.SelectProvider(req)

fmt.Printf("Provider: %s\n", selection.ProviderID)
fmt.Printf("Model: %s\n", selection.ModelID)
fmt.Printf("Reason: %s\n", selection.Reason)
```

## Agent Info & Monitoring

```go
// Ottieni info sugli agenti registrati
info := orchestrator.GetAgentInfo()

for agentType, details := range info {
    fmt.Printf("Agent: %s\n", agentType)
    fmt.Printf("  Preferred Models: %v\n", details["preferred_models"])
    fmt.Printf("  Fallback Chain: %v\n", details["fallback_chain"])
}

// Ottieni availability dei modelli
availability := orchestrator.GetModelAvailability()
for model, available := range availability {
    fmt.Printf("%s: %v\n", model, available)
}

// Pulisci cache se necessario
orchestrator.ClearModelCache()
```

## Esempi Pratici

### Coding Task
```go
result, _ := orchestrator.Execute(ctx,
    "Implement a REST API in Go with JWT authentication",
    messages)
// Usa: CodingAgent → Codestral/DeepSeek
```

### Creative Writing
```go
result, _ := orchestrator.Execute(ctx,
    "Write a creative story about time travel",
    messages)
// Usa: CreativeAgent → Claude/GPT-4
```

### Data Analysis
```go
result, _ := orchestrator.Execute(ctx,
    "Analyze this dataset and find trends",
    messages)
// Usa: AnalysisAgent → Claude/GPT-4
```

### Quick Question
```go
result, _ := orchestrator.Execute(ctx,
    "Quick: what is the capital of France?",
    messages)
// Usa: FastAgent → Groq/Cerebras (ultra-fast)
```

### Translation
```go
result, _ := orchestrator.Execute(ctx,
    "Translate this text from English to Italian",
    messages)
// Usa: TranslationAgent → GPT-4/Claude
```

### Complex Problem (Multi-Step)
```go
chain := agents.NewChain(agents.ChainTypeMultiStep, orchestrator)
result, _ := chain.Execute(ctx,
    "Design a distributed system for real-time analytics",
    messages)
// Decompose → Analyze → Solve step-by-step
```

### Best Answer (Consensus)
```go
chain := agents.NewChainBuilder(agents.ChainTypeConsensus, orchestrator).
    WithStep("technical", codingAgent, "%s").
    WithStep("creative", creativeAgent, "%s").
    WithStep("analytical", analysisAgent, "%s").
    Build()

result, _ := chain.Execute(ctx, "What is the best programming language?", messages)
// Get perspectives from 3 different agents → synthesize best answer
```

## Configuration

Il sistema usa la configurazione esistente di GoLeapAI:

```yaml
routing:
  strategy: "agent_orchestration"  # Usa agent routing
  failover_enabled: true            # Abilita fallback automatico
  max_retries: 3                    # Max retry per fallback chain
```

## Performance

- **Model Caching**: Cache di availability per evitare health check ripetuti
- **Parallel Execution**: Chains parallele per performance
- **Fast Agents**: Usa provider ultra-veloci (Groq, Cerebras) per quick responses
- **Smart Fallback**: Fallback automatico senza overhead

## Testing

```bash
# Compila
go build ./internal/agents/...

# Test
go test ./internal/agents/... -v

# Examples
go test ./internal/agents/... -v -run Example
```

## Roadmap

- [ ] Learning system per migliorare selezione nel tempo
- [ ] Cost optimization basato su budget
- [ ] Custom agent registration
- [ ] Agent performance metrics
- [ ] A/B testing tra diversi modelli
- [ ] Response quality scoring
- [ ] Dynamic fallback chain adjustment

## Architettura

```
┌─────────────────────────────────────────────────────────┐
│                    Orchestrator                         │
│  ┌────────────────┐  ┌────────────────┐                │
│  │ Agent Registry │  │ Provider Reg.  │                │
│  └────────────────┘  └────────────────┘                │
│  ┌────────────────────────────────────┐                │
│  │     Context Analyzer                │                │
│  │  - Task Type Detection              │                │
│  │  - Complexity Estimation            │                │
│  │  - Language Detection               │                │
│  └────────────────────────────────────┘                │
└─────────────────────────────────────────────────────────┘
                    │
                    ├─────┬─────┬─────┬─────┬─────┐
                    │     │     │     │     │     │
                    ▼     ▼     ▼     ▼     ▼     ▼
              ┌────────┐ │     │     │     │     │
              │ Coding │ │     │     │     │     │
              │ Agent  │ │     │     │     │     │
              └────────┘ │     │     │     │     │
           ┌─────────────┘     │     │     │     │
           ▼                   │     │     │     │
      ┌──────────┐             │     │     │     │
      │Creative  │             │     │     │     │
      │ Agent    │             │     │     │     │
      └──────────┘             │     │     │     │
                ┌──────────────┘     │     │     │
                ▼                    │     │     │
           ┌──────────┐              │     │     │
           │Analysis  │              │     │     │
           │ Agent    │              │     │     │
           └──────────┘              │     │     │
                      ┌──────────────┘     │     │
                      ▼                    │     │
                 ┌─────────────┐           │     │
                 │Translation  │           │     │
                 │   Agent     │           │     │
                 └─────────────┘           │     │
                               ┌───────────┘     │
                               ▼                 │
                          ┌────────┐             │
                          │  Fast  │             │
                          │ Agent  │             │
                          └────────┘             │
                                    ┌────────────┘
                                    ▼
                               ┌──────────┐
                               │ General  │
                               │  Agent   │
                               └──────────┘
```

## Licenza

MIT
