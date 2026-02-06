# GUIDA COMPLETA: FLUSSO OPERATIVO AUTONOMO PER VIBE CODING
## Sistemi che funzionano senza intervento umano e non allucinano

**Analisi basata su:** Addy Osmani (Claude Code Swarms + Agentic Engineering), Birgitta Boeckeler/Martin Fowler (Context Engineering), Cursor Research (Self-Driving Codebases), Mitchell Hashimoto (AI Adoption Journey), Andrew Codesmith (900+ ore di Claude Code/Cursor)

---

## PREMESSA FONDAMENTALE: VIBE CODING vs AGENTIC ENGINEERING

Il "vibe coding" puro (prompta, accetta tutto, non leggere i diff) funziona SOLO per prototipi e demo. Per sistemi autonomi e affidabili serve quello che Andrej Karpathy e Addy Osmani chiamano "Agentic Engineering": **l'AI fa l'implementazione, l'umano possiede architettura, qualità e correttezza.**

Il paradosso chiave: l'AI-assisted development premia le buone pratiche ingegneristiche PIÙ del coding tradizionale:
- Migliori sono le spec, migliore l'output
- Più completi i test, più puoi delegare con sicurezza
- Più pulita l'architettura, meno l'AI allucina

---

## SEZIONE 1: CONTEXT ENGINEERING - IL FONDAMENTO ANTI-ALLUCINAZIONE

La context engineering è il singolo fattore più importante per prevenire le allucinazioni. Come spiega Birgitta Boeckeler (Thoughtworks/Martin Fowler): *"Context engineering is curating what the model sees so that you get a better result."*

### PRINCIPIO 1: Costruisci il contesto in modo incrementale, non massivo
NON copiare setup da estranei su internet. Costruisci le tue regole gradualmente basandoti sui problemi REALI che incontri. I modelli sono potenti: quello che serviva 6 mesi fa potrebbe non servire più.

### PRINCIPIO 2: Mantieni il contesto snello e focalizzato
Anche se le finestre di contesto sono enormi (1M token con Opus 4), più informazione ci metti, peggio ragiona l'agente. L'efficacia degrada con il contesto espanso. Aggiungere note strategiche a un contesto che deve fixare un bug CSS PEGGIORA le prestazioni.

### PRINCIPIO 3: Gerarchia di contesto strutturata

Usa questa gerarchia per organizzare il contesto nei tuoi progetti:

| Livello | Descrizione | Esempio |
|---------|-------------|---------|
| **CLAUDE.md / AGENTS.md** | Convenzioni generali SEMPRE caricate. Solo le regole più ripetute e universali | "usiamo yarn, non npm", "attiva il virtual environment prima di tutto" |
| **Rules con scope per file** | Regole che si caricano solo quando rilevanti | regole per *.sh, regole per *.ts |
| **Skills (lazy-loaded)** | Documentazione, istruzioni, script che l'LLM carica ON-DEMAND | come accedere a JIRA, convenzioni React, come integrare una API specifica |
| **Subagents** | Task complessi che meritano il proprio contesto separato | code review con modello diverso per "seconda opinione" |
| **MCP Servers** | Accesso strutturato a API e tool esterni | - |
| **Hooks** | Script deterministici su eventi del ciclo di vita | dopo ogni edit di file JS, esegui prettier automaticamente |

### PRINCIPIO 4: Chi decide di caricare il contesto conta enormemente

Tre possibilità:
- **L'LLM decide** → più automazione, meno controllo
- **L'umano decide** → più controllo, meno automazione  
- **Il software decide** → deterministico, come gli hooks

Per sistemi autonomi, punta su una combinazione di **Skills (LLM decide) + Hooks (deterministici)**.

---

## SEZIONE 2: HARNESS ENGINEERING - IL SISTEMA CHE PREVIENE GLI ERRORI

Mitchell Hashimoto (creatore di Terraform, Vagrant, Ghostty) chiama questo approccio "harness engineering": ogni volta che l'agente fa un errore, ingegnerizza una soluzione perché non lo faccia MAI PIÙ.

### TIP 1: Dai all'agente strumenti per auto-verificarsi
Se l'agente può verificare il proprio lavoro, nella maggioranza dei casi corregge i propri errori e previene regressioni. Questa è LA singola differenza più grande tra vibe coding e agentic engineering.
- **Senza test**: l'agente dichiara "fatto" su codice rotto
- **Con test**: itera in loop finché tutto passa

### TIP 2: Aggiorna AGENTS.md ogni volta che l'agente sbaglia
Ogni riga del tuo AGENTS.md deve nascere da un comportamento errato osservato. Non pre-popolare con regole teoriche. Documenta errori reali e le soluzioni. Hashimoto conferma che questo risolve quasi completamente i comportamenti problematici.

