# Azula — Go Backend Architecture (Delivery)

**Stack:** Go · GraphQL (gqlgen) · MongoDB · Electron + Web  
**Deadline build:** See [DELIVERY_SPEC.md](DELIVERY_SPEC.md)

---

## Layer diagram

```
cmd/api/main.go
    └── graphql/          # gqlgen generated + resolvers
    └── internal/
            domain/       # entities, interfaces (no external deps)
            auth/         # JWT, MFA, trusted devices
            investigation/
            llm/          # router, providers, worker pool
            mcp/          # file read, version store
            finetune/     # LoRA/QLoRA jobs
            gdpr/         # export, delete, consent
            repository/   # MongoDB implementations
```

---

## GraphQL API (minimum for jury)

```graphql
type Mutation {
  register(email: String!, password: String!): AuthPayload!
  login(email: String!, password: String!, mfaCode: String, deviceId: String!): AuthPayload!
  enrollMfa: MfaEnrollPayload!
  deleteAccount: Boolean!
  exportMyData: String!

  createOrganization(name: String!): Organization!
  createProject(workspaceId: ID!, name: String!): Project!
  uploadFile(projectId: ID!, file: Upload!): ProjectFile!
  swapFileVersion(projectId: ID!, fileName: String!, version: Int!): ProjectFile!

  updateModelConfig(input: ModelConfigInput!): ModelConfig!
  startFineTuneJob(input: FineTuneInput!): FineTuneJob!

  startInvestigation(projectId: ID!, prompt: String): Investigation!
}

type Query {
  me: User!
  projects(workspaceId: ID!): [Project!]!
  investigation(id: ID!): Investigation!
  modelConfig(workspaceId: ID!): ModelConfig!
  fineTuneJobs(workspaceId: ID!): [FineTuneJob!]!
  llmOpsMetrics(workspaceId: ID!): LLMOpsMetrics!
}

type Investigation {
  id: ID!
  status: InvestigationStatus!
  plan: [PlanStep!]!
  fastResult: FastResult
  deepResult: DeepResult
  councilResult: CouncilResult
}

type PlanStep {
  order: Int!
  description: String!
  status: StepStatus!
}
```

---

## LLM worker pool

```go
package llm

type Router struct {
    mu      sync.Mutex
    slots   [5]Slot
    modelA  Provider
    modelB  Provider
    config  ModelConfig
}

func (r *Router) Acquire() (*Slot, error) {
    // least queue depth among 5 slots
}

func (r *Router) Route(stage Stage) Provider {
    switch stage {
    case StageFast, StagePlan:
        return r.pick(r.config.ModelAId, r.modelA)
    case StageDeep, StageCouncil:
        return r.pick(r.config.ModelBId, r.modelB)
    }
}
```

---

## MongoDB indexes

```javascript
db.investigations.createIndex({ projectId: 1, createdAt: -1 })
db.users.createIndex({ email: 1 }, { unique: true })
db.auditLogs.createIndex({ userId: 1, createdAt: -1 })
db.fileVersions.createIndex({ projectId: 1, fileName: 1, version: -1 })
```

---

## masterfabric-go usage

Use as **reference** for hexagonal layout and auth patterns. Azula uses **MongoDB**, not masterfabric's PostgreSQL. Copy:

- JWT middleware pattern
- Audit log handler
- RBAC permission checks (simplified for demo)

Do **not** spend >4h integrating full masterfabric monolith — time budget does not allow.

---

## Electron

```
electron/main.js → load WEB_URL (dev: http://localhost:3001)
electron-builder → targets: win, mac
```

Web and Electron share one GraphQL endpoint.
