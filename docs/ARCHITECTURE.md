# Azula — Architecture

**Version:** 1.0  
**Status:** Design spec for MVP implementation

---

## 1. System Overview

```
┌─────────────────────────────────────────────────────────┐
│                     Web Client                          │
│  (Workspace, Projects, Investigation UI, Analytics)     │
└─────────────────────────┬───────────────────────────────┘
                          │ GraphQL
┌─────────────────────────▼───────────────────────────────┐
│                    GraphQL API                          │
│  ┌──────────┐  ┌──────────────────┐  ┌─────────────┐  │
│  │   Auth   │  │  Investigation   │  │  Analytics  │  │
│  │ Service  │  │    Service       │  │   Service   │  │
│  └──────────┘  └────────┬─────────┘  └─────────────┘  │
└─────────────────────────┼───────────────────────────────┘
                          │
          ┌───────────────┼───────────────┐
          ▼               ▼               ▼
     ┌─────────┐    ┌───────────┐   ┌──────────┐
     │ MongoDB │    │ AI Router │   │   MCP    │
     └─────────┘    └─────┬─────┘   │Connector │
                          │          └────┬─────┘
                    ┌─────┴─────┐         │
                    ▼           ▼         ▼
               Fast Model  Deep Model   Files
                              │
                         Judge Model
```

---

## 2. Service Boundaries

### 2.1 Auth Service

- User registration, login, JWT issuance
- Workspace ownership
- Tier limit checks (delegate to `TierService`)

### 2.2 Investigation Service

- Owns investigation lifecycle (state machine)
- Orchestrates Fast → Deep → Council pipeline
- Persists messages, results, evidence
- **Must not** call LLM providers directly — uses `ModelRouter`

### 2.3 Analytics Service

- Aggregates investigation metrics per workspace
- Computes top root causes, model performance stats
- Read-only; no AI calls

### 2.4 AI Model Router

- Selects model by strategy (Fast / Deep / Judge)
- Abstracts provider (OpenAI, local, custom)
- Returns structured responses

### 2.5 MCP Connector

- Single entry point for agent file access
- MVP: `FilesConnector` only
- Future: `GitConnector`, `DatabaseConnector`

---

## 3. Investigation State Machine

```
                    ┌─────────┐
                    │ pending │
                    └────┬────┘
                         │ startInvestigation()
                         ▼
                 ┌───────────────┐
                 │ fast_classify │
                 └───────┬───────┘
                         ▼
                 ┌───────────────┐
                 │ deep_analyze  │
                 └───────┬───────┘
                         ▼
                    ┌─────────┐
                    │ council │
                    └────┬────┘
                         ▼
                   ┌───────────┐
                   │ completed │
                   └───────────┘

  Any state ──error──▶ failed
```

**State transitions are enforced in `InvestigationService`** — resolvers must not skip states. Deep and Council always run after Fast unless a Pro entitlement gate blocks Deep.

---

## 4. AI Council Pipeline

```
InvestigationService
       │
       ▼
  ModelRouter.runCouncil(context)
       │
       ├──▶ Investigator (Model B — Deep / QLoRA if attached)
       │         └── MCP: ranked files under token budget
       │
       ├──▶ Challenger (different Ollama family when installed, else Model A)
       │         └── Must disagree; not a second copy of Investigator
       │
       └──▶ Judge (Model C OpenAI if OPENAI_API_KEY set, else Model A)
                 └── Narrative agreements / disagreements
                       ▼
                 Go weighted vote + echo-chamber detector
```

**Parallelism:** Investigator and Challenger run in parallel. Judge waits for both. Aggregation is **not** “they said the same thing so it is true”: same-family agreement is flagged `echo_chamber`.

See [PROMPTING.md](PROMPTING.md) for templates and budgets.

---

## 5. Module Abstractions

### 5.1 LLMProvider

