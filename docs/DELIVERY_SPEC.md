# Azula — Jury Delivery Spec

**Deadline:** 6 Eylül 2026, Pazar 01:00  
**Budget:** ~100 saat (~4 gün tam zamanlı)  
**Status:** Code for Tier A/B is in the repo. Checkboxes below match implementation, not the original empty sprint list.

---

## 1. Hard Requirements Checklist

| # | Requirement | Delivery strategy |
|---|-------------|-------------------|
| 1 | Backend **Go** + **GraphQL** | gqlgen layer on Go services |
| 2 | Database **MongoDB** (object/document) | Primary store for all entities |
| 3 | **Electron** desktop (Windows + Mac) | Wraps web app; same GraphQL client |
| 4 | **Web** client | React/Next.js or Vite + React |
| 5 | **Two LLMs** + switch between them | Fast + Deep; user-selectable in dashboard |
| 6 | **5 concurrent users** | Worker pool + LLM request queue + load-aware routing |
| 7 | **Agent LLM** + planning | Investigation agent with plan → execute → synthesize |
| 8 | Load-based **distribution** | Router assigns requests to least-loaded worker/model slot |
| 9 | **Security** (high) | JWT, bcrypt, rate limit, input sanitization, audit log |
| 10 | **MFA** | TOTP (Google Authenticator) on login |
| 11 | **Trusted devices** | Device fingerprint + register on first login; block unknown devices |
| 12 | **Ephemeral login verification** | Email OTP or time-limited session token for new devices |
| 13 | **User account deletion** | GDPR/KVKK right to erasure — wipe all user data |
| 14 | **Enterprise workspace** | Org/company creation, team isolation |
| 15 | **LLM manipulable** | Dashboard: model, temperature, system prompt, role config |
| 16 | **LLM fine-tune** (LoRA / QLoRA) | **Real** QLoRA on open-source model (Qwen2.5-1.5B or Llama-3.2-3B); Python trainer + Ollama inference |
| 17 | **LLM dashboard** | Model config, usage, switch, fine-tune status |
| 18 | **MCP** + file read | Filesystem MCP for logs, code, config |
| 19 | **Version swap** | File version history; compare/swap during investigation |
| 20 | **GDPR / KVKK** | Consent, data export, deletion, processing log |
| 21 | **Agent planning** | Visible plan steps before execution |
| 22 | **LLMOps** | Model registry, job tracking, metrics on dashboard |

---

## 2. Realistic Delivery Tiers (4 days)

Not everything can be production-grade. Jury demo uses **Tier A** live + **Tier B** stubbed + **Tier C** documented.

### Tier A — Must work live (demo)

- [x] Go GraphQL API (auth, workspace, project, investigation)
- [x] MongoDB persistence
- [x] Web UI: onboarding + investigation + Council result
- [x] Electron shell loading same web app (Windows pack on this machine; Mac DMG on macOS/CI)
- [x] Two LLM switch (Fast ↔ Deep) via dashboard
- [x] MCP file read from uploaded / sample pipeline
- [x] Agent planning UI (show plan steps, then execute)
- [x] 5-user concurrency test script (`scripts/loadtest.go`) — run against a live API; not a substitute for a recorded demo
- [x] MFA (TOTP) — register + verify on login
- [x] Trusted device registration on login
- [x] User delete account (cascade delete in MongoDB)
- [x] Sample pipeline end-to-end investigation
- [x] LoRA/QLoRA fine-tune — **real training** on open-source model; see [FINETUNE.md](FINETUNE.md)

### Tier B — Works with stub / minimal (show UI + one happy path)

- [x] Version swap — store 2 versions per file, swap in UI
- [x] Load-based routing — round-robin or queue depth metric (real, simple)
- [x] GDPR export — JSON dump of user data
- [x] Enterprise org creation — org + email invite + **admin / engineer / viewer** RBAC on mutations
- [x] KVKK consent banner + processing log table

### Tier C — Document + architecture slide only (intentionally not built)

- [x] Horizontal scale starter (K8s manifests in `deploy/k8s` — not multi-region)
- [ ] Production LoRA training pipeline (GPU cluster)
- [x] Full RBAC matrix (admin/engineer/viewer)
- [x] SSO / OIDC (SAML not included)

---

## 3. Updated Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Clients                                                    │
│  ├── Web (React)                                            │
│  └── Electron (Windows / macOS) → loads Web                 │
└──────────────────────────┬──────────────────────────────────┘
                           │ GraphQL (gqlgen)
