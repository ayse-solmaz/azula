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

1. [MVP Scope](docs/MVP.md) — what ships first, user stories, acceptance criteria
2. [Architecture](docs/ARCHITECTURE.md) — system design, data model, module boundaries
3. [PRD](docs/PRD.md) — full product requirements (6 pillars)
4. [Roadmap](docs/ROADMAP.md) — MVP → Team → Enterprise

Supporting specs:

- [Onboarding](docs/ONBOARDING.md)
- [Monetization](docs/MONETIZATION.md)
- [Analytics](docs/ANALYTICS.md)
- [MCP Integration](docs/MCP.md)

## Sample Pipeline

Onboarding demo files: [`samples/broken-pipeline/`](samples/broken-pipeline/)

## Tech Stack (Planned)

```
Web Client
    ↓
GraphQL API
    ↓
MongoDB + AI Model Router (Fast / Deep / Judge)
    ↓
MCP Connectors (Files, Git, Database)
```

## Remote

- Repository: https://github.com/ayse-solmaz/azula

## Agent Guidance

- Cursor rules: [.cursor/rules/azula.mdc](.cursor/rules/azula.mdc)
- Investigation skill: [.cursor/skills/azula-investigation/SKILL.md](.cursor/skills/azula-investigation/SKILL.md)

## Setup

```bash
cp .env.example .env
# Edit .env with your API keys
```
