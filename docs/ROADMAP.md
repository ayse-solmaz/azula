# Azula — Product Roadmap

**Last updated:** 2026-09-04

Product is **ahead of this file’s old empty checkboxes**. Phase 1 Investigate + Council is implemented in Go/GraphQL/web. Delivery-spec items (MFA, trusted devices, GDPR, Electron, QLoRA) were pulled forward for the jury date. They are marked below where they exist in code.

**Shipped this pass:** Generate, Evaluate, Git MCP, Stripe/Pro gates, OIDC SSO, org RBAC, Kubernetes starter manifests, **LLM diversity + Council aggregation + prompt/context docs**.

---

## Overview

```
MVP ──▶ MVP+1 ──▶ Team ──▶ Enterprise
 │        │          │           │
 │        │          │           └── Custom models, MFA, audit, API
 │        │          └── Multi-user workspaces, RBAC
 │        └── Generate, Evaluate, Git MCP
 └── Investigate + Council (core loop)
```

---

## Phase 1: MVP

**Timeline:** Weeks 1–6  
**Goal:** Prove investigation + AI Council loop

### Deliverables

- [x] Auth (email/password)
- [x] Workspace + Project CRUD
- [x] File upload (MCP Files connector)
- [x] Onboarding with sample broken pipeline
- [x] Investigate mode: Fast → Deep → Council
- [x] Investigation results UI (root cause, evidence, fix)
- [x] Investigation history
- [x] Basic analytics dashboard (LLMOps counts, avg confidence, top causes)
- [x] MongoDB persistence
- [x] GraphQL API
- [x] Free tier limits (config-enforced project cap)
- [x] Cursor rules + investigation skill
- [x] Escalation reason shown in the investigation UI
- [x] `executionMode` live/fallback/mixed badge (demo honesty)
- [x] Token-budget file selection for Deep/Council
- [x] Small gold-set eval (`samples/goldset`, `internal/eval`)
- [x] CI: `.github/workflows/test.yml` + `lint.yml` (+ `govulncheck`, `npm audit`)
- [x] Agentic security controls (HttpOnly session, Git SSRF, secret redaction, kill switch) — [AGENTIC_SECURITY.md](AGENTIC_SECURITY.md)
- [x] Electron desktop — Windows pack (`scripts/pack-electron.ps1`); Mac DMG on macOS/CI only (`scripts/pack-electron.sh`)

### Success Criteria

- Demo pipeline: signup → root cause < 60s (live Ollama Fast + Deep)
- Council structured JSON (`agreements`, `disagreements`, `finalJudgment`)
- 5 concurrent users: `go run ./scripts/loadtest.go` against a running API (worker pool, not device-limit proof)

### Not in MVP (unchanged)

- Payment integration
- Generate / Evaluate modes
- Git MCP
- Full team RBAC / SSO
- Job queue / durable workers (Scale)
- Refresh tokens
- Executor / dedicated inference workers

---

## Phase 2: MVP+1

**Timeline:** Weeks 7–10  
**Goal:** Complete the 4-mode product loop

### Deliverables

- [x] **Generate mode** — synthetic dataset from investigation context
- [x] **Evaluate mode** — compare original vs fixed dataset metrics
- [x] **Git MCP connector** — clone, blame, diff
- [x] Version diff UI (compare / swap file versions on the project page)
- [x] Pro tier UI gates (upgrade prompts)
- [x] Model selection (Fast/Deep names, temperature, role prompts on LLM dashboard)
- [x] Full analytics dashboard (Council metrics, model comparison) — LLMOps subset shipped
- [ ] Investigation export (PDF) — GDPR JSON export exists, not PDF

### Success Criteria

- End-to-end flow: Investigate → Generate fix dataset → Evaluate → Council
- Git MCP reads repo files for investigation
- Pro upgrade prompts shown at correct gate points

---

## Phase 3: Team

**Timeline:** Weeks 11–16  
**Goal:** Multi-user collaboration

### Deliverables

- [x] Team workspaces (org create + email invite — delivery stub, not full product)
- [x] Role-based access: Admin, Engineer, Viewer (enforced on mutations)
- [x] Shared investigation history (org workspace)
- [x] Payment integration (Stripe Checkout + webhook; demo upgrade when keys are unset)
- [x] Pro tier activation
- [ ] Email notifications (investigation complete, limit warnings) — device OTP email only
- [ ] Improved onboarding for team admins

### Success Criteria

- Team of 5 can share projects and investigations
- Pro subscription flow works end-to-end
- RBAC enforced on all sensitive actions

