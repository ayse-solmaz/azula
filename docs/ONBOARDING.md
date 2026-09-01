# Azula — Onboarding Specification

**Goal:** Answer *"what does this product do?"* in 30 seconds through a live demo — not a feature tour.

---

## 1. Opening Experience

### First Screen

**Headline:**
> Your pipeline failed. Let's find out why.

**Subtext:**
> Connect your project. Azula investigates logs, code, and configs — then tells you what broke and how to fix it.

**Primary CTA:** `Create Workspace`  
**Secondary CTA:** `See how it works` (30-second video or animated demo)

---

## 2. Onboarding Flow

```
Step 1: Create Workspace
         ↓
Step 2: Connect Project (or use sample)
         ↓
Step 3: Run First Investigation
         ↓
Step 4: View Root Cause + Fix
         ↓
Step 5: Explore Analytics (optional nudge)
```

**Target completion time:** < 60 seconds from signup to first root cause.

---

## 3. Step-by-Step Screens

### Step 1 — Create Workspace

| Element | Content |
|---------|---------|
| Title | Name your workspace |
| Input | Workspace name (default: "My ML Lab") |
| CTA | Continue |
| Skip | Not available (required) |

### Step 2 — Connect Project

| Element | Content |
|---------|---------|
| Title | Connect your first project |
| Option A | **Try sample pipeline** (recommended, pre-selected) |
| Option B | Upload your own files |
| Sample description | "A broken ML training pipeline with logs, config, and code — ready to investigate." |
| CTA | Start Investigation |

**Default path:** Sample pipeline is pre-selected. User clicks one button.

### Step 3 — Investigation In Progress

Show real-time progress:

```
✓ Reading project files
✓ Fast classification (2.1s)
● Deep analysis in progress...
○ AI Council
○ Final report
```

**Do not** show a blank loading spinner. Show stage names and elapsed time.

### Step 4 — Results

Highlight the outcome immediately:

```
╭──────────────────────────────────────────────╮
│  ROOT CAUSE                                  │
│  Schema drift in `customer_status` field   │
│  Confidence: 91%                             │
├──────────────────────────────────────────────┤
│  SUGGESTED FIX                               │
│  Re-encode feature; reduce batch size to 32  │
├──────────────────────────────────────────────┤
│  EVIDENCE                                    │
│  📄 dataset.jsonl (lines 1204-1210)          │
│  📄 training.log (line 847)                  │
│  📄 config.yaml (batch_size: 128)            │
╰──────────────────────────────────────────────╯
```

**CTA:** `View full Council report`  
**Secondary CTA:** `Upload your own project`

### Step 5 — Analytics Nudge (Optional)

After first investigation:

> You've completed your first investigation in 38 seconds.  
> See how your incidents compare → [View Analytics]

---

## 4. Sample Pipeline Contents

Project name: `sample-broken-pipeline`

### training.log

Key entries to include:
- `WARNING: Column 'customer_status' has unseen categories`
- `CUDA out of memory. Tried to allocate 2.00 GiB`
- `Epoch 3/10 — val_accuracy: 0.61 (↓ from 0.74)`
- `Schema validation failed: expected int, got string`

### config.yaml

```yaml
model:
  name: classifier_v2
  batch_size: 128        # too large for GPU
  learning_rate: 0.1     # too high
  epochs: 10
data:
  train_path: dataset.jsonl
  features:
    - customer_status    # problematic feature
    - purchase_count
```

### pipeline.py

Include a subtle target leakage pattern:

```python
def engineer_features(df):
    df["target_leak"] = df["label"]  # leakage
    df["customer_status"] = df["customer_status"].astype(str)  # schema issue
    return df
```

### dataset.jsonl

Include rows where `customer_status` mixes int and string values.

### metrics.json

```json
{
  "train_accuracy": [0.72, 0.81, 0.89, 0.94],
  "val_accuracy": [0.74, 0.71, 0.65, 0.61],
  "notes": "Validation accuracy declining — possible overfitting or data issue"
}
```

---

## 5. Expected Demo Output

| Field | Expected value |
|-------|----------------|
| Incident type | Data quality + resource error |
| Root cause | Schema drift in `customer_status` |
| Secondary cause | Batch size causing OOM |
| Confidence | 85–95% |
| Suggested fix | Fix schema encoding, reduce batch_size to 32, remove leaky feature |
| Council agreement | Both models agree on data issues |
| Council disagreement | Drift vs leakage as primary cause |

---

## 6. Onboarding Metrics (Analytics)

Track these funnel events:

| Event | Description |
|-------|-------------|
| `onboarding_started` | User lands on welcome screen |
| `workspace_created` | Step 1 complete |
| `project_connected` | Step 2 complete (sample or upload) |
| `first_investigation_started` | Step 3 begins |
| `first_investigation_completed` | Step 4 — root cause shown |
| `onboarding_completed` | User views full report or uploads own project |

**Target conversion:** `onboarding_started` → `first_investigation_completed` > 70%

---

## 7. Copy Guidelines

- Use ML engineer language, not marketing fluff
- Lead with the problem (*pipeline failed*), not the product name
- Never say "AI-powered platform" in onboarding — show, don't tell
- Confidence scores and evidence are always visible
- Error states: *"Investigation failed — retry or upload different files"* (not generic 500)

---

## 8. Post-Onboarding

After completing onboarding, user lands on **Project Dashboard** with:

- Investigation history (1 entry: sample)
- Project files list
- CTA: `New Investigation`
- Banner: *"Upload your own project to investigate a real incident"*

---

## 9. Related Documents

- [MVP.md](MVP.md) — US-1 acceptance criteria
- [PRD.md](PRD.md) — Pillar 1: Onboarding
- [ANALYTICS.md](ANALYTICS.md) — funnel metrics