┌──────────────────────────▼──────────────────────────────────┐
│  Go API Server                                              │
│  ├── Auth (JWT + MFA TOTP + trusted devices)                │
│  ├── Workspace / Org / Project                              │
│  ├── Investigation Service (agent planning + state machine) │
│  ├── LLM Router (2 models, load-aware, switchable)           │
│  ├── MCP Connector (files, version store)                   │
│  ├── Fine-tune Service (LoRA/QLoRA job queue)               │
│  └── GDPR Service (export, delete, consent log)             │
└──────────┬────────────────────────────┬─────────────────────┘
           │                            │
    ┌──────▼──────┐              ┌──────▼──────┐
    │  MongoDB    │              │ Worker Pool │
    │  (objects)  │              │ (5 slots)   │
    └─────────────┘              └──────┬──────┘
                                        │
                              ┌─────────▼─────────┐
                              │ LLM Providers     │
                              │ ├── Model A Fast  │
                              │ └── Model B Deep  │
                              │ (+ LoRA adapter)  │
                              └───────────────────┘
```

**masterfabric-go:** Use selectively for patterns (hexagonal layout, JWT, audit) — **MongoDB replaces PostgreSQL** in Azula fork. Do not block on full masterfabric integration if time is short; copy auth/audit patterns.

---

## 4. MongoDB Collections

```
Users              — auth, MFA secret, trustedDevices[]
Organizations      — enterprise workspace
Workspaces         — belongs to org or user
Projects           — files[], versions[]
Investigations     — status, plan[], results, council
Messages           — agent conversation
ModelConfigs       — per-workspace LLM settings (manipulable)
FineTuneJobs       — LoRA/QLoRA job status
FileVersions       — version swap history
AuditLogs          — security + GDPR processing log
ConsentRecords     — KVKK/GDPR consent
```

---

## 5. Two-LLM Switch + 5-User Scale

### Model roles

| Slot | Default role | User can switch |
|------|--------------|-----------------|
| **Model A** | Fast — classify, plan | Yes (dashboard) |
| **Model B** | Deep — analyze, Council | Yes (dashboard) |

### Load distribution (MVP implementation)

```go
type LLMRouter struct {
    workers   [5]WorkerSlot  // max 5 concurrent investigations
    modelA    LLMProvider
    modelB    LLMProvider
}

// Route: pick worker with lowest queue depth
// Route model: user config or auto (fast first, deep on low confidence)
```

### Concurrency proof for jury

- `scripts/loadtest.go` — 5 parallel `startInvestigation` mutations
- All complete without timeout; p95 < 60s with API models

---

## 6. Security Stack

| Feature | Implementation |
|---------|----------------|
| Password | bcrypt |
| Session | JWT (short-lived) + refresh token |
| MFA | TOTP (`github.com/pquerna/otp`) |
| Trusted device | Device ID hash in cookie + `Users.trustedDevices[]` |
| New device | Email OTP or MFA required before trust |
| Rate limit | Per-IP + per-user (Redis or in-memory for demo) |
| Audit | All auth, investigation, delete events → `AuditLogs` |
| Account delete | Mutation `deleteAccount` → cascade all collections |
| GDPR export | Mutation `exportMyData` → ZIP/JSON |

---

## 7. LLM Dashboard (manipulable)

User can configure per workspace:

- Model A / Model B provider + model name (OpenAI, Ollama, custom endpoint)
- Temperature, max tokens
- System prompts per agent role (Investigator, Challenger, Judge)
- Switch active model for next investigation
- Fine-tune job: upload dataset → start LoRA/QLoRA → attach adapter to Model B

**Demo path:** Change temperature → re-run investigation → different confidence shown.

---

## 8. Agent Planning (visible to user)

```
PLAN (shown before execution)
  1. Read training.log for errors
  2. Read config.yaml for misconfiguration
  3. Read pipeline.py for code bugs
  4. Cross-check dataset.jsonl schema
  5. Run Council with findings
     ↓
EXECUTE (step-by-step progress in UI)
     ↓