---

## Phase 4: Enterprise

**Timeline:** Weeks 17–24  
**Goal:** Enterprise-ready platform

### Deliverables

- [x] MFA + trusted devices (pulled into jury delivery)
- [x] Audit logs (auth, investigation, GDPR events)
- [x] GDPR / KVKK: consent, export, account deletion (residency options not shipped)
- [ ] API access with scoped tokens
- [x] Custom / LoRA model support (QLoRA trainer + Ollama import)
- [ ] Private MCP connectors (internal repos, databases)
- [ ] Custom Council lineup (2–8 models)
- [ ] Higher concurrent user limits (fixed 5 worker slots)
- [x] SSO (OIDC authorization-code + ID token)
- [x] Electron desktop app — Windows installer locally; Mac unsigned DMG via macOS or GitHub Actions
- [ ] SLA and dedicated support tooling

### Success Criteria

- Pass security questionnaire for 1 enterprise pilot
- API documented and functional
- Audit log covers all investigation and file access events

---

## Phase 5: Scale

**Timeline:** Post-GA  
**Goal:** Horizontal scaling and multi-provider routing

### Deliverables

- [ ] Job queue for long investigations (Bull/Redis)
- [x] Worker pool behind load balancer / Kubernetes (`deploy/k8s`)
- [ ] Response caching
- [x] Multi-provider model routing (OpenAI + Ollama; Anthropic/Gemini not wired)
- [ ] MongoDB replica set
- [ ] Dedicated inference workers
- [ ] User-configurable Council (GPT + Claude + Gemini + Llama)

---

## Feature Timeline Matrix

| Feature | MVP | MVP+1 | Team | Enterprise | Scale |
|---------|-----|-------|------|------------|-------|
| Investigate | ✓ | ✓ | ✓ | ✓ | ✓ |
| Council | ✓ | ✓ | ✓ | ✓ | ✓ |
| Generate | | ✓ | ✓ | ✓ | ✓ |
| Evaluate | | ✓ | ✓ | ✓ | ✓ |
| Files MCP | ✓ | ✓ | ✓ | ✓ | ✓ |
| Git MCP | | ✓ | ✓ | ✓ | ✓ |
| Private MCP | | | | ✓ | ✓ |
| Basic analytics | ✓ | | | | |
| Full analytics | | ✓ | ✓ | ✓ | ✓ |
| Free tier | ✓ | ✓ | ✓ | ✓ | ✓ |
| Pro tier | | ✓ | ✓ | ✓ | ✓ |
| Payment | | | ✓ | ✓ | ✓ |
| Team workspace | | | ✓ | ✓ | ✓ |
| RBAC | | | ✓ | ✓ | ✓ |
| MFA | | | | ✓ | ✓ |
| Audit logs | | | | ✓ | ✓ |
| API | | | | ✓ | ✓ |
| Custom models | | | | ✓ | ✓ |
| Electron | ✓ (Win pack; Mac on macOS/CI) | ✓ | ✓ | ✓ | ✓ |
| Job queue | | | | | ✓ |
| Multi-provider | | | | | ✓ |
| SSO | | | | ✓ | ✓ |
| Kubernetes | | | | | ✓ |

MFA, audit, GDPR export/delete, org invite, and LoRA shipped early for the 6 Sep 2026 delivery spec; they remain Enterprise-shaped in this matrix. Generate / Evaluate / Git MCP / Stripe Pro / OIDC SSO / K8s starter manifests are implemented (SAML, multi-region, and a Redis job queue are still later).

---

## Risk Register

| Risk | Impact | Mitigation |
|------|--------|------------|
| LLM latency too high for demo | High | Use fast model for demo; pre-cache sample pipeline results |
| Council output inconsistent | High | Strict JSON schema validation; retry on malformed output |
| MCP file access security | High | Path traversal protection; project-scoped directories |
| Free tier abuse | Medium | Rate limiting; investigation cap enforcement |
| Enterprise sales cycle long | Medium | Ship Team phase before full Enterprise; land Pro first |
| Product only on a laptop | High | Push source to GitHub; do not commit weights or `electron/dist` |
| Mac DMG from Windows | High | Build on macOS or GitHub Actions `desktop` workflow; unsigned DMG |

---

## Related Documents

- [MVP.md](MVP.md) — Phase 1 detailed spec
- [PRD.md](PRD.md) — full product requirements
- [ARCHITECTURE.md](ARCHITECTURE.md) — scalability path
- [MONETIZATION.md](MONETIZATION.md) — tier definitions