### TIP 3: Crea tool programmatici di supporto
Script per: catturare screenshot, eseguire test filtrati, validare output, controllare stili. Associa ogni tool a un'entry in AGENTS.md che ne spiega l'esistenza all'agente.

### TIP 4: Usa vincoli invece di istruzioni
Scoperta confermata sia da Cursor Research che da Hashimoto:
- ❌ "ricordati di completare le implementazioni"
- ✅ "No TODOs, no partial implementations"

I modelli fanno cose buone per default. I vincoli definiscono i confini. Le istruzioni positive vengono spesso ignorate; i vincoli negativi vengono rispettati.

### TIP 5: Sii specifico con le quantità
- ❌ "genera molti task" → produce pochi risultati
- ✅ "genera 20-100 task" → comunica ambizione e produce comportamento completamente diverso

Usa numeri e range concreti.

---

## SEZIONE 3: ARCHITETTURA MULTI-AGENTE PER SISTEMI AUTONOMI

Claude Code Agent Teams (Swarms) e il Cursor Self-Driving Codebases convergono sullo stesso pattern architetturale validato:

### PATTERN: Planner + Executor + Workers specializzati

La ricerca di Cursor ha testato MOLTI approcci prima di convergere:

1. **Self-coordination (agenti alla pari)** → FALLIMENTO. Lock contention, confusione, throughput di 20 agenti ridotto a 1-3.

2. **Singolo executor continuo** → FALLIMENTO. Troppi ruoli simultanei causano comportamenti patologici (sleep random, stop agenti, lavoro in proprio invece di delegare).

3. **Planner gerarchico + Workers indipendenti** → SUCCESSO. Il design finale che ha prodotto ~1000 commit/ora per una settimana senza intervento umano.

### IL DESIGN CHE FUNZIONA:

```
                    ┌─────────────┐
                    │ Root Planner│ (possiede l'intero scope, NON fa coding)
                    └──────┬──────┘
                           │
            ┌──────────────┼──────────────┐
            ▼              ▼              ▼
    ┌───────────┐  ┌───────────┐  ┌───────────┐
    │Sub-Planner│  │Sub-Planner│  │  Worker   │
    └─────┬─────┘  └─────┬─────┘  └───────────┘
          │              │
     ┌────┴────┐    ┌────┴────┐
     ▼         ▼    ▼         ▼
 ┌───────┐ ┌───────┐ ┌───────┐ ┌───────┐
 │Worker │ │Worker │ │Worker │ │Worker │
 └───────┘ └───────┘ └───────┘ └───────┘
```

- **Root Planner**: possiede l'intero scope. Produce task specifici e targetizzati. NON fa coding. NON sa chi raccoglie i task.
- **Sub-Planners**: quando lo scope si può suddividere, il planner spawna sub-planner che possiedono completamente la loro fetta. Ricorsivo.
- **Workers**: raccolgono task e li portano a completamento. NON comunicano con altri planner o worker. Lavorano sulla propria copia del repo.

Quando finiscono, scrivono un **handoff** che include:
- Cosa è stato fatto
- Note importanti
- Preoccupazioni
- Deviazioni
- Scoperte
- Feedback

Il Planner riceve gli handoff come messaggi follow-up, mantiene la vista globale, e continua a pianificare.

### PERCHÉ FUNZIONA SENZA INTERVENTO:
- Nessuna sincronizzazione globale necessaria
- Le informazioni si propagano verso l'alto attraverso gli handoff
- I planner con vista progressivamente globale prendono decisioni senza overhead di cross-talk tra worker
- Il sistema è anti-fragile: il fallimento di un singolo agente non blocca gli altri

---

## SEZIONE 4: COME USARE CLAUDE CODE AGENT TEAMS IN PRATICA

### SETUP
Aggiungi al settings.json:
```json
{
  "env": {
    "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"
  }
}
```

### CASI D'USO VALIDATI DOVE FUNZIONA MEGLIO:

1. **Debug con ipotesi concorrenti**: spawna 5 teammate, ognuno investiga una teoria diversa. Meglio dell'investigazione sequenziale perché evita l'anchoring bias. Gli investigatori in dibattito convergono sulla root cause più velocemente.

2. **Code review multi-prospettiva**: un teammate sulla sicurezza, uno sulle performance, uno sulla copertura test. Un singolo reviewer gravita verso un tipo di issue alla volta. Specialisti paralleli danno attenzione completa a ogni dominio.

3. **Feature cross-layer**: frontend, backend, test ognuno posseduto da un teammate diverso. Parallelismo con focus completo su ogni dominio.

4. **Ricerca ed esplorazione**: teammate multipli investigano approcci diversi, condividono scoperte, convergono sul migliore.

### REGOLE OPERATIVE CRITICHE:

