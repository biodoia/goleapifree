# MEMORY.md - Long-Term Memory

> Curated wisdom from daily sessions
>
> **ARCHITECTURAL RULE (2026-02-11)**: Tutti i repo Biodoia devono appoggiarsi ai database su memogo.

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

### Ralph Loop Pattern (2026-02-11)
**Canonical architecture** for continuous autonomous coding:
```
PRD → Priority story → Implement → Test → Commit → Update → Repeat
```
**Key implementations**:
- frankbria/ralph-claude-code: v0.11.4, 484 tests, 100% pass rate
- snarktank/ralph: Agent-agnostic
- vercel/ralph-loop-agent: TypeScript + AI SDK

**Critical insight**: Each iteration starts with fresh context while progress persists through files and git history.

### Agent Comparison (2026-02-11)
| Agent | Stars | Loop | Multi-Agent | Best For |
|-------|-------|------|-------------|----------|
| **OpenHands** | 67.7K | Native | ✅ 1000s | Enterprise multi-agent |
| **Cline** | 51K | Auto-approve | Single | VS Code integration |
| **Aider** | 40.4K | Wrapper | Single | Interactive pair |
| **SWE-agent** | 10.5K | Native | Parallel | SWE-bench |
| **Claude Code** | 10K+ | YOLO | ✅ Subagents | Terminal |

### OpenHands Deep Dive (2026-02-11)
**Architecture**:
- Core Engine: Osserva → Pianifica → Agisci
- Event Stream: Risolve "identity amnesia" (+20-30% performance)
- Sandboxing: Docker-based per sicurezza enterprise
- Subagents: Delegation pattern (analysis + fixing agents)

**Benchmark SWE-Bench**:
| Configuration | Verified | Lite |
|---------------|----------|------|
| CodeAct 2.1 + Claude 3.5 | **53%** | 41.7% |
| CodeAct 2.1 + Qwen3-235B | 34.4% | - |
| OpenHands LM (32B) | 37.4% | - |
| Real-world claim | 87% issues same-day | - |

**Limiti**: GPU-heavy (12GB+ VRAM), dipende da LLM sottostante

**Proposta**: Integrare Event Stream pattern in GOLEM, testare con FrameGoTUI dashboard

### Go TUI Recommendation (2026-02-11)
**Bubbletea + Lipgloss + Bubbles** (proved by OpenCode 95K stars, Crush 19.7K stars):
- `viewport`: Log streaming
- `progress`: Spring physics animation
- `filepicker`: File browsing
- `list`: Fuzzy task filtering

**tview alternative**: Built-in Grid + Flex layout, good for traditional widgets.
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

### OpenHands SDK Insights (2026-02-10)
Python SDK con skill system simile al nostro + Remote Agent Server:

**Skill Types** (3 livelli):
- `repo_skills`: AGENTS.md style, sempre attivi
- `knowledge_skills`: knowledge/ subdirs, progressive disclosure
- `agent_skills`: SKILL.md files, triggered or on-demand

**Keyword Triggers** (estensione OpenHands):
```python
Skill(
    name="encryption-helper",
    content="Use encrypt.sh",
    trigger=KeywordTrigger(keywords=["encrypt", "decrypt"]),
)
```

**Remote Agent Architecture**:
```
Client (SDK) → Agent Server (HTTP/WS) → Workspace (Docker/VM/Local)
```

**Per GOLEM**: Implementare pattern simile in Go:
- GOLEMWorkspace interface
- DockerRemoteWorkspace, LocalWorkspace
- WebSocket event streaming

**Public Skills Registry**: OpenHands usa `github.com/OpenHands/skills` - potremmo fare `github.com/biodoia/autoschei-skills`

Source: `gofainder/notebooklm-dumps/2026-02-10-openhands-analysis.md`

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

### EDDOps (Evaluation-Driven Dev & Ops) - 2026-02-11
Evaluation come funzione di governo permanente:
- **Offline (dev)** ↔ **Online (runtime)** feedback loop
- **Metriche**: task success, context usage, hallucination rate, human override
- **Process model** con reference architecture
- **Red-teaming integrato** come parte del processo

### Superposition Prompting (Apple) - 2026-02-11
RAG acceleration technique:
- **Speedup**: 93x su query RAG
- **Accuracy boost**: 43% su NaturalQuestions
- **Perché funziona**: Riduce interference tra retrieved chunks

### FullStack Bench (Bytedance) - 2026-02-11
Benchmark multi-linguaggio:
- **Coverage**: 16+ linguaggi
- **Focus**: Cross-language code generation
- **Reference**: Per validare agenti su diversi stack

### Megagon Labs Blueprint - 2026-02-11
Stream-based orchestration per agenti:
- **Componenti**: Planner + Orchestrator + Data streams
- **Ottimizzazione**: Task scheduling intelligente
- **Reference architecture**: Per offline/online evaluation loops

