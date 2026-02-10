// Package agents fornisce un sistema di orchestrazione multi-agent per GoLeapAI.
//
// Il sistema include:
//   - Agenti specializzati per diversi tipi di task (coding, creative, analysis, translation, fast)
//   - Orchestrator per la selezione intelligente di agenti e modelli
//   - Context analyzer per l'analisi automatica del tipo di task
//   - Chain per l'esecuzione di pipeline multi-agent (sequential, parallel, draft-refine, multi-step, consensus)
//   - Fallback automatico con catene di modelli alternativi
//   - Routing context-aware basato su keywords e complessità
//
// Esempio di utilizzo base:
//
//	orchestrator := agents.NewOrchestrator(config, db)
//	orchestrator.RegisterProvider(openaiProvider)
//	orchestrator.RegisterProvider(anthropicProvider)
//
//	result, err := orchestrator.Execute(ctx, "Write a Python function to sort a list", messages)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.Content)
//
// Esempio di utilizzo con chain:
//
//	chain := agents.NewChainBuilder(agents.ChainTypeDraftRefine, orchestrator).
//	    WithStep("draft", draftAgent, "Create a draft: %s").
//	    WithStep("refine", refineAgent, "Refine this draft").
//	    Build()
//
//	result, err := chain.Execute(ctx, "Write an article about AI", messages)
//
package agents