```typescript
interface LLMProvider {
  complete(params: CompletionParams): Promise<CompletionResult>;
  stream?(params: CompletionParams): AsyncIterable<string>;
}

// Implementations
class OpenAIProvider implements LLMProvider { ... }
class LocalProvider implements LLMProvider { ... }
class CustomProvider implements LLMProvider { ... }
```

### 5.2 ModelRouter

```typescript
interface ModelRouter {
  classify(context: InvestigationContext): Promise<FastClassification>;
  analyze(context: InvestigationContext): Promise<DeepAnalysis>;
  runCouncil(context: InvestigationContext): Promise<CouncilResult>;
}

class ModelRouterImpl implements ModelRouter {
  constructor(
    private fastStrategy: FastStrategy,
    private deepStrategy: DeepStrategy,
    private councilOrchestrator: CouncilOrchestrator,
  ) {}
}
```

### 5.3 MCPConnector

```typescript
interface MCPConnector {
  listFiles(projectId: string): Promise<ProjectFile[]>;
  readFile(projectId: string, path: string): Promise<string>;
  searchFiles(projectId: string, query: string): Promise<SearchResult[]>;
}

// MVP
class FilesConnector implements MCPConnector { ... }

// MVP+1
class GitConnector implements MCPConnector { ... }
```

### 5.4 Agent Modules

```typescript
interface Agent {
  role: 'investigator' | 'challenger' | 'judge' | 'generator' | 'evaluator';
  run(context: AgentContext): Promise<AgentResult>;
}
```

**Rule:** New agents implement `Agent` interface. No agent calls LLM directly — goes through `ModelRouter`.

---

## 6. MongoDB Data Model

### 6.1 Users

```typescript
{
  _id: ObjectId,
  email: string,
  passwordHash: string,
  tier: 'free' | 'pro' | 'enterprise',
  createdAt: Date,
  updatedAt: Date
}
```

### 6.2 Workspaces

```typescript
{
  _id: ObjectId,
  name: string,
  ownerId: ObjectId,       // ref Users
  createdAt: Date,
  updatedAt: Date
}
```

### 6.3 Projects

```typescript
{
  _id: ObjectId,
  workspaceId: ObjectId,   // ref Workspaces
  name: string,
  isSample: boolean,       // true for onboarding demo
  files: [
    { name: string, path: string, mimeType: string, uploadedAt: Date }
  ],
  createdAt: Date,
  updatedAt: Date
}
```

### 6.4 Investigations

Canonical GraphQL: `Investigation` in [`graph/schema.graphqls`](../graph/schema.graphqls). Mongo document (`internal/domain`):

```typescript
{
  _id: ObjectId,
  projectId: ObjectId,
  workspaceId: ObjectId,
  userId: ObjectId,
  status: 'pending' | 'fast_classify' | 'deep_analyze' | 'council' | 'completed' | 'failed',
  prompt: string,
  plan: PlanStep[],
  filesAccessed: string[],
  fastResult: FastResult | null,
  deepResult: DeepResult | null,
  councilResult: CouncilResult | null,
  errorMessage: string | null,
  modelAName: string | null,   // Fast / Challenger fallback
  modelBName: string | null,   // Deep / Investigator
  modelCName: string | null,   // Judge (OpenAI) when set
  escalationReason: string | null,
  executionMode: 'live' | 'fallback' | 'mixed' | null,
  fallbackStages: string[],    // e.g. ["fast", "council"]
  createdAt: Date,
  updatedAt: Date
}
```

There is **no** `council_state` or `badge_type` field. The Council result screen derives:

```
pending  ← status in {pending, fast_classify, deep_analyze, council} && councilResult == null
complete ← councilResult != null && executionMode != fallback
fallback ← executionMode == fallback
```

Badges (do not persist as a parallel enum):

```
fallback              ← executionMode == fallback
echo_chamber          ← councilResult.aggregation == "echo_chamber"
independent_consensus ← aggregation == "consensus"
split_judgment        ← aggregation == "disagreement"
needs_review          ← needsReview == true (overlay on any aggregation)
```