### Trust-Tier Framework (2026-02-11)
Staged autonomy per production adoption:
1. **Advisory**: AI gives non-blocking signals
2. **Recommender**: AI suggests, human approves
3. **Autonomous**: AI acts with human override paths

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
- **Orchestration**: Supreme Orchestrator (Premium), Task Router (Cheap), Priority Manager (Cheap)
- **Sentinel** (sistema immunitario): Code Archaeologist (Free), Pattern Architect, Debt Sentinel, Security Auditor (Cheap)
- **Coding**: Feature Architect, Implementation Coder, Refactor Oracle (Premium), Test Engineer (Cheap)
- **Infrastructure**: Git Manager, Monitor Daemon (Free), CI/CD Controller, Deploy Agent (Cheap)

📖 Full doc: `gofainder/notebooklm-dumps/2026-02-10-bibbia-coding-agentico.md`

### Sentinel Protocol (5 Quality Gates)
Validazione automatica con blocco su:
- Debt Score > 50 → merge bloccato
- Complessità ciclotomatica > 15
- Pattern compliance < 80%
- Test coverage delta negativo
- Efferent coupling > 20

### Frizione Automatizzata (2026-02-10)
L'AI rimuove l'attrito umano → debito tecnico invisibile ("Ghost in the Machine")
Soluzione: incorporare l'attrito nel design via **sistema immunitario agentico**
- Sistema "Inherited" (ereditato) → male, AI amplifica debito
- Sistema "Owned" (posseduto) → bene, vincoli qualità embedded

### 10 Comandamenti del Coding Autonomo
1. Non codificare mai al buio (contesto obbligatorio)
2. Valida prima del merge (Sentinel non bypassabile)
3. Sii padrone dei tuoi pattern (Pattern Registry sacro)
4. Testa ciò che crei (coverage delta ≥ 0)
5. Rifattorizza, non rattoppare (trasforma, non benda)
6. Documenta ogni decisione (ADR = memoria storica)
7. Separa le preoccupazioni (specializzazione > generalismo)
8. Escala l'incertezza < 80% (umano = ultima difesa)
9. Logga tutto (audit trail per debug swarm)
10. Migliora continuamente (scavenging modelli/provider)

### 4-Tier Hybrid Infrastructure
1. **Paid** (forfettari): Claude, GPT, Gemini - task critici
2. **Free** (API gratuite): Groq, OpenRouter Free - alta frequenza
3. **Local** (GPU): RTX 4060/4070, Ollama - privacy, zero costo
4. **Emergency** (GPT4Free fork): solo fallback

Costo stimato: $0.10-0.15 per workflow completo

### Free Models (OpenRouter, 2026-02-10)
**6 modelli completamente gratuiti:**
| Model | Provider | Type | Context |
|-------|----------|------|---------|
| qwen3-4b:free | Qwen | Reasoning | 40K |
| deepseek-r1t-chimera:free | TNG | Reasoning | 163K |
| mistral-small-3.1-24b-instruct:free | Mistral | Multimodal | 128K |
| gemma-3-27b-it:free | Google | General | 131K |
| llama-3.3-70b-instruct:free | Meta | General | 128K |
| llama-3.2-3b-instruct:free | Meta | General | 131K |

**Vantaggi:**
- Zero costo per开发和测试
- Alta frequenza di chiamate
- Multi-instance per parallelismo
- OpenRouter aggregation

### Bifrost vs LiteLLM
| Metrica | LiteLLM | Bifrost (maximhq) |
|---------|---------|-------------------|
| Latency | 500µs | 11µs |
| RAM | 372MB | 120MB |
| Providers | 100+ | 15+ |
| MCP | ❌ | ✅ |
| Semantic Cache | ❌ | ✅ |
| Web UI | ❌ | ✅ |

**Bifrost (maximhq)**: https://github.com/maximhq/bifrost - Gateway completo con MCP, semantic caching, enterprise features

**bifrost-free**: Plugin per maximhq/bifrost che aggiunge free provider aggregation

### Free LLM APIs (2026-02-10)
**Lista completa**: `gofainder/notebooklm-dumps/2026-02-10-free-llm-apis.md`

| Provider | Limiti | Note |
|----------|--------|------|
| OpenRouter | 20 req/min, 50 req/day | 30+ modelli gratuiti |
| **Groq** | 14,400 req/day (Llama 3.1 8B) | ⚡ Migliore velocità |
| Google AI Studio | 250K tokens/min | Gemini models |
| Cloudflare Workers | 10K neurons/day | Edge inference |
| Cerebras | 30 req/min | Large models |
| NVIDIA NIM | 40 req/min | Enterprise |
| HuggingFace | $0.10/mese | Qualsiasi modello open |

**Trial credits**: Fireworks $1, Baseten $30, Nebius $1, Novita $0.5/anno

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

### Multi-Agent Orchestration (2026-02-11)

**Purpose-built for software development**:
| Framework | Stars | Built-in Roles | Parallel |
|-----------|-------|----------------|----------|
| **MetaGPT** | 57.6K | PM, Architect, Engineer, QA | ✅ MacNet |
| **ChatDev** | 27.1K | CEO, CTO, Programmer, Tester | ✅ |
| **Magentic-One** | Microsoft | Orchestrator, Coder, WebSurfer | ✅ |

