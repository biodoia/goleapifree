# HEARTBEAT.md - 24/7 Autonomous Work

## 🔄 Active Task Queue

### Priority 1: cligolist cleanup + enrichment ✅ DONE
- [x] Removed 27 dead/404 projects
- [x] Enriched major tools (open-interpreter, gpt-engineer, etc.)
- [x] Final count: 63 quality cards
- Quality over quantity! 🎯

### Priority 2: Autoschei ecosystem ✅ PARTIAL
- [x] Build test: 5/8 modules pass
- [x] Update GitHub repo descriptions (autoschei, gommander, cligolist, framegotui)
- [ ] Sync empty submodules (later)

### Priority 3: GOLEM development ✅ DONE
- [x] 6 agents implemented (Architect, Coder, Reviewer, Debugger, Tester, Docs)
- [x] Z.AI model integration (GLM-4-32B, GLM-Z1-32B)
- [x] Agent specialization by model capability
- [x] Demo tape created

### Priority 4: goclit-ai animation 🔄 IN PROGRESS
- [x] Copilot animation technique analyzed
- [x] VHS installed (v0.10.0)
- [x] Frames extracted to /tmp/copilot-frames/
- [ ] Copilot-quality multicolor ASCII animation
- [ ] Fly-in effect implementation
- [ ] More frames (>27)

### Priority 5: FrameGoTUI Dashboard ✅ DONE (2026-02-10)
- [x] 9 iterations total
- [x] Iter 5: 8 cards at center
- [x] Iter 6: Adaptive layout (Sergio confirmed: "questo è lo stesse che voglio")
- [x] Iter 7: Status colors + progress bars
- [x] Iter 8: Harmonica smooth animations
- [x] Iter 9: Mouse support
- [x] Commits: `c869eb9`, `f7b50a7`, `9e6904d`, `9f93e85`, `3efab6e`
- Location: `cmd/framegotui/dashboard/`

### Priority 6: Harmonica Animations ✅ DONE
- [x] Cursor spring: FPS(60), ω=8.0, ζ=0.5
- [x] Progress springs: FPS(60), ω=6.0, ζ=0.3
- [x] Smooth interpolation between values

### Priority 7: Mouse Support ✅ DONE
- [x] tea.WithMouseAllEvents() - All mouse events
- [x] tea.MouseLeft - Click to select module
- [x] tea.MouseMotion - Hover effect (highlight)
- [x] Mouse position in footer: [🖱️ X,Y]
- [x] Sergio: "supporto del mouse"

### Priority 8: Auto-Claude-Go ✅ DONE
- [x] Research: AndyMik90/Auto-Claude (11,706 stars)
- [x] Create Go TUI implementation with bubbletea
- [x] Port all 6 features to TUI:
  - ✅ Kanban Board
  - ✅ Agent Terminals
  - ✅ Memory Layer
  - 🔄 Insights Chat
  - 🔄 Ideation
  - 🔄 Roadmap
- [x] Apply FrameGoTUI dashboard style
- [x] Commit: `23a2a69`
- Location: `/home/lisergico25/repos/auto-claude-go`

### Priority 9: Multi-Instance FREE Models ✅ DONE (NEW)
- [x] Integrated 6 free models (all $0.00):
  - Qwen3 4B (Reasoning, 40K context)
  - DeepSeek R1T (Reasoning, 163K context)
  - Mistral 24B (Multimodal, 128K context)
  - Gemma 3 27B (General, 131K context)
  - Llama 70B (General, 128K context)
  - Llama 3B (General, 131K context)
- [x] Rotate models with [R] key
- [x] Each agent assigned a free model
- [x] Model info panel showing provider, context, cost
- [x] Sergio: "lancia piu istanze con modelli free di opencode per non cappare"
- [x] Commit: `b5a6815`

## 📊 Progress Tracking
Check `memory/2026-02-10.md` for today's progress.

## 🎯 Current Focus
FrameGoTUI Dashboard ✅ 9 iterations complete
Auto-Claude-Go ✅ Multi-instance free models (6 models, $0.00)
Next: Complete remaining Auto-Claude features (Insights, Ideation, Roadmap)

## ⏰ Work Pattern
- Each heartbeat: pick next task, execute, commit
- Log progress to daily memory file
- Never idle - always working on something
