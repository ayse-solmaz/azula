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

# Ollama — Fast/Deep local models until a fine-tuned adapter is attached
ollama pull qwen2.5:1.5b
```

Fine-tune (QLoRA, ~15–25 min on Colab T4): open [notebooks/azula_qlora_colab.ipynb](notebooks/azula_qlora_colab.ipynb) via [Open in Colab](https://colab.research.google.com/github/ayse-solmaz/azula/blob/master/notebooks/azula_qlora_colab.ipynb). See [docs/COLAB_KAGGLE.md](docs/COLAB_KAGGLE.md).
