# Azula Investigation Skill

Use this skill when implementing, testing, or debugging the Azula investigation pipeline.

## When to Use

- Implementing `InvestigationService`
- Wiring Fast → Deep → Council flow
- Testing investigation with sample pipeline
- Debugging MCP file access during investigation
- Validating Council output schema

## Investigation Workflow

### Step 1: Load Context

```typescript
const context: InvestigationContext = {
  investigationId,
  projectId,
  prompt: investigation.prompt,
  files: await mcpConnector.listFiles(projectId),
};
```

Read files only through `MCPConnector` — never direct `fs` calls.

### Step 2: Fast Classification

```typescript
// Transition: pending → fast_classify
const fastResult = await modelRouter.classify(context);

// Expected output:
{
  summary: string,
  incidentType: string,  // e.g. "schema_mismatch", "memory_gpu"
  confidence: number     // 0.0–1.0
}
```

**Target latency:** < 5 seconds (p95)

**Auto-escalation:** If `confidence < 0.7`, proceed to Step 3 automatically.

### Step 3: Deep Analysis

```typescript
// Transition: fast_classify → deep_analyze
const deepResult = await modelRouter.analyze(context);

// Expected output:
{
  rootCause: string,
  confidence: number,
  evidence: [
    { file: string, lines: string, excerpt: string }
  ],
  suggestedFix: string
}
```

**Target latency:** < 30 seconds (p95)

MCP reads required files during this step. Log which files were accessed.

### Step 4: AI Council

```typescript
// Transition: deep_analyze → council
const councilResult = await modelRouter.runCouncil(context);

// Investigator and Challenger run in parallel
// Judge waits for both, then synthesizes
```

**Expected output schema:**

```json
{
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
    "simulation": null
  }
}
```

### Step 5: Complete

```typescript
// Transition: council → completed
await investigationService.complete(investigationId, {
  fastResult,
  deepResult,
  councilResult,
});
```

## Agent Roles

| Agent | Model | Role |
|-------|-------|------|
| Investigator | Deep | Builds and defends root-cause hypothesis |
| Challenger | Deep | Questions Investigator, proposes alternatives |
| Judge | Fast/Judge | Synthesizes agreements, disagreements, final judgment |

**Prompt principle:** Challenger must actively disagree or find weaknesses — not just produce a second opinion.

## MCP File Access in Dev

### Setup

```env
MCP_FILE_ROOT=./uploads
```

### Sample Pipeline Location

```
samples/broken-pipeline/
  training.log
  config.yaml
  pipeline.py
  dataset.jsonl
  metrics.json
```

Copy to `{MCP_FILE_ROOT}/{projectId}/` on project creation when `isSample: true`.

### Read Pattern

```typescript
const content = await mcpConnector.readFile(projectId, 'training.log');
// Search for OOM, schema warnings, accuracy decline
```

## Validation Checklist

Before marking investigation complete:

- [ ] Fast result has `incidentType` and `confidence`
- [ ] Deep result has at least 1 evidence entry per claim
- [ ] Council has both `agreements` and `disagreements` arrays
- [ ] `finalJudgment.confidence` is present
- [ ] All evidence `file` paths exist in project files
- [ ] State machine transitions are logged

## Error Handling

| Error | Action |
|-------|--------|
| LLM timeout | Retry once; then `failed` with message |
| Malformed JSON from model | Retry with stricter prompt; then `failed` |
| MCP file not found | Log warning; continue with available files |
| Tier limit exceeded | Return 403 with upgrade message (no investigation started) |

## Testing with Sample Pipeline

Expected root cause for `sample-broken-pipeline`:
- Primary: Schema drift in `customer_status`
- Secondary: Batch size causing OOM
- Expected fix: Fix schema encoding, reduce batch_size, remove leaky feature

Run end-to-end test:
1. Create workspace + sample project
2. `startInvestigation(projectId)`
3. Assert `completed` within 60 seconds
4. Assert `councilResult.finalJudgment.confidence > 0.8`

## Related Documents

- `docs/MVP.md` — user stories and acceptance criteria
- `docs/ARCHITECTURE.md` — state machine and data model
- `docs/PRD.md` — Council output schema
- `docs/ONBOARDING.md` — sample pipeline contents
- `.cursor/rules/azula.mdc` — project conventions
