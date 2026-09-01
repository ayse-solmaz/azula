# Azula — Product Roadmap

**Last updated:** 2026-09-01

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

- [ ] Auth (email/password)
- [ ] Workspace + Project CRUD
- [ ] File upload (MCP Files connector)
- [ ] Onboarding with sample broken pipeline
- [ ] Investigate mode: Fast → Deep → Council
- [ ] Investigation results UI (root cause, evidence, fix)
- [ ] Investigation history
- [ ] Basic analytics dashboard
- [ ] MongoDB persistence
- [ ] GraphQL API
- [ ] Free tier limits (config-enforced)
- [ ] Cursor rules + investigation skill

### Success Criteria

- Demo pipeline: signup → root cause < 60s
- Council structured output > 90%
- 5 concurrent users without degradation

### Not in MVP

- Payment integration
- Generate / Evaluate modes
- Git MCP
- Team features
- Enterprise security

---

## Phase 2: MVP+1

**Timeline:** Weeks 7–10  
**Goal:** Complete the 4-mode product loop

### Deliverables

- [ ] **Generate mode** — synthetic dataset from investigation context
- [ ] **Evaluate mode** — compare original vs fixed dataset metrics
- [ ] **Git MCP connector** — clone, blame, diff
- [ ] Version diff UI (compare file versions across investigations)
- [ ] Pro tier UI gates (upgrade prompts)
- [ ] Model selection (choose Fast/Deep/Judge models)
- [ ] Full analytics dashboard (Council metrics, model comparison)
- [ ] Investigation export (PDF/JSON)

### Success Criteria

- End-to-end flow: Investigate → Generate fix dataset → Evaluate → Council
- Git MCP reads repo files for investigation
- Pro upgrade prompts shown at correct gate points

---

## Phase 3: Team

**Timeline:** Weeks 11–16  
**Goal:** Multi-user collaboration

### Deliverables

- [ ] Team workspaces (invite members)
- [ ] Role-based access: Admin, Engineer, Viewer
- [ ] Shared investigation history
- [ ] Payment integration (Stripe)
- [ ] Pro tier activation
- [ ] Email notifications (investigation complete, limit warnings)
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

- [ ] MFA + trusted devices
- [ ] Audit logs (all actions)
- [ ] GDPR / KVKK: data export, deletion, residency options
- [ ] API access with scoped tokens
- [ ] Custom / LoRA model support
- [ ] Private MCP connectors (internal repos, databases)
- [ ] Custom Council lineup (2–8 models)
- [ ] Higher concurrent user limits
- [ ] SSO (SAML/OIDC) — if required by early enterprise customers
- [ ] Electron desktop app (optional)
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
- [ ] Worker pool behind load balancer
- [ ] Response caching
- [ ] Multi-provider model routing (OpenAI, Anthropic, local, custom)
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
| Job queue | | | | | ✓ |
| Multi-provider | | | | | ✓ |

---

## Risk Register

| Risk | Impact | Mitigation |
|------|--------|------------|
| LLM latency too high for demo | High | Use fast model for demo; pre-cache sample pipeline results |
| Council output inconsistent | High | Strict JSON schema validation; retry on malformed output |
| MCP file access security | High | Path traversal protection; project-scoped directories |
| Free tier abuse | Medium | Rate limiting; investigation cap enforcement |
| Enterprise sales cycle long | Medium | Ship Team phase before full Enterprise; land Pro first |

---

## Related Documents

- [MVP.md](MVP.md) — Phase 1 detailed spec
- [PRD.md](PRD.md) — full product requirements
- [ARCHITECTURE.md](ARCHITECTURE.md) — scalability path
- [MONETIZATION.md](MONETIZATION.md) — tier definitions
