# Azula — MVP Specification

**Version:** 1.0  
**Status:** Ready for implementation  
**Goal:** Prove the investigation + AI Council loop in a single workspace.

---

## 1. MVP Mission Statement

> A user can connect a broken ML project, run an investigation, and receive a Council-backed root cause with evidence and a suggested fix — in one workspace.

The MVP is **not** a full platform. It is a focused proof that Azula's core value proposition works: fast incident classification, deep MCP-grounded analysis, and multi-model Council synthesis.

---

## 2. Scope Summary

### In Scope

| Area | Included | Notes |
|------|----------|-------|
| Auth | Yes | Email/password, single user |
| Workspace + Project | Yes | Create, list, select |
| Onboarding | Yes | Sample broken pipeline demo |
| Investigate mode | Yes | Fast → Deep escalation |
| AI Council | Yes | Investigator + Challenger + Judge |
| MCP: file upload | Yes | logs, config, code, JSONL |
| Investigation UI | Yes | Root cause, confidence, evidence, fix |
| Investigation history | Yes | Per-project list |
| Basic analytics | Yes | Count, avg time, top causes, fast vs deep |
| MongoDB | Yes | Core collections |
| GraphQL API | Yes | Projects, investigations, messages |
| Free tier limits | Yes | Config-enforced caps |

### Out of Scope (Deferred)

| Area | Deferred to | Reason |
|------|-------------|--------|
| Generate mode (synthetic data) | MVP+1 | Focus on investigation first |
| Evaluate mode | MVP+1 | Depends on Generate |
| Git MCP connector | MVP+1 | File upload sufficient for demo |
| Electron desktop app | Post-MVP | Web-first |
| Payment / billing | Post-MVP | Document tiers, enforce via config |
| Enterprise: MFA, audit, RBAC, API | Enterprise phase | Not needed for proof |
| Team workspaces | Team phase | Single-user MVP |
| Version diff UI | MVP+1 | Store versions in DB, minimal UI |
| Custom / LoRA models | Enterprise | Standard models only |

---

## 3. User Stories & Acceptance Criteria

### US-1: Onboarding Demo

**As a** new user  
**I want to** complete onboarding and see a root cause for a sample pipeline  
**So that** I understand what Azula does within 60 seconds

**Acceptance criteria:**
- [ ] Opening screen shows: *"Your pipeline failed. Let's find out why."*
- [ ] User can create a workspace and project without external setup
- [ ] Sample pipeline is pre-loaded (no upload required)
- [ ] Investigation completes and shows root cause + suggested fix
- [ ] Total time from signup to root cause < 60 seconds

### US-2: Upload Project Files

**As an** ML engineer  
**I want to** upload my training logs, config, and code  
**So that** Azula can investigate my actual incident

**Acceptance criteria:**
- [ ] Supported file types: `.log`, `.yaml`, `.yml`, `.py`, `.json`, `.jsonl`, `.csv`, `.txt`
- [ ] Files are associated with a project
- [ ] Uploaded files are accessible to MCP agents during investigation
- [ ] File list is visible in project view

### US-3: Fast Classification

**As an** ML engineer  
**I want to** see a fast initial classification of my incident  
**So that** I get immediate orientation on the problem type

**Acceptance criteria:**
- [ ] Fast model returns incident type within 5 seconds (p95)
- [ ] Response includes: type, brief summary, confidence score
- [ ] UI shows loading state during classification
- [ ] User can proceed to deep analysis or skip

### US-4: Deep Investigation

**As an** ML engineer  
**I want to** run deep analysis that reads my project files  
**So that** I get evidence-backed root cause findings

**Acceptance criteria:**
- [ ] Deep model reads files via MCP connector
- [ ] Response includes evidence links (file name + line/excerpt)
- [ ] Deep analysis completes within 30 seconds (p95)
- [ ] Findings are stored in investigation record

### US-5: AI Council

**As an** ML engineer  
**I want to** see two models debate the root cause  
**So that** I trust the conclusion more than a single model answer

**Acceptance criteria:**
- [ ] Investigator model produces hypothesis + confidence + evidence
- [ ] Challenger model questions Investigator and proposes alternative
- [ ] Judge produces: agreements, disagreements, final judgment
- [ ] Council output is structured JSON (see PRD schema)
- [ ] UI renders all three sections clearly

### US-6: Evidence Traceability

**As an** ML engineer  
**I want** every root-cause claim to link to source evidence  
**So that** I can verify the AI's reasoning

**Acceptance criteria:**
- [ ] Each claim has at least one evidence reference
- [ ] Evidence shows file name and excerpt
- [ ] Clicking evidence opens or highlights the source file
- [ ] Claims without evidence are flagged in UI

### US-7: Investigation History

**As a** user  
**I want to** see past investigations for my project  
**So that** I can reference previous findings

