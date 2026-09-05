# Azula

**Generate. Investigate. Debate. Improve.**

Azula is an AI workspace for data and machine learning teams. It helps engineers automatically investigate pipeline failures, training crashes, and data quality incidents — then debate hypotheses across multiple models and recommend actionable fixes.

## What Azula Does

| Mode | Question | Outcome |
|------|----------|---------|
| **Investigate** | Why did my pipeline or model fail? | Root cause, confidence score, evidence, suggested fix |
| **Generate** | What training or eval data do I need? | Synthetic datasets (JSONL / CSV) |
| **Council** | Which hypothesis is correct? | Multi-model debate with agreements, disagreements, final judgment |
| **Evaluate** | Is the proposed fix actually better? | Metrics comparison and validation report |

## Why Council is not snake oil

Two model slots are not “twice as true.” Azula measures agreement **in code**, not in marketing:

- **Different families, same cause** (e.g. both name CUDA OOM from `batch_size`) → `consensus`, confidence up.
- **Same family, same wording** (Qwen Fast vs Qwen QLoRA restating OOM) → `echo_chamber`, review flag. That is *not* 95% independent certainty.
- **Different causes** (OOM vs memory leak / schema vs target leak) → `disagreement`, confidence down, `needsReview`.

We score this with **type-match + keyword recall** on `samples/goldset/` (three hold-out incidents) and on the composite demo `samples/broken-pipeline/expected.json`. That is recall of gold phrases, **not** a public F1 leaderboard.

## 6 Sep demo dataset

The jury walkthrough is **`samples/broken-pipeline/`** (onboarding sample: schema drift + GPU OOM + target leak in one project). It is synthetic, in-repo, and seeded when you create the sample workspace — not a customer log dump.

`samples/goldset/` (gpu-oom, schema-drift, target-leak) is the **hold-out scoring set**, split so each incident is primary. Do not train QLoRA on those folders.

Live customer logs are out of scope for the deadline unless you upload them in the UI after the sample run.

## Documentation

Start here:

1. **[Delivery Spec](docs/DELIVERY_SPEC.md)** — jury requirements + 4-day sprint (start here)
2. [Go Architecture](docs/ARCHITECTURE_GO.md) — Go + GraphQL + MongoDB layout
3. [MVP Scope](docs/MVP.md) — original MVP user stories
4. [Architecture](docs/ARCHITECTURE.md) — product architecture reference
5. [PRD](docs/PRD.md) — full product requirements (6 pillars)
6. [Roadmap](docs/ROADMAP.md) — MVP → Team → Enterprise

Supporting specs:

- [Onboarding](docs/ONBOARDING.md)
- [Monetization](docs/MONETIZATION.md)
- [Analytics](docs/ANALYTICS.md)
- [Fine-tune (Colab/Kaggle)](docs/COLAB_KAGGLE.md) — QLoRA training guide
- [Prompting](docs/PROMPTING.md) — what the models are asked, token budget, how tests relate to live demo

## Shipped vs deferred (2026-09-04)

**In the repo:** Investigate + Council, MCP files, GraphQL/Go API, web UI, MFA/trusted devices, GDPR export/delete, org invite, LLM dashboard, QLoRA trainer, Windows Electron pack.

**Not in git:** QLoRA weights (`adapters/azula-incident/merged-fp16`), packaged installers (`electron/dist`). Keep those local; the source and Modelfile are what belong on GitHub.

**Intentionally deferred / incomplete:** public F1 benchmark, Claude as Model C, 6–7B default Challenger, PDF export.

**Mac desktop:** `scripts/pack-electron.sh` on macOS, or the GitHub Action `desktop`. Windows cannot emit a `.dmg`.

## Sample Pipeline

Onboarding demo files: [`samples/broken-pipeline/`](samples/broken-pipeline/)

## Tech Stack (Delivery — deadline 6 Sep 2026)

```
Electron (Win/Mac) + Web (React)
            ↓ GraphQL
     Go API (gqlgen)
            ↓
    MongoDB + Worker Pool (5 concurrent)
            ↓
   LLM A (Fast) ↔ LLM B (Deep) — switchable, LoRA/QLoRA
            ↓
      MCP + File Version Swap
```

**Mandatory:** MFA, trusted devices, GDPR/KVKK, LLM dashboard, agent planning.  
See **[DELIVERY_SPEC.md](docs/DELIVERY_SPEC.md)** for the 4-day sprint plan.

## Remote

- Repository: https://github.com/ayse-solmaz/azula

## Agent Guidance

- Cursor rules: [.cursor/rules/azula.mdc](.cursor/rules/azula.mdc)
- Investigation skill: [.cursor/skills/azula-investigation/SKILL.md](.cursor/skills/azula-investigation/SKILL.md)

## Setup

```bash
cp .env.example .env

# MongoDB (Docker Desktop running)
docker run -d --name azula-mongo -p 27017:27017 --restart unless-stopped mongo:7

# API (Day 1 skeleton: auth, createProject, uploadFile)
go run ./cmd/api

# 5 concurrent startInvestigation calls (API + Mongo must already be running).
# Proves the 5-slot worker pool accepts 5 users; it does not record a UI demo.
go run ./scripts/loadtest.go

# Web (proxies /graphql → :8080)
cd web
npm install
npm run dev

# Electron (separate trusted device from the browser)
cd electron
npm install
npm start

# Windows installer (bundles the web UI; API still runs locally)
powershell -File scripts/pack-electron.ps1

# macOS unsigned DMG (must run on a Mac; same API localhost:8080 at runtime)
bash scripts/pack-electron.sh


# GraphQL playground: http://localhost:8080
# Web UI: http://localhost:3000

# Ollama — Fast (Qwen) + optional diverse Challenger (Mistral) + QLoRA Deep (azula-incident)
ollama pull qwen2.5:1.5b
ollama pull mistral
# After extracting the fp16 merge into adapters/azula-incident/merged-fp16:
powershell -File scripts/import-azula-incident.ps1
```

Weights in `adapters/azula-incident/merged-fp16/` are gitignored. The Ollama recipe is [`adapters/azula-incident/Modelfile`](adapters/azula-incident/Modelfile) (official Qwen2.5 chat template). Import patches `rope_theta` into `config.json` so Ollama does not emit `@` tokens. Default Model B name is `azula-incident` (see `.env.example`). Council Challenger uses a **different family** when `mistral` (or Llama/DeepSeek) is installed so debate is not Qwen-vs-Qwen. Optional Model C: set `OPENAI_API_KEY` for an API judge on disagreement. Prompts and token budgets: [`docs/PROMPTING.md`](docs/PROMPTING.md). Agent security (MCP, sessions, kill switch): [`docs/AGENTIC_SECURITY.md`](docs/AGENTIC_SECURITY.md).

Regenerate GraphQL after schema changes:

```bash
go run github.com/99designs/gqlgen generate
```

Fine-tune (QLoRA, ~15–25 min on Colab T4): open [notebooks/azula_qlora_colab.ipynb](notebooks/azula_qlora_colab.ipynb) via [Open in Colab](https://colab.research.google.com/github/ayse-solmaz/azula/blob/master/notebooks/azula_qlora_colab.ipynb). See [docs/COLAB_KAGGLE.md](docs/COLAB_KAGGLE.md).
