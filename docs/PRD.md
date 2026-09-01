# Azula — Product Requirements Document

**Version:** 1.0  
**Status:** Draft  
**Product:** Azula — AI workspace for data/ML engineering  
**Tagline:** *Generate. Investigate. Debate. Improve.*

---

## 1. Executive Summary

Azula is an enterprise AI platform that helps data science and machine learning teams automatically investigate technical incidents — pipeline failures, model training crashes, data quality problems, and configuration errors.

Users connect logs, code, configs, and datasets to a project workspace. Azula classifies the problem with a fast model, escalates to deep analysis when needed, and uses MCP (Model Context Protocol) connectors to inspect project files. For high-stakes conclusions, the **AI Council** runs two models in opposing roles (Investigator vs Challenger) and a Judge synthesizes a final judgment with confidence scores, evidence links, and recommended actions.

Unlike general-purpose multi-model tools (e.g. Perplexity Model Council), Azula is specialized for **data/ML engineering workflows**: datasets, pipelines, training logs, metrics, and model behavior.

---

## 2. Problem Statement

ML and data engineering teams spend hours manually tracing failures across:

- Training logs and stack traces
- Pipeline configuration files
- Dataset schemas and drift reports
- Dependency and infrastructure errors

Existing tools are fragmented: log viewers, notebook debuggers, MLOps dashboards, and generic AI chatbots do not connect evidence to root cause in a single investigation workspace.

**Azula's promise:** Answer *what happened, why it happened, and how to fix it* in one place — with traceable evidence and multi-model validation.

---

## 3. Target Users

| Persona | Primary need |
|---------|--------------|
| ML Engineer | Debug training failures, hyperparameter issues, GPU/memory errors |
| Data Engineer | Trace pipeline breaks, schema mismatches, dependency failures |
| MLOps Engineer | Monitor incident patterns, audit investigations, manage model routing |
| Data Science Lead | Review team incident analytics, validate AI recommendations |

---

## 4. Product Modes

### 4.1 Investigate

**User question:** *"Why did my pipeline or model fail?"*

**Flow:**
```
Upload context → Fast classification → Deep analysis (optional) → Council (optional) → Root cause + fix
```

**Outputs:**
- Incident type classification
- Root cause hypothesis with confidence score
- Affected components
- Evidence references (file, line, excerpt)
- Recommended fix with expected impact

### 4.2 Generate

**User question:** *"Create training or evaluation data for this problem."*

**Flow:**
```
Problem context → Synthetic Data Agent → JSONL / CSV output → Optional evaluation
```

**Example prompt:** *"Generate 5,000 Turkish customer support problems with ideal responses."*

**Outputs:**
- Synthetic dataset file
- Generation metadata (schema, distribution, quality notes)
- Link to originating investigation (if applicable)

### 4.3 Council

**User question:** *"Which hypothesis is correct?"*

**Flow:**
```
Shared context → Model A (Investigator) + Model B (Challenger) → Judge (Synthesizer) → Final judgment
```

**Model roles:**
- **Investigator (Model A):** Builds and defends a root-cause hypothesis
- **Challenger (Model B):** Questions the Investigator's analysis, proposes alternatives
- **Judge:** Compares arguments, surfaces agreements/disagreements, produces final judgment

**Outputs:**
- Per-model hypothesis + confidence
- Agreements section
- Disagreements section
- Final judgment + overall confidence
- Evidence file list

### 4.4 Evaluate

**User question:** *"Is the proposed fix actually better?"*

**Flow:**
```
Original vs fixed dataset/model → Evaluation Agent → Metrics comparison → Council (optional)
```

**Outputs:**
- Before/after metrics
- Improvement or regression summary
- Recommendation to adopt or reject fix

---

## 5. AI Council — Core Differentiator

### 5.1 Design Principles

1. **Role separation:** Models do not just produce parallel answers — they have distinct adversarial roles.
2. **Evidence grounding:** All claims must reference MCP-accessible files.
3. **Transparent disagreement:** Disagreements are surfaced, not hidden.
4. **Synthesized judgment:** A dedicated Judge model resolves conflicts.

### 5.2 Council Output Schema