- **Ownership dei file**: DUE teammate che editano lo STESSO file causano sovrascritture. Dividi il lavoro per set di file distinti.

- **Task sizing**: 5-6 task per teammate. Troppo piccoli e il coordinamento domina. Troppo grandi e lavorano troppo senza check-in.

- **Il Lead NON deve implementare**: usa Shift+Tab (delegate mode) per forzare il lead a fare solo coordinamento. Altrimenti si distrae e implementa da solo invece di delegare.

- **Contesto specifico per ogni teammate**: i teammate NON ereditano la storia della conversazione del lead. Includi dettagli specifici nel prompt di spawn.

Esempio:
> "Revisiona il modulo auth in src/auth/ per vulnerabilità di sicurezza. Focus su token handling, session management, input validation. L'app usa JWT in httpOnly cookies."

---

## SEZIONE 5: IL WORKFLOW QUOTIDIANO AUTONOMO (da Hashimoto, validato)

### FASE 1 - Fine giornata (ultimi 30 min)
Avvia agenti per: deep research, esplorazioni parallele di idee vaghe, triage di issue/PR con GitHub CLI. NON lasciare che gli agenti rispondano pubblicamente. Genera solo report per il giorno dopo.

### FASE 2 - Inizio giornata
Prendi i risultati del triage notturno. Filtra manualmente per trovare issue che l'agente risolverà quasi certamente bene. Avvia agenti in background (uno alla volta se non usi multi-agent).

### FASE 3 - Deep work parallelo
**DISATTIVA le notifiche desktop dell'agente.** Il context switching è costosissimo. TU decidi quando controllare l'agente, non il contrario. Lavora in deep focus su task che ami/richiedono pensiero umano. Nei break naturali, controlla l'agente.

### FASE 4 - Review e integrazione
Rivedi l'output dell'agente con lo stesso rigore di una PR di un collega. Se non puoi spiegare cosa fa un modulo, non va in produzione.

### OBIETTIVO
Avere SEMPRE un agente in esecuzione. Se non ce n'è uno, chiediti *"c'è qualcosa che un agente potrebbe fare per me ora?"*

---

## SEZIONE 6: PREVENIRE LE ALLUCINAZIONI - TECNICHE VALIDATE E INCROCIATE

Le seguenti tecniche sono confermate da MULTIPLE fonti indipendenti:

### 1. TEST COME GUARDRAIL PRIMARIO (Osmani + Hashimoto + Cursor)
I test trasformano un agente inaffidabile in un sistema affidabile. L'agente itera finché i test passano. Senza test, dichiara successo su codice rotto. **Scrivi test PRIMA di delegare all'agente.**

### 2. PLAN MODE PRIMA DELL'ESECUZIONE (Osmani + Codesmith + Hashimoto)
Separa sempre planning ed execution. Sessioni di planning producono spec/design doc. Sessioni di execution seguono le spec. Per task rischiosi, richiedi approvazione del piano prima dell'implementazione.

### 3. CONTESTO MINIMO E FOCALIZZATO (Boeckeler + Osmani)
Meno contesto = ragionamento migliore. Non dumpare tutto nella finestra di contesto. Usa lazy-loading (Skills) per caricare informazioni solo quando necessario.

### 4. VINCOLI > ISTRUZIONI (Cursor Research)
Definisci cosa l'agente NON deve fare piuttosto che cosa deve fare. I modelli fanno cose buone per default; i vincoli prevengono deviazioni.

### 5. SCRATCHPAD FRESCO, NON APPENDERE (Cursor Research)
Per agenti long-running, il scratchpad deve essere RISCRITTO frequentemente, non appenduto. Agenti che appendono accumulano contesto stale che causa drift e allucinazioni.

### 6. SELF-REFLECTION PERIODICA (Cursor Research)
Inserisci reminders nel system prompt per incoraggiare l'agente a sfidare le proprie assunzioni e pivotare quando necessario. Gli agenti tendono ad andare troppo in profondità su percorsi sbagliati senza stimoli esterni.

### 7. HANDOFF STRUTTURATI (Cursor Research)
Quando un worker finisce, deve scrivere non solo cosa ha fatto, ma anche: preoccupazioni, deviazioni, scoperte inaspettate, feedback. Questo propaga informazioni critiche verso l'alto.

### 8. ACCETTARE UN TASSO DI ERRORE PICCOLO MA STABILE (Cursor Research)
Richiedere 100% correttezza ad ogni commit causa serializzazione e crolli di throughput. Un tasso di errore piccolo e stabile con un branch "green" per il reconciliation finale è più produttivo e paradossalmente più affidabile a livello di sistema.

