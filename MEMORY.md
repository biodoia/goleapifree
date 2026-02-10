# MEMORY.md - Long-Term Memory

> Curated wisdom from daily sessions

---

## 🗿 GOLEM Project
- **Repo**: https://github.com/biodoia/golem
- GLM-Powered CLI = Claude Code + OpenCode + Crush + Harmonica + Z.AI native
- Z.AI Models: GLM-4-32B, GLM-Z1-32B, GLM-Z1-Rumination, GLM-Z1-9B, GLM-4.1V-Thinking, CodeGeeX-4
- Agent specialization: Deep thinkers (Z1-32B) for Architect/Reviewer/Debugger, Fast (GLM-4-32B) for Coder/Tester/Docs

## 🎬 Copilot CLI Animation Technique
- 6000+ lines TypeScript, frame-by-frame rendering
- ANSI color roles (different colors per element type)
- 75ms interval (~13fps), Ink renderer
- Fly-in effect, multicolor ASCII mascot
- Reference: `github.blog/engineering/from-pixels-to-characters-the-engineering-behind-github-copilot-clis-animated-ascii-banner/`

---

## 🔑 Key Patterns

### Perpetual Engine (2026-02-10)
Autonomous loop pattern for self-improving systems:
1. **Fresh context per iteration** - Clean slate every cycle
2. **Persistent memory** - Git + Memory Bank + task_state.json
3. **6 phases**: OBSERVE → PLAN → EXECUTE → VALIDATE → PERSIST → LEARN
4. **Empirical validation** over formal proofs
5. **Population-based evolution** with Agent Archive

Sources: Ralph (snarktank), Darwin-Godel (lemoz)

### Memory Bank (5 Files)
```
productContext.md  - What we're building
activeContext.md   - Current focus
systemPatterns.md  - Learned patterns
decisionLog.md     - Why we chose X over Y
progress.md        - Done/doing/next
```

### Tecnica Ralph Wiggum (2026-02-10)
Stateless-but-iterative loop per sviluppo agentico:
```
SELECT → IMPLEMENT → VALIDATE → COMMIT → UPDATE → RESET → LOOP
```
**4 canali memoria persistente:**
- AGENTS.md (semantic) - convenzioni, gotchas
- Git history (code evolution)
- progress.txt (activity log)
- prd.json (task compass)

**Key insight**: Context reset previene rot, AGENTS.md mantiene continuità

### AGENTS.md > Skills (Vercel benchmark)
- AGENTS.md: **100% pass rate** (passive, sempre in prompt)
- Dynamic Skills: 79% pass rate (active retrieval, decisione richiesta)
- Takeaway: Passive context >> active retrieval

### EDDOps (Evaluation-Driven Dev & Ops)
Valutazione = funzione di governo permanente, non checkpoint finale
- Offline (dev) ↔ Online (runtime) feedback loop
- Metriche: task success, context usage, hallucination rate, human override

### Sistole/Diastole Pattern
Ciclo biologico per agenti:
- **Sistole** = esecuzione task, output attivo
- **Diastole** = consolidamento, cleanup, "sogno"
→ Implementato in OpenClaw come Heartbeat

### Subagent-as-a-Tool
Invece di 50 tools all'agente principale:
- Incapsula capabilities in sub-agenti chiamabili
- Mantiene snello lo spazio d'azione
- Ogni sub-agente: 3-5 tools max

### LLM-as-a-Judge
Panel multi-model per validation:
- Agente A genera → Agenti B,C,D valutano
- Mitiga bias del singolo modello
- Voting pesato per decisioni

### DOE Framework (2026)
Nuovo paradigma sviluppo:
```
D - DIRECTIVE     → Obiettivi in NL (markdown PRD)
O - ORCHESTRATION → AI pianifica (MCP, API)
E - EXECUTION     → AI esegue codice
```
Developer = "architetto di intenti", non scrittore di sintassi

### Self-Annealing
Agenti che leggono propri error messages per auto-correggersi.
Loop autonomi per ore/giorni finché task completato.

### Paradosso Produttività 2026
- PR speed +20%, ma incidenti +23.5%
- "AI slop" = volume > capacità review
- Soluzione: verification harness automatici

### Three Laws of Agentic Development (2026-02-10)
1. **Verification > Generation** - Cost(deploying wrong) >> Cost(generating wrong)
2. **Context is Finite, Memory is Infinite** - Externalize to disk
3. **Reset Prevents Rot** - Fresh context + persistent memory = success

### The Comprehension Paradox
- Code volume → ∞
- Human review capacity → Fixed
- Gap = Existential debt
- Solution: Micro-tasks, test-as-spec, AI-assisted review

### Brain-Inspired Agent Architecture
- Neocortex = Generation (LLM)
- Prefrontal Cortex = Planning (DOE orchestrator)
- Hippocampus = Memory (4 channels, vector DB)
- Thalamus = Filtering (input routing, risk eval)
- Wake/Sleep cycles for consolidation