**General-purpose frameworks**:
| Framework | Stars | Enterprise | Architecture |
|-----------|-------|------------|--------------|
| **CrewAI** | 42.6K | 60% Fortune 500 | Crews + Flows (YAML) |
| **AutoGen/MAF** | 40-48K | Microsoft | CodeExecutorAgent |
| **LangGraph** | 11.7K | LinkedIn, Uber, Replit | Graph-based |

**Go options**:
- **Google ADK-Go** (6.8K): A2A protocol
- **Eino** (ByteDance): DeepAgent
- **openai-agents-go**: MCP + handoffs

**Bifurcation insight**: MetaGPT provides opinionated roles; CrewAI/LangGraph offer flexible building blocks.

**Critical insight**: "Multi-agent orchestration for code generation is bifurcating: MetaGPT and ChatDev provide opinionated software company simulations; CrewAI and LangGraph offer flexible building blocks."

### CI/CD AI Tools (2026-02-11)
| Tool | Type | Feature |
|------|------|---------|
| CircleCI Chunk | CI native | Fix flaky tests (60% success) |
| GitHub Agentic Workflows | YAML | Multi-agent orchestration |
| **CodeRabbit** | Commercial | Most installed (632K PRs reviewed) |
| **Qodo pr-agent** | Open Source | 10K stars, /review /improve |

**Self-healing patterns**:
- **Elastic**: Renovate → CI fails → Claude fixes → commits → restarts (saved ~20 days)
- **AWS**: CloudWatch → Lambda → Bedrock → PR → human review

### Ralph Loop Pattern (2026-02-11)
**Canonical architecture for continuous autonomous coding**:
```
PRD → Priority story → Implement → Test → Commit → Update → Repeat
```
**Key implementations**:
- frankbria/ralph-claude-code: v0.11.4, 484 tests, 100% pass rate
- snarktank/ralph: Agent-agnostic
- vercel/ralph-loop-agent: TypeScript + AI SDK, verification gates, cost limits

**Critical insight**: Each iteration starts with fresh context while progress persists through files and git history.

### WebTUI Reference Stack (2026-02-11)
| Component | Purpose |
|-----------|---------|
| **xterm.js** | Browser terminal emulation (19.5K stars) |
| **WebTUI CSS** | Terminal aesthetics (2.2K stars) |
| **gotty** | Go + WebSocket PTY streaming (19.4K stars) |
| **Nexterm** | Reference: Node.js + React + Socket.IO + xterm.js |

**Architecture**:
```
CSS Grid Container
├── xterm.js (Agent 1) ── WebSocket ── PTY
├── xterm.js (Agent 2) ── WebSocket ── PTY
└── xterm.js (Agent N) ── WebSocket ── PTY
```

### Full Autonomous Loop (2026-02-11)
| Phase | Tool | Purpose |
|-------|------|---------|
| Analyze | CodeRabbit / Qodo Merge | Automated PR review |
| Generate | Ralph Loop + OpenHands | Autonomous code |
| Review | pr-agent / Graphite | AI-powered review |
| Test | Qodo Cover + CircleCI Chunk | Auto-test + flaky fix |
| Deploy | GitHub Actions | AI-optimized pipelines |
| Monitor | SRE Agent / Snyk | Self-healing |
| Repeat | GitHub Agentic Workflows | Event-triggered cycles |

**Key insight**: "The full autonomous loop remains an assembly of specialized tools... but continuous autonomous programming is running in production today."

### WebTUI Reference Stack (2026-02-11)
| Component | Purpose |
|-----------|---------|
| **xterm.js** | Browser terminal emulation (19.5K stars) |
| **gotty** | Go + WebSocket PTY streaming |
| **WebTUI** | Pure CSS terminal aesthetics |
| **Nexterm** | Reference: Node.js + React + Socket.IO + xterm.js |

**Pattern**: CSS Grid with xterm.js instances, each WebSocket-connected to separate agent PTY.

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
- FrameGoTUI dashboard per management centralizzato

### Systemd Manager (2026-02-10)
Panel in FrameGoTUI per gestione backend:
- View all services: `systemctl status autoschei-*`
- Start/Stop/Restart: `systemctl <action> <service>`
- Enable/Disable: `systemctl enable/disable <service>`
- Logs: `journalctl -u <service> -f`
- Filter: active/failed/inactive status

**Template location:** `framegotui/systemd/`

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

- **Memogo**: `/home/lisergico25/repos/memogo` (Database centrale!)
  - URL Database: `memogo/url_database/`
  - Memory Bank: `memogo/memory-bank/`
  - Knowledge Graph: `memogo/graph/`
- Autoschei: `/home/lisergico25/repos/autoschei`
- FrameGoTUI: `/home/lisergico25/repos/framegotui`
- Daily notes: `/home/lisergico25/clawd/memory/`

---

## 🎯 Sergio's Preferences

- Language: Italian preferred
- Stack: Go, gRPC, ConnectRPC, Charm TUI
- Build: Taskfile.yml (NO Make)
- Style: No "cappelli", be direct
- Autonomy: High - don't ask, do

---

*Last updated: 2026-02-11 (Multi-agent orchestration + WebTUI + Definitive Stack Guide)*