### 9. NON ISTRUIRE CIÒ CHE IL MODELLO SA GIÀ (Cursor + Hashimoto)
Tratta l'agente come un brillante neo-assunto che conosce l'engineering ma non il TUO specifico codebase. Istruisci solo per ciò che è specifico al tuo dominio (come eseguire i test, la pipeline di deploy, le API interne).

### 10. MULTI-PASS VERIFICATION (Community/Codesmith)
Non accettare mai la prima strategia o output. Usa plan mode, verifica, poi esegui. Rigenera con approccio diverso se necessario.

---

## SEZIONE 7: STRUMENTI E STACK RACCOMANDATO

### TIER 1 - Coding Agents
- Claude Code con Agent Teams (per orchestrazione multi-agente)
- Cursor con Background Agents (per agenti long-running)
- Usa modelli diversi per task diversi (Opus per ragionamento, modelli più veloci per task meccanici)

### TIER 2 - Context Engineering
- CLAUDE.md / AGENTS.md per convenzioni globali
- Skills per documentazione lazy-loaded
- Hooks per automazione deterministica
- MCP Servers per accesso a API/tool esterni

### TIER 3 - Quality Assurance Automatica
- Test suite completa (unit + integration + e2e)
- CI/CD pipeline che gli agenti possono invocare
- Linting e formatting automatici via hooks
- Screenshot comparison per UI

### TIER 4 - Workflow Enhancement
- Compound Engineering Plugin (plan→work→review→compound cycle) che documenta gli apprendimenti per agenti futuri
- GitHub CLI per triage automatizzato
- Scratchpad con riscrittura periodica

---

## SEZIONE 8: ERRORI COMUNI DA EVITARE ASSOLUTAMENTE

| Errore | Problema | Soluzione |
|--------|----------|-----------|
| "Build me an app" come prompt | Troppo vago. Brucia token mentre gli agenti floundering | Task chiaramente definiti con deliverable specifici |
| Lasciare che il Lead implementi | Pattern comune nelle agent teams | Usa delegate mode (Shift+Tab) |
| Due agenti sullo stesso file | Causa sovrascritture | Assegna ownership di file chiara |
| Contesto troppo grande all'inizio | Le regole copiate spesso confondono | Costruisci incrementalmente |
| Nessun meccanismo di verifica | L'agente non sa quando sbaglia | Test/linting/CI obbligatori |
| Aspettarsi perfezione deterministica | LLM = probabilità, non certezze | Context engineering AUMENTA la probabilità, non garantisce |
| Context switching per controllare l'agente | Il deep work umano è il valore principale | Disattiva notifiche, controlla nei break |

---

## SEZIONE 9: RICETTA RAPIDA - SETUP DA ZERO PER SISTEMA AUTONOMO

| Giorno | Azione |
|--------|--------|
| **1** | Crea CLAUDE.md con 5-10 regole essenziali basate sul progetto |
| **2-3** | Lavora normalmente con l'agente. Ogni errore → aggiungi regola |
| **7** | Crea 2-3 Skills per la documentazione di dominio più consultata |
| **14** | Implementa hooks per formatting e linting automatico |
| **21** | Sperimenta con subagent per code review con modello diverso |
| **30** | Attiva agent teams per feature cross-layer |

**Ogni iterazione**: documenta gli apprendimenti nel tuo sistema di contesto.

**L'80% è planning e review, il 20% è esecuzione.**

---

## FONTI ANALIZZATE E CROSS-VERIFICATE

1. Addy Osmani - "Claude Code Swarms" (Feb 5, 2026) - addyosmani.com/blog/claude-code-agent-teams/
2. Addy Osmani - "Agentic Engineering" (Feb 4, 2026) - addyosmani.com/blog/agentic-engineering/
3. Birgitta Boeckeler / Martin Fowler - "Context Engineering for Coding Agents" (Feb 5, 2026) - martinfowler.com
4. Cursor Research - "Towards Self-Driving Codebases" (Feb 5, 2026) - cursor.com/blog/self-driving-codebases
5. Mitchell Hashimoto - "My AI Adoption Journey" (Feb 5, 2026) - mitchellh.com/writing/my-ai-adoption-journey
6. Andrew Codesmith - "900+ hours of Learning Claude Code/Cursor" (Jan 26, 2026) - YouTube
7. Claude Code Docs - "Orchestrate teams of Claude Code sessions" - code.claude.com/docs/en/agent-teams

---

## NOTE FINALI

Questo documento sintetizza pattern CONVERGENTI da fonti indipendenti. Le tecniche elencate sono state verificate incrociando almeno 2-3 fonti.

**Il principio guida è**: l'autonomia dell'agente cresce proporzionalmente alla qualità dell'infrastruttura di verifica che gli costruisci attorno.

- Un agente senza test è un generatore di allucinazioni.
- Un agente con test, contesto focalizzato, vincoli chiari e ownership di file definita è un sistema produttivo che funziona mentre dormi.