**Acceptance criteria:**
- [ ] Project page lists all investigations (newest first)
- [ ] Each entry shows: date, incident type, status, confidence
- [ ] User can open a past investigation and view full results

### US-8: Basic Analytics

**As a** user  
**I want to** see investigation statistics  
**So that** I understand patterns in my project's incidents

**Acceptance criteria:**
- [ ] Dashboard shows: total investigations, avg investigation time
- [ ] Top 3 root causes displayed with percentages
- [ ] Fast vs Deep model comparison (response time, accuracy if available)
- [ ] Data updates after each completed investigation

---

## 4. Investigation State Machine

```
pending
  → fast_classify
  → deep_analyze (optional, user-triggered or auto-escalate)
  → council
  → completed

Error paths:
  any state → failed (with error message)
  fast_classify → completed (if user skips deep + council)
```

**Auto-escalation rule (MVP):** If fast model confidence < 0.7, automatically trigger deep analysis.

---

## 5. Sample Pipeline (Onboarding Demo)

Pre-loaded project: `sample-broken-pipeline`

**Files included:**

| File | Purpose |
|------|---------|
| `training.log` | Shows `CUDA out of memory` and schema warning |
| `config.yaml` | Batch size too large, learning rate misconfigured |
| `pipeline.py` | Feature engineering bug (target leakage) |
| `dataset.jsonl` | Schema mismatch in `customer_status` field |
| `metrics.json` | Declining validation accuracy |

**Expected root cause (demo):** Schema drift in `customer_status` combined with batch size causing OOM.

**Expected fix:** Reduce batch size, fix schema encoding, remove leaky feature.

---

## 6. API Endpoints (GraphQL Sketch)

```graphql
type Query {
  me: User
  workspaces: [Workspace!]!
  project(id: ID!): Project
  investigation(id: ID!): Investigation
  analytics(workspaceId: ID!): AnalyticsSummary
}

type Mutation {
  createWorkspace(name: String!): Workspace
  createProject(workspaceId: ID!, name: String!): Project
  uploadFile(projectId: ID!, file: Upload!): ProjectFile
  startInvestigation(projectId: ID!, prompt: String): Investigation
  escalateInvestigation(id: ID!, stage: InvestigationStage!): Investigation
}

enum InvestigationStage {
  FAST_CLASSIFY
  DEEP_ANALYZE
  COUNCIL
}
```

---

## 7. Technical Stack

| Layer | Technology | Notes |
|-------|------------|-------|
| Frontend | Web (React / Next.js) | TBD at implementation |
| API | GraphQL | Apollo or similar |
| Database | MongoDB | Core collections |
| AI | LLM Router | Fast + Deep + Judge models |
| Tools | MCP | File connector (MVP) |
| Auth | JWT / session | Email + password |

---

## 8. Environment Variables (MVP)

```env
MONGODB_URI=
JWT_SECRET=
OPENAI_API_KEY=          # or provider of choice
FAST_MODEL=gpt-4o-mini
DEEP_MODEL=gpt-4o
JUDGE_MODEL=gpt-4o-mini
MCP_FILE_ROOT=./uploads
FREE_TIER_MAX_PROJECTS=3
FREE_TIER_MAX_INVESTIGATIONS=10
```

---

## 9. Free Tier Limits (MVP)

| Limit | Value | Enforcement |
|-------|-------|-------------|
| Max projects | 3 | Block `createProject` when exceeded |
| Investigations/month | 10 | Block `startInvestigation` when exceeded |
| Models available | Fast only | Deep + Council require Pro (UI gate) |
| MCP connectors | Files only | Git/DB connectors hidden |

---

## 10. Success Metrics

| Metric | Target |
|--------|--------|
| Time to first root cause (demo) | < 60s |
| Fast model p95 latency | < 5s |
| Deep model p95 latency | < 30s |
| Council structured output rate | > 90% |
| Evidence link coverage | 100% of claims |
| 5 concurrent users | No errors or >2x latency |

---

## 11. Definition of Done

- [ ] All 8 user stories pass manual QA
- [ ] Sample pipeline demo works end-to-end without user uploads
- [ ] Council produces agreements + disagreements + final judgment
- [ ] Analytics dashboard shows investigation stats
- [ ] Free tier limits enforced
- [ ] README and docs linked from app
- [ ] No secrets in repository

---

## 12. MVP+1 Preview (Not in MVP)

These ship immediately after MVP validation:

1. **Generate mode** — synthetic dataset from investigation context
2. **Evaluate mode** — compare original vs fixed dataset metrics
3. **Git MCP** — clone repo, blame, diff
4. **Version diff UI** — compare file versions across investigations
5. **Pro tier** — unlock Deep + Council + higher limits

See [ROADMAP.md](ROADMAP.md) for full timeline.