### 6.5 Messages

```typescript
{
  _id: ObjectId,
  investigationId: ObjectId,
  role: 'user' | 'assistant' | 'system',
  content: string,
  stage: 'fast_classify' | 'deep_analyze' | 'council',
  createdAt: Date
}
```

### 6.6 Evidence

```typescript
{
  file: string,
  lines: string,       // e.g. "84-92"
  excerpt: string
}
```

### 6.7 CouncilResult

Matches `CouncilResult` / `CouncilModel` in [`graph/schema.graphqls`](../graph/schema.graphqls). Go vote overlay: `internal/llm/council.go`.

```typescript
{
  models: [
    {
      role: 'investigator' | 'challenger',
      hypothesis: string,
      confidence: number,          // 0.0–1.0; UI displays Math.round(confidence * 100)
      evidence: Evidence[],        // file + lines + excerpt (not a path-only list)
      model: string | null         // provider model id; else use Investigation.modelAName / modelBName
    }
  ],
  agreements: string[],
  disagreements: [
    { topic: string, investigator: string, challenger: string }
  ],
  finalJudgment: {
    mostLikelyCause: string,
    confidence: number,
    recommendedAction: string
  },
  aggregation: 'consensus' | 'echo_chamber' | 'disagreement',
  needsReview: boolean,
  aggregationNote: string
}
```

Judge output is **not** a separate GraphQL type. The Judge writes `agreements` / `disagreements` / `finalJudgment`; Go then sets `aggregation`, `needsReview`, and `aggregationNote`. Same-family similar hypotheses become `echo_chamber` (review flag), not independent consensus. `modelFamilies` / `sameFamily` are computed at vote time and are not stored.

On `executionMode: fallback`, do not present canned Council JSON as a live debate — UI shows Fast summary plus an explicit fallback badge. Mixed runs (`executionMode: mixed`) still render Council with a mixed-live warning.

### 6.8 ModelConfigs

```typescript
{
  _id: ObjectId,
  workspaceId: ObjectId,
  fastModel: string,
  deepModel: string,
  judgeModel: string,
  provider: 'openai' | 'local' | 'custom',
  updatedAt: Date
}
```

### 6.9 AuditLogs (Enterprise)

```typescript
{
  _id: ObjectId,
  workspaceId: ObjectId,
  userId: ObjectId,
  action: string,
  resource: string,
  metadata: object,
  createdAt: Date
}
```

---

## 7. GraphQL Schema (MVP)

**Source of truth:** [`graph/schema.graphqls`](../graph/schema.graphqls) (generated Go models in `graph/model`). Do not add parallel Council fields (`councilState`, `badgeType`, `isLiveModels`, `Judge.verdict`). The web client query is `INV_FIELDS` in `web/src/api.ts`.

Council-relevant excerpt:

```graphql
enum InvestigationStatus {
  PENDING
  FAST_CLASSIFY
  DEEP_ANALYZE
  COUNCIL
  COMPLETED
  FAILED
}

enum ExecutionMode {
  LIVE
  FALLBACK
  MIXED
}

type Evidence {
  file: String!
  lines: String!
  excerpt: String!
}

type FastResult {
  summary: String!
  incidentType: String!
  confidence: Float!
}

type DeepResult {
  rootCause: String!
  confidence: Float!
  evidence: [Evidence!]!
  suggestedFix: String!
}

type CouncilModel {
  role: String!
  hypothesis: String!
  confidence: Float!
  evidence: [Evidence!]!
  model: String
}

type Disagreement {
  topic: String!
  investigator: String!
  challenger: String!
}

type FinalJudgment {
  mostLikelyCause: String!
  confidence: Float!
  recommendedAction: String!
}

type CouncilResult {
  models: [CouncilModel!]!
  agreements: [String!]!
  disagreements: [Disagreement!]!
  finalJudgment: FinalJudgment!
  aggregation: String!
  needsReview: Boolean!
  aggregationNote: String!
}

type Investigation {
  id: ID!
  projectId: ID!
  prompt: String!
  status: InvestigationStatus!
  plan: [PlanStep!]!
  filesAccessed: [String!]!
  fastResult: FastResult
  deepResult: DeepResult
  councilResult: CouncilResult
  errorMessage: String
  modelAName: String
  modelBName: String
  modelCName: String
  escalationReason: String
  executionMode: ExecutionMode
  fallbackStages: [String!]!
  createdAt: String!
}
```

