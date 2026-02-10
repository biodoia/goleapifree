package agents_test

import (
	"context"
	"fmt"
	"log"

	"github.com/biodoia/goleapifree/internal/agents"
	"github.com/biodoia/goleapifree/internal/providers"
	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
)

// ExampleOrchestrator mostra come utilizzare l'orchestrator
func ExampleOrchestrator() {
	// Setup
	cfg, _ := config.Load("")
	db, _ := database.New(&cfg.Database)

	// Crea orchestrator
	orchestrator := agents.NewOrchestrator(cfg, db)

	// Registra providers (esempio - in produzione useresti provider reali)
	// orchestrator.RegisterProvider(openaiProvider)
	// orchestrator.RegisterProvider(anthropicProvider)

	ctx := context.Background()

	// Caso 1: Coding task
	codingMessages := []providers.Message{
		{Role: "user", Content: "Write a Python function to calculate fibonacci numbers"},
	}

	result, err := orchestrator.Execute(ctx, "Write a Python function to calculate fibonacci numbers", codingMessages)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Agent: %s\n", result.AgentType)
	fmt.Printf("Model: %s\n", result.Model)
	fmt.Printf("Response: %s\n", result.Content)

	// Output example:
	// Agent: coding
	// Model: codestral-latest
	// Response: [Python code for fibonacci]
}

// ExampleChain mostra come utilizzare chain multi-agent
func ExampleChain() {
	cfg, _ := config.Load("")
	db, _ := database.New(&cfg.Database)

	orchestrator := agents.NewOrchestrator(cfg, db)

	ctx := context.Background()

	// Crea una chain Draft → Refine
	chain := agents.NewChainBuilder(agents.ChainTypeDraftRefine, orchestrator).
		WithStep("draft", agents.NewFastAgent(), "Create a quick draft: %s").
		WithStep("refine", agents.NewCreativeAgent(), "Refine and improve this content").
		Build()

	messages := []providers.Message{
		{Role: "user", Content: "Write a blog post about AI ethics"},
	}

	result, err := chain.Execute(ctx, "Write a blog post about AI ethics", messages)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Final result: %s\n", result.FinalResult.Content)
	fmt.Printf("Draft was created with: %s\n", result.IntermediateResults["draft"].Model)
	fmt.Printf("Refined with: %s\n", result.IntermediateResults["refined"].Model)
}

// ExampleContextAnalyzer mostra come funziona l'analisi del contesto
func ExampleContextAnalyzer() {
	analyzer := agents.NewContextAnalyzer()

	// Caso 1: Coding task
	ctx1 := analyzer.Analyze("Write a function to implement quicksort in Go", []providers.Message{})
	fmt.Printf("Task Type: %s, Confidence: %.2f\n", ctx1.TaskType, ctx1.Confidence)
	// Output: Task Type: coding, Confidence: 0.23

	// Caso 2: Creative task
	ctx2 := analyzer.Analyze("Write a creative story about a robot learning to paint", []providers.Message{})
	fmt.Printf("Task Type: %s, Confidence: %.2f\n", ctx2.TaskType, ctx2.Confidence)
	// Output: Task Type: creative, Confidence: 0.19

	// Caso 3: Translation task
	ctx3 := analyzer.Analyze("Translate this text from English to Italian", []providers.Message{})
	fmt.Printf("Task Type: %s, Confidence: %.2f\n", ctx3.TaskType, ctx3.Confidence)
	// Output: Task Type: translation, Confidence: 0.33

	// Caso 4: Fast response
	ctx4 := analyzer.Analyze("Quick question: what is 2+2?", []providers.Message{})
	fmt.Printf("Task Type: %s, Requires Fast: %v\n", ctx4.TaskType, ctx4.RequiresFastResponse)
	// Output: Task Type: fast, Requires Fast: true
}

// ExampleMultiStepChain mostra reasoning multi-step
func ExampleMultiStepChain() {
	cfg, _ := config.Load("")
	db, _ := database.New(&cfg.Database)

	orchestrator := agents.NewOrchestrator(cfg, db)

	ctx := context.Background()

	// Crea una chain multi-step per problem solving complesso
	chain := agents.NewChain(agents.ChainTypeMultiStep, orchestrator)

	messages := []providers.Message{
		{Role: "user", Content: "Design a scalable microservices architecture for an e-commerce platform"},
	}

	result, err := chain.Execute(ctx, "Design a scalable microservices architecture for an e-commerce platform", messages)
	if err != nil {
		log.Fatal(err)
	}

	// Il multi-step prima decompone il problema, poi lo risolve step by step
	fmt.Printf("Decomposition: %s\n", result.IntermediateResults["decomposition"].Content)
	fmt.Printf("Solution: %s\n", result.FinalResult.Content)
}

// ExampleConsensusChain mostra come ottenere consenso da multiple agents
func ExampleConsensusChain() {
	cfg, _ := config.Load("")
	db, _ := database.New(&cfg.Database)

	orchestrator := agents.NewOrchestrator(cfg, db)

	ctx := context.Background()

	// Crea una chain di consenso
	chain := agents.NewChainBuilder(agents.ChainTypeConsensus, orchestrator).
		WithStep("technical", agents.NewCodingAgent(), "%s").
		WithStep("creative", agents.NewCreativeAgent(), "%s").
		WithStep("analytical", agents.NewAnalysisAgent(), "%s").
		Build()

	messages := []providers.Message{
		{Role: "user", Content: "What are the best practices for API design?"},
	}

	result, err := chain.Execute(ctx, "What are the best practices for API design?", messages)
	if err != nil {
		log.Fatal(err)
	}

	// Il risultato finale è una sintesi delle risposte di tutti gli agenti
	fmt.Printf("Consensus result: %s\n", result.FinalResult.Content)
	fmt.Printf("Based on %d different perspectives\n", len(result.IntermediateResults))
}

// ExampleAdvancedRouting mostra routing avanzato con opzioni
func ExampleAdvancedRouting() {
	cfg, _ := config.Load("")
	db, _ := database.New(&cfg.Database)

	orchestrator := agents.NewOrchestrator(cfg, db)

	ctx := context.Background()

	// Forza uso di un agente specifico
	agentType := agents.AgentTypeCoding
	opts := &agents.AdvancedRoutingOptions{
		ForceAgentType:     &agentType,
		RequireHighQuality: true,
	}

	req := &providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: "Implement a binary search tree in Go with full test coverage"},
		},
	}

	result, err := agents.RouteWithOptions(ctx, orchestrator, req, opts)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Result: %s\n", result.Content)
}

// ExampleGetAgentInfo mostra come ottenere info sugli agenti
func ExampleGetAgentInfo() {
	cfg, _ := config.Load("")
	db, _ := database.New(&cfg.Database)

	orchestrator := agents.NewOrchestrator(cfg, db)

	// Ottieni info su tutti gli agenti
	info := orchestrator.GetAgentInfo()

	for agentType, agentInfo := range info {
		fmt.Printf("\nAgent Type: %s\n", agentType)
		if infoMap, ok := agentInfo.(map[string]interface{}); ok {
			fmt.Printf("  Name: %s\n", infoMap["name"])
			fmt.Printf("  Preferred Models: %v\n", infoMap["preferred_models"])
			fmt.Printf("  Fallback Chain: %v\n", infoMap["fallback_chain"])
		}
	}
}
