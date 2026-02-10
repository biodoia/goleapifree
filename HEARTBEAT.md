# HEARTBEAT.md - 24/7 Autonomous Work

## 🔄 Active Task Queue

### Priority 1: cligolist cleanup + enrichment ✅ DONE
- [x] Removed 27 dead/404 projects
- [x] Enriched major tools (open-interpreter, gpt-engineer, etc.)
- [x] Final count: 63 quality cards
- Quality over quantity! 🎯

### Priority 2: Autoschei ecosystem ✅ DONE
- [x] Build test: 9/9 modules pass ✅ (was 5/8)
- [x] Update GitHub repo descriptions (autoschei, gommander, cligolist, framegotui)
- [x] Fixed gommander: extracted shared types to types.go, fixed package declarations
- [x] Fixed framegotui: added pq dependency, fixed unused var, moved broken example
- [x] Sync empty submodules ✅ (all 27 submodules initialized)

### Priority 3: GOLEM development 🔄 IN PROGRESS
- [x] 6 agents implemented (Architect, Coder, Reviewer, Debugger, Tester, Docs)
- [x] Z.AI model integration (GLM-4-32B, GLM-Z1-32B)
- [x] Agent specialization by model capability
- [x] Demo tape created
- 🔄 Phase 2: Z.AI provider (streaming, function calling)
- 🔄 Phase 3: Claude parity (file ops, shell, session management)
  - [x] Session persistence (save/load, :s/:n/:l commands)
- 🔄 Phase 4: MCP enterprise (full integration)
- 🔄 Phase 5: FrameGoTUI TUI (enhancements)
- 🔄 Phase 6: Enterprise quality (polishing)
- Latest: `b792b02` - 💾 Session persistence

### Priority 4: goclit-ai animation ✅ DONE
- [x] Copilot animation technique analyzed
- [x] VHS installed (v0.10.0)
- [x] Frames extracted to /tmp/copilot-frames/
- [x] Copilot-quality multicolor ASCII animation (10-color spectrum)
- [x] Fly-in effect with spring physics (omega=8, zeta=0.6)
- [x] 35+ effective frames (10 eye expressions × 5 phases)
- [x] Commit: `22a39bd`

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

### Priority 8: Auto-Claude-Go ✅ DONE (2026-02-10)
- [x] Research: AndyMik90/Auto-Claude (11,706 stars)
- [x] Create Go TUI implementation structure
- [x] Add core models (task, memory, roadmap, ideation, terminal, chat, changelog)
- [x] Commit: `12e19f7` - 371 lines of models
- [x] Kanban Board - 4-column task management (TODO/PROGRESS/REVIEW/DONE)
- [x] Agent Terminals - 6 agent grid with FREE model rotation
- [x] Memory Layer - Searchable memory with relevance scores
- [x] Insights Chat - Codebase Q&A interface
- [x] Ideation - Ideas & issues board with voting/approval
- [x] Roadmap - Quarterly feature planning (Q1-Q4 2026)
- [x] Changelog - Version history with semantic changes
- [x] Commit: `6d9fbf6` - 1,455 lines of TUI code
- Location: `/home/lisergico25/repos/auto-claude-go`

### Priority 9: Extended Free + Ultra-Cheap Models ✅ DONE (UPDATED)
- [x] Sergio's feedback: "Free models su opencode, ci sono glm 4.7, minimax 2.1, big pickle etc etc"
- [x] **17 FREE models** (all $0.00):
  - Z.AI: glm-4.5-air:free
  - TNG: tng-r1t-chimera:free, deepseek-r1t2-chimera:free
  - NVIDIA: nemotron-30b:free, nemotron-12b-vl:free
  - OpenAI: gpt-oss-120b:free, gpt-oss-20b:free
  - Qwen: qwen3-4b:free, qwen3-coder:free
  - Mistral: mistral-24b:free, dolphin-mistral-24b:free
  - Google: gemma-3-27b:free, gemma-3-12b:free, gemma-3-4b:free
  - Meta: llama-70b:free, llama-3b:free
- [x] **6 ULTRA-CHEAP models** (<$0.000001/1M):
  - z-ai/glm-4.7-flash: **$0.00000006/1M** 🔥 CHEAPEST!
  - z-ai/glm-4-32b: $0.0000001/1M
  - minimax/minimax-01: $0.0000002/1M
  - minimax/minimax-m2: $0.000000255/1M
  - minimax/minimax-m2.1: $0.00000027/1M
  - minimax/minimax-m2-her: $0.0000003/1M
- [x] [R] Rotate all models, [U] Ultra-cheap mode
- [x] Extended info panel with pricing
- [x] Total: **23 models** available
- [x] Commit: `5b22ae8`

### Priority 10: Systemd Service Manager ✅ DONE (NEW)
- [x] Systemd service management panel in FrameGoTUI
- [x] View all backend services status (active/inactive/failed)
- [x] Start/Stop/Restart services
- [x] Enable/Disable auto-start
- [x] View logs with journalctl
- [x] Filter by status
- [x] Sergio: "ogni backend server è un service systemd 24/7"
- [x] Commit: `2551a69`
- Location: `cmd/framegotui/dashboard/systemd_manager.go`

## 📊 Progress Tracking
Check `memory/2026-02-10.md` for today's progress.

## 🎯 Current Focus
**GOLEM development** 🔄 IN PROGRESS
- Phase 2: Z.AI provider (streaming, function calling)
- Phase 3: Claude parity (file ops, shell, session management)
- Phase 4: MCP enterprise (full integration)
- Taskfile: `/home/lisergico25/repos/golem/Taskfile.yml`

## ⏰ Work Pattern
- Each heartbeat: pick next task, execute, commit
- Log progress to daily memory file
- Never idle - always working on something
