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
                         │
            ┌────────────┼────────────┐
            │ confidence < 0.7       │ user skips
            ▼                        ▼
    ┌──────────────┐          ┌───────────┐
    │ deep_analyze │          │ completed │
    └──────┬───────┘          └───────────┘
           │
           ▼
      ┌─────────┐
      │ council │
      └────┬────┘
           │
           ▼
      ┌───────────┐
      │ completed │
      └───────────┘

  Any state ──error──▶ failed
```

**State transitions are enforced in `InvestigationService`** — resolvers must not skip states.

---

## 4. AI Council Pipeline

```
InvestigationService
       │
       ▼
  ModelRouter.runCouncil(context)
       │
       ├──▶ InvestigatorAgent (Deep model)
       │         └── MCP: read files, build hypothesis
       │
       ├──▶ ChallengerAgent (Deep model, different prompt)
       │         └── MCP: read files, challenge hypothesis
       │
       └──▶ JudgeAgent (Fast/Judge model)
                 └── Synthesize: agreements, disagreements, final judgment
```

**Parallelism:** Investigator and Challenger run in parallel. Judge waits for both.

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

```typescript
{
  _id: ObjectId,
  projectId: ObjectId,
  userId: ObjectId,
  status: 'pending' | 'fast_classify' | 'deep_analyze' | 'council' | 'completed' | 'failed',
  prompt: string,
  incidentType: string | null,
  fastResult: {
    summary: string,
    incidentType: string,
    confidence: number,
    completedAt: Date
  } | null,
  deepResult: {
    rootCause: string,
    confidence: number,
    evidence: Evidence[],
    suggestedFix: string,
    completedAt: Date
  } | null,
  councilResult: CouncilResult | null,
  error: string | null,
  startedAt: Date,
  completedAt: Date | null
}
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

```typescript
{
  models: [
    {
      role: 'investigator' | 'challenger',
      hypothesis: string,
      confidence: number,
      evidence: Evidence[]
    }
  ],
  agreements: string[],
  disagreements: [
    { topic: string, investigator: string, challenger: string }
  ],
  finalJudgment: {
    mostLikelyCause: string,
    confidence: number,
    recommendedAction: string,
    simulation: object | null
  }
}
```

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

```graphql
type User {
  id: ID!
  email: String!
  tier: Tier!
}

type Workspace {
  id: ID!
  name: String!
  projects: [Project!]!
}

type Project {
  id: ID!
  name: String!
  isSample: Boolean!
  files: [ProjectFile!]!
  investigations: [Investigation!]!
}

type ProjectFile {
  name: String!
  path: String!
  mimeType: String!
  uploadedAt: String!
}

type Investigation {
  id: ID!
  status: InvestigationStatus!
  prompt: String
  incidentType: String
  fastResult: FastResult
  deepResult: DeepResult
  councilResult: CouncilResult
  messages: [Message!]!
  startedAt: String!
  completedAt: String
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

type CouncilResult {
  models: [CouncilModelOutput!]!
  agreements: [String!]!
  disagreements: [Disagreement!]!
  finalJudgment: FinalJudgment!
}

type AnalyticsSummary {
  totalInvestigations: Int!
  resolvedByAiPercent: Float!
  avgInvestigationTimeSec: Float!
  topRootCauses: [RootCauseStat!]!
  fastModel: ModelStats!
  deepModel: ModelStats!
}

enum InvestigationStatus {
  PENDING
  FAST_CLASSIFY
  DEEP_ANALYZE
  COUNCIL
  COMPLETED
  FAILED
}

enum Tier {
  FREE
  PRO
  ENTERPRISE
}
```

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

- JWT tokens expire after 24h (MVP)
- File uploads: max 50MB per file, whitelist extensions
- MCP reads scoped to project directory only (no path traversal)
- API keys stored in environment variables, never in DB
- Enterprise: audit log on all investigation and file access events

---

## 12. Related Documents

- [PRD.md](PRD.md) — product requirements
- [MVP.md](MVP.md) — scoped first release
- [ROADMAP.md](ROADMAP.md) — phased delivery