### Meta-Insight: Process > Comprehension
Intelligence emerges from **structured process**, not understanding.
Agents don't understand code, but with right process produce correct code.
Verification ensures correctness despite non-comprehension.

### Bottegaiarda Architecture (2026-02-10)
Sistema autonomo di coding con 14 agenti e 4 layer:
- **Orchestration**: Supreme Orchestrator, Task Router, Priority Manager
- **Sentinel** (sistema immunitario): Code Archaeologist, Pattern Architect, Debt Sentinel, Security Auditor
- **Coding**: Feature Architect, Implementation Coder, Test Engineer, Refactor Oracle
- **Infrastructure**: Git Manager, CI/CD Controller, Deploy Agent, Monitor Daemon

### Sentinel Protocol
Validazione automatica con blocco su:
- Debt Score > 50 → merge bloccato
- Complessità ciclotomatica > 15
- Pattern compliance < 80%
- Test coverage delta negativo

### 4-Tier Hybrid Infrastructure
1. **Paid** (forfettari): Claude, GPT, Gemini - task critici
2. **Free** (API gratuite): Groq, OpenRouter Free - alta frequenza
3. **Local** (GPU): RTX 4060/4070, Ollama - privacy, zero costo
4. **Emergency** (GPT4Free fork): solo fallback

Costo stimato: $0.10-0.15 per workflow completo

### Bifrost vs LiteLLM
| Metrica | LiteLLM | Bifrost |
|---------|---------|---------|
| Latency | 500µs | 11µs |
| RAM | 372MB | 120MB |

### Capability-Based Routing
Virtual endpoints: `fast-go-coder`, `deep-reasoner`, `long-context-analyst`
- 52.8% prompt gestibili da modelli <20B parametri
- ELO ranking dinamico per provider
- Discovery automatica nuovi endpoint

### Adaptive Feature Layout (2026-02-10)
TUI dashboard design pattern:
- **Left panel**: System status (CPU/RAM/DISK/NET), Recent activity, Alerts
- **Center**: 8 module cards, each with 3 feature-specific panels
- **Right panel**: Quick actions, Focus module details, Hotkeys

Module-specific panels:
- GOLEM: Models/Tasks/Success
- GoBro: Consciousness/Agents/Loops
- CodyGody: Files/Commits/Coverage
- Gommander: Commands/Permissions/Uptime
- Tuitty: Tasks/Goals/Progress
- Ideaeater: Pipeline/Sources/Quality
- Progotti: Projects/Milestones/Team
- Memogo: Memory/Context/Learning

Key insight: Arrange panels based on **actual features** of each TUI app, not generic metrics.

---

## 🧠 Autoschei Ecosystem

### Architecture
- **GoBro** = Distributed brain (wrapper of Clawd instances)
- **Ideaeater** = Idea capture ("il lavandino")
- **Progotti** = Project manager (macro/phase/microphase)
- **SpegoPlain** = Spec generator
- **Gommander** = Command executor
- **goaiaiai** = Universal validator
- **gociccidai** = Code generator (LLM)
- **ghrego** = Git operations
- **memogo** = Dual memory (System 1 + System 2)
- **redteam** = Adversarial testing
- **govai** = Deploy manager
- **gomanagos** = OS/ecosystem manager (installer, monitor, maintainer)

### gRPC Ports
- goaiaiai: 50051
- gociccidai: 50052
- ghrego: 50053
- progotti: 50054
- memogo: 50055
- govai: 50056

### Deployment Pattern
**Tutti i moduli = servizi systemd sempre attivi**
- Ogni modulo ha backend daemon (`*-serve` o `*d`)
- Gestiti via systemd (start/stop/restart/status)
- Logs via journalctl
- Auto-restart on failure

### Core Flow
```
ghrego(scan) → gociccidai → goaiaiai → govai(deploy) → ghrego(commit)
```

---

## 💡 Lessons Learned

### From gociccidai build
- Use OpenRouter for free models (gemini-2.0-flash-exp:free)
- Streaming essential for progress feedback
- QA validator catches 80% of issues early

### From redteam implementation
- Target abstraction (HTTP/gRPC/Agent) enables universal attacks
- Auto-detection simplifies UX
- Fuzzer + Attacker + Liar covers main threat vectors

### From parallel sub-agents
- 5 workers simultaneously = massive speedup
- Each sub-agent needs clear scope
- Failure recovery: spawn v2 with feedback

---

## 📍 Important Paths

- Autoschei: `/home/lisergico25/repos/autoschei`
- Engine: `/home/lisergico25/repos/autoschei/engine`
- Memory Bank: `/home/lisergico25/repos/autoschei/memory-bank`
- Binaries: `/home/lisergico25/repos/autoschei/bin`
- Daily notes: `/home/lisergico25/clawd/memory/`

---

## 🎯 Sergio's Preferences

- Language: Italian preferred
- Stack: Go, gRPC, ConnectRPC, Charm TUI
- Build: Taskfile.yml (NO Make)
- Style: No "cappelli", be direct
- Autonomy: High - don't ask, do

---

*Last updated: 2026-02-10*