Web mapping helpers: `councilViewState` / `councilBadges` in `web/src/ui.tsx`.

---

## 8. Directory Structure (Suggested)

```
azula/
├── apps/
│   ├── web/                 # Next.js frontend
│   └── api/                 # GraphQL server
├── packages/
│   ├── shared/              # Types, GraphQL schema
│   ├── ai-router/           # ModelRouter, strategies, agents
│   └── mcp-connector/       # Files, Git connectors
├── docs/                    # Product docs (this folder)
├── samples/
│   └── broken-pipeline/     # Onboarding demo files
└── .cursor/
    ├── rules/
    └── skills/
```

---

## 9. Scalability Path

### MVP (Phase 1)
- Single Node.js process
- MongoDB single instance
- Synchronous investigation pipeline
- Target: 5 concurrent users

### Phase 2
```
Load Balancer
    ├── API Worker 1
    ├── API Worker 2
    └── API Worker 3
            ↓
        Job Queue (Bull / Redis)
            ↓
        LLM Router
```

### Phase 3
- Response caching (classification results)
- Multi-provider routing (fallback models)
- Horizontal MongoDB replica set
- Dedicated inference workers

---

## 10. MCP Configuration (MVP)

### Files Connector

```typescript
// Config
MCP_FILE_ROOT=./uploads

// Behavior
// - Files uploaded via GraphQL stored at: {MCP_FILE_ROOT}/{projectId}/{filename}
// - Agents read via MCPConnector.readFile(projectId, path)
// - No direct fs access outside MCPConnector
```

### Future Connectors

| Connector | Config | Phase |
|-----------|--------|-------|
| Git | `GIT_TOKEN`, repo URL | MVP+1 |
| GitHub | `GITHUB_TOKEN` | Post-MVP |
| MongoDB | `MONGODB_URI` (read-only views) | Post-MVP |

---

## 11. Security Notes

See [AGENTIC_SECURITY.md](AGENTIC_SECURITY.md) and [SECURITY.md](SECURITY.md).

- Web sessions: HttpOnly `azula_session` cookie (SameSite=Lax). Electron stores the JWT in the main process with OS `safeStorage` and attaches `Authorization` on GraphQL requests — the renderer never keeps the token in `localStorage`.
- JWT max age defaults to 8h (`JWT_EXPIRY`)
- File uploads: max 50MB, allowlisted extensions, project-scoped paths
- Git MCP: HTTPS only; clone hosts that resolve to private/loopback/link-local addresses are rejected
- LLM payloads: secret redaction + untrusted-file delimiters (prompt is not authorization)
- Production: `AZULA_ENV=production` requires a non-default `JWT_SECRET`; GraphQL playground/introspection off unless `AZULA_GRAPHQL_PLAYGROUND=true`
- Rate limits: 120 GraphQL req/min/IP; 20/min for login/register
- Kill switch: `AZULA_KILL_SWITCH=true`; per-run `cancelInvestigation`
- Security headers on the API; CSP/frame/nosniff on nginx
- Audit: register/login plus `agent.start`, `agent.cancel`, `mcp.read`

---

## 12. Related Documents

- [AGENTIC_SECURITY.md](AGENTIC_SECURITY.md) — agent/MCP controls
- [SECURITY.md](SECURITY.md) — disclosure and production checklist
- [MVP.md](MVP.md) — scoped first release
- [ROADMAP.md](ROADMAP.md) — phased delivery
