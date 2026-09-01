---
description: Azula project conventions — investigation pipeline, module boundaries, and AI output requirements
globs:
  - "**/*"
alwaysApply: true
---

# Azula — Project Rules

## Product Context

Azula is an AI workspace for data/ML teams. Core loop: **Generate → Investigate → Council → Evaluate**.

MVP focus: **Investigate + Council** with MCP file access.

Read product docs before implementing features:
- `docs/MVP.md` — scope and acceptance criteria
- `docs/ARCHITECTURE.md` — system design and data model
- `docs/PRD.md` — full requirements

## Module Boundaries (Critical)

Never bypass these abstractions:

```
GraphQL Resolvers
       ↓
InvestigationService  (orchestrates state machine)
       ↓
ModelRouter           (selects Fast / Deep / Judge strategy)
       ↓
LLMProvider           (OpenAI / Local / Custom)
       ↓
MCPConnector          (Files / Git / Database)
```

**Forbidden:**
- Calling LLM providers directly from resolvers
- Reading files with `fs` outside `MCPConnector`
- Skipping investigation state machine stages

## Investigation State Machine

Enforce this order in `InvestigationService`:

```
pending → fast_classify → deep_analyze → council → completed
```

- Auto-escalate to `deep_analyze` when fast confidence < 0.7
- Any state can transition to `failed` with error message
- Resolvers must not skip states

## AI Output Requirements

Every AI-generated claim must include:

1. **Confidence score** (0.0–1.0)
2. **Evidence link** (file name + line/excerpt) for root-cause claims
3. **Structured JSON** for Council output (see `docs/PRD.md` schema)

Council output must always include:
- `agreements[]`
- `disagreements[]`
- `finalJudgment` with `mostLikelyCause`, `confidence`, `recommendedAction`

## MongoDB Conventions

Collection names: PascalCase plural (`Users`, `Investigations`, `Projects`)

Required fields on all documents: `createdAt`, `updatedAt`

Investigation status enum: `pending | fast_classify | deep_analyze | council | completed | failed`

## Tier Limits

Check tier before gated actions. Config in `docs/MONETIZATION.md`.

```typescript
// Always check before:
createProject()        → maxProjects
startInvestigation()   → maxInvestigationsPerMonth
escalateToDeep()       → deepAnalysisEnabled
runCouncil()           → councilEnabled
```

## File Upload Security

- Max file size: 50MB
- Allowed extensions: `.log`, `.yaml`, `.yml`, `.py`, `.json`, `.jsonl`, `.csv`, `.txt`
- Store at: `{MCP_FILE_ROOT}/{projectId}/{filename}`
- No path traversal — validate all paths against project scope

## Code Style

- TypeScript for API and shared packages
- Match existing naming in each package
- No one-line helper abstractions unless reused 3+ times
- Comments only for non-obvious business logic

## Environment Variables

Never commit secrets. Required vars documented in `docs/MVP.md` and `docs/ARCHITECTURE.md`.

## When Adding Features

1. Check `docs/ROADMAP.md` — is this in current phase?
2. Check `docs/MVP.md` — is it in scope?
3. Update relevant doc if behavior changes
4. Follow investigation state machine
5. Add evidence links to all AI claims