RESULT
```

Store `plan[]` on Investigation document; UI checks off each step.

---

## 9. MCP + Version Swap

### MCP (MVP)

- Read files from `{uploadRoot}/{projectId}/`
- Agent tools: `read_file`, `list_files`, `search_in_file`

### Version swap

- On upload: save as `FileVersions` v1, v2, ...
- UI: dropdown "compare v1 ↔ v2" or "use v1 for investigation"
- Investigation links `fileVersionIds[]` in evidence

---

## 10. Fine-tune (LoRA / QLoRA)

### Demo implementation (4 days)

1. UI: upload JSONL dataset, select base model, start job
2. Backend: `FineTuneJobs` document `status: queued → training → ready`
3. For demo: pre-baked adapter or simulate 30s training
4. Attach `adapterId` to `ModelConfigs.modelB` for next investigation

### Jury narrative

> "Enterprise customers fine-tune with LoRA/QLoRA on their incident data; adapter loads into Model B for domain-specific root cause analysis."

---

## 11. GDPR / KVKK

| Requirement | Implementation |
|-------------|----------------|
| Consent on signup | Checkbox + `ConsentRecords` |
| Processing log | `AuditLogs` with `type: data_processing` |
| Right to access | `exportMyData` |
| Right to erasure | `deleteAccount` |
| LLM data handling | Prompt: no PII in logs; optional redaction middleware |
| KVKK | Turkish consent text; data residency note in docs |

---

## 12. Electron (Windows + Mac)

```
electron/
  main.js       — BrowserWindow → bundled web/ or localhost:3000
  preload.js    — device id + GraphQL URL
  package.json  — electron-builder win (nsis) + mac (dmg, unsigned)
```

| Platform | How to build | Where the artifact comes from |
|----------|----------------|-------------------------------|
| Windows | `powershell -File scripts/pack-electron.ps1` | `electron/dist` (gitignored). Unpacked exe is local-only. |
| macOS | `bash scripts/pack-electron.sh` on a Mac | Unsigned `.dmg`. **Cannot be produced on Windows.** |
| Mac without a Mac | GitHub Actions workflow `desktop` (`macos-latest`) | Download the Actions artifact. Still unsigned (no Apple Developer ID). |

Dev: `localhost:3000` (API must be running).  
Packaged: static files under `electron/web/` (copied from `web/dist`, gitignored). API is still local `localhost:8080`.

---

## 13. Four-Day Sprint

### Day 1 (24h) — Skeleton

- Go monorepo: `cmd/api`, `internal/`, GraphQL schema
- MongoDB connection + User, Project, Investigation models
- Auth: register, login, JWT
- GraphQL: `createProject`, `startInvestigation` (mock response)
- Web: login + project list shell
- Electron: empty window loading web

### Day 2 (24h) — AI + Agent

- LLM Router: Model A + Model B, switch config
- MCP file reader
- Investigation state machine + agent planning steps
- Wire real LLM calls (OpenAI or Ollama)
- Council output (Investigator + Challenger + Judge)
- Worker pool (5 slots)

### Day 3 (24h) — Security + Dashboard

- MFA TOTP enroll + verify
- Trusted devices
- LLM dashboard (model switch, temperature, prompts)
- Version swap (2 versions per file)
- Fine-tune job UI (stub pipeline)
- GDPR: export + delete account
- Analytics / LLMOps dashboard basics

### Day 4 (16h) — Polish + Demo

- Onboarding + sample pipeline
- Electron build (Windows pack script; Mac on macOS or CI)
- Load test 5 users (`go run ./scripts/loadtest.go` with API + Mongo up)
- MFA + trusted device demo script
- Slide deck from docs
- Backup demo video
- Bug fixes

---

## 14. Jury Demo Script (3 minutes)

1. **0:00** — "Your pipeline failed. Let's find out why." → sample project
2. **0:20** — Agent plan appears (5 steps) → executes live
3. **0:45** — Council: Model A vs Model B, agreement/disagreement
4. **1:15** — LLM dashboard: switch model, change temperature, re-run
5. **1:45** — Security: MFA login, trusted device prompt
6. **2:15** — Version swap: compare config v1 vs v2
7. **2:30** — Fine-tune: start LoRA job (demo completes)
8. **2:45** — GDPR: export my data / delete account mention
9. **3:00** — Architecture: Go + GraphQL + MongoDB + 5-user scale

---

## 15. What to tell jury when asked

| Question | Answer |
|----------|--------|
| Why MongoDB? | Investigation objects, flexible evidence/council JSON, version history |
| Why two LLMs? | Fast triage + Deep analysis; user switches per workload |
| Scale? | 5 concurrent workers; queue depth routing; tested with load script |
| Fine-tune? | LoRA/QLoRA job pipeline; demo adapter on domain incident data |
| Security? | MFA, trusted devices, audit logs, GDPR/KVKK delete + export |
| Electron? | Same web UI in a desktop shell. Windows installer from `pack-electron.ps1`. Mac DMG from a Mac or the `desktop` GitHub Action — not from a Windows machine. |

---

## Related docs

- [MVP.md](MVP.md) — original scope (superseded by this doc for deadline)
- [ARCHITECTURE.md](ARCHITECTURE.md) — update in progress for Go+Mongo+Electron
- [PRD.md](PRD.md) — full product vision
- [MONETIZATION.md](MONETIZATION.md) — enterprise features map to Tier A/B above