```json
{
  "investigationId": "inv_abc123",
  "models": [
    {
      "role": "investigator",
      "hypothesis": "Schema drift detected in feature column `customer_status`",
      "confidence": 0.89,
      "evidence": [
        { "file": "dataset.jsonl", "lines": "1204-1210", "excerpt": "..." }
      ]
    },
    {
      "role": "challenger",
      "hypothesis": "Data leakage from target column into training features",
      "confidence": 0.71,
      "evidence": [
        { "file": "pipeline.py", "lines": "84-92", "excerpt": "..." }
      ]
    }
  ],
  "agreements": [
    "Both models detected data quality issues in the training set."
  ],
  "disagreements": [
    {
      "topic": "Root cause",
      "investigator": "Schema drift",
      "challenger": "Data leakage"
    }
  ],
  "finalJudgment": {
    "mostLikelyCause": "Schema drift in `customer_status`",
    "confidence": 0.91,
    "recommendedAction": "Remove or re-encode `customer_status`; retrain with corrected schema",
    "simulation": {
      "accuracyChange": "-1.8%",
      "generalizationImprovement": "expected"
    }
  }
}
```

### 5.3 Full Investigation Pipeline

```
INCIDENT DETECTED
       ↓
Fast Diagnosis (classify incident type)
       ↓
Deep Investigation (MCP file analysis)
       ↓
AI COUNCIL
  /          \
Model A    Model B
  \          /
     Judge
       ↓
Root Cause + Suggested Fix
```

---

## 6. Six Product Pillars

### 6.1 Onboarding

**Goal:** Answer *"what does this product do?"* in 30 seconds — not with feature lists, but with a live demo.

**Opening line:**
> *"Your pipeline failed. Let's find out why."*

**Flow:**
1. Create Workspace
2. Connect Project
3. Run First Investigation

**Sample pipeline:** A pre-loaded broken ML pipeline is provided so the user experiences:

```
Upload logs → AI Investigation → Root Cause → Suggested Fix
```

within the first minute — without uploading their own files.

See [ONBOARDING.md](ONBOARDING.md) for screen-by-screen spec.

### 6.2 Monetization

B2B SaaS with tiered plans. Revenue comes from depth of analysis, team features, and enterprise compliance — not just seat count.

| Tier | Projects | Investigations/month | Models | Key features |
|------|----------|----------------------|--------|--------------|
| **Free / Developer** | 3 | 10 | Fast only | Basic MCP (file upload) |
| **Pro** | Unlimited | 100 | Fast + Deep + Council | History, versioning, model selection |
| **Enterprise** | Unlimited | Custom | Custom / LoRA | MFA, audit logs, GDPR, private MCP, RBAC, API, team workspace |

See [MONETIZATION.md](MONETIZATION.md) for enforceable limit keys.

### 6.3 Analytics

Measure **problem resolution**, not vanity metrics like login count.

**Primary dashboard metrics:**
- Total investigations
- Resolved by AI (%)
- Avg investigation time (AI vs human baseline)
- Top root causes (schema mismatch, data drift, GPU/memory, dependency failure, config error)

**Model performance:**
- Fast model: avg response time, accuracy
- Deep model: avg response time, accuracy

**Council metrics:**
- Agreement rate
- Model A vs Model B win rate
- Council overturn rate
- Synthetic datasets generated

See [ANALYTICS.md](ANALYTICS.md) for metric definitions.

### 6.4 Scalability

**MVP (Phase 1):**
```
User → Web → GraphQL API → MongoDB → LLM Router → Fast/Deep Model → MCP
```
Target: 5 concurrent users on single-node deployment.

**Phase 2:**
```
Load Balancer → Worker 1..N → LLM Router → MCP
```

**Phase 3:**
- Job queue for long investigations
- Response caching
- Multi-provider model routing
- Horizontal scaling
- Custom inference providers

### 6.5 Architecture

```
             WEB / ELECTRON
                    │
                    ▼
              GraphQL API
                    │
          ┌─────────┴─────────┐
          ▼                   ▼
      Auth Service       Investigation Service
                              │
                              ▼
                         AI Model Router
                       ┌──────┴──────┐
                       ▼             ▼
                  FAST MODEL     DEEP MODEL
                       │             │
                       └──────┬──────┘
                              ▼
                             MCP
                              │
                ┌─────────────┼─────────────┐
                ▼             ▼             ▼
              Logs           Code         Config
                              │
                              ▼
                           MongoDB
```

**MongoDB collections:**
- `Users`
- `Workspaces`
- `Projects`
- `Investigations`
- `Messages`
- `ModelConfigs`
- `MCPContexts`
- `Versions`
- `AuditLogs`

See [ARCHITECTURE.md](ARCHITECTURE.md) for implementation details.

### 6.6 Maintainability

All external dependencies are behind abstractions:

```
LLMProvider
  ├── OpenAIProvider
  ├── LocalProvider
  └── CustomProvider

ModelRouter
  ├── FastStrategy
  └── DeepStrategy

MCPConnector
  ├── FilesConnector
  ├── GitConnector
  └── DatabaseConnector

Agent modules
  ├── InvestigatorAgent
  ├── ChallengerAgent
  ├── JudgeAgent
  ├── GeneratorAgent
  └── EvaluatorAgent
```

**Rule:** Swapping OpenAI for another provider or adding GitHub MCP must not require rewriting the investigation pipeline.

---

## 7. MCP Integration

Azula agents access user project context through MCP connectors.

| Connector | Access | Phase |
|-----------|--------|-------|
| Files | Uploaded logs, code, config, datasets | MVP |
| Git | Clone, blame, diff | MVP+1 |
| Database | Query investigation history | Post-MVP |
| GitHub | PR/issue context | Post-MVP |

Agents must only access MCP through the `MCPConnector` abstraction — never direct filesystem calls from resolvers.

---

## 8. Non-Functional Requirements

| Requirement | Target (MVP) |
|-------------|--------------|
| Fast model response (p95) | < 5 seconds |
| Deep model response (p95) | < 30 seconds |
| Time to first root cause (sample pipeline) | < 60 seconds |
| Evidence traceability | 100% of claims link to source file |
| Confidence scores | Required on all hypotheses |
| Concurrent users | 5 without degradation |
| Uptime | 99% (post-MVP SLA for Enterprise) |

---

## 9. Security & Compliance (Enterprise)

| Feature | Tier |
|---------|------|
| Email/password auth | MVP |
| MFA | Enterprise |
| Trusted devices | Enterprise |
| Audit logs | Enterprise |
| GDPR / KVKK data handling | Enterprise |
| Role-based access control | Enterprise |
| Private MCP sources | Enterprise |
| API access with scoped tokens | Enterprise |

---

## 10. Out of Scope (v1)

- Custom LoRA training UI
- Full enterprise SSO (SAML/OIDC) — roadmap item
- Mobile native app
- Real-time collaborative editing
- Automated pipeline repair (suggest only, not auto-apply)

---

## 11. Success Metrics

| Metric | Target |
|--------|--------|
| Onboarding completion rate | > 70% |
| Time to first root cause (demo) | < 60s |
| AI resolution rate | > 60% (MVP), > 71% (GA) |
| Council structured output rate | > 90% |
| Pro conversion (from Free) | Track; target TBD |

---

## 12. Competitive Positioning

| Product | Focus | Azula difference |
|---------|-------|------------------|
| Perplexity Model Council | General research, multi-model synthesis | Azula: ML/data engineering incidents with MCP-grounded evidence |
| Dropoutt | Dataset analysis, synthetic data, training groups | Azula: full incident lifecycle + Council debate + fix evaluation |
| Generic AI chatbots | Ad-hoc Q&A | Azula: structured investigation workflow with confidence and evidence |

---

## 13. Related Documents

- [MVP.md](MVP.md) — scoped first release
- [ARCHITECTURE.md](ARCHITECTURE.md) — technical design
- [ROADMAP.md](ROADMAP.md) — phased delivery
- [ONBOARDING.md](ONBOARDING.md) — first-run experience
- [MONETIZATION.md](MONETIZATION.md) — pricing tiers
- [ANALYTICS.md](ANALYTICS.md) — metrics spec
