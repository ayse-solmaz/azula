# Azula — Fine-Tune Pipeline (Open Source)

**Requirement:** Real LoRA/QLoRA fine-tune on an open-source model — not simulation.  
**Use case:** Domain-specific ML incident analysis after training on incident datasets.

---

## 1. Recommended open-source model

Pick **one** base model for the deadline (small = faster train, runs on modest GPU):

| Model | Size | Why |
|-------|------|-----|
| **Qwen2.5-1.5B-Instruct** | 1.5B | Fast QLoRA, good instruction following, Ollama support |
| **Llama-3.2-3B-Instruct** | 3B | Meta ecosystem, strong for code/logs |
| **Phi-3-mini-4k-instruct** | 3.8B | Microsoft, good reasoning |

**Default choice for Azula:** `Qwen2.5-1.5B-Instruct` — trains in ~15–30 min on 8GB GPU with QLoRA.

Download via Hugging Face (`huggingface-cli`) or Ollama pull for inference after merge.

---

## 2. Architecture

Fine-tuning runs in **Python** (PyTorch ecosystem). Go API orchestrates jobs.

```
┌─────────────┐     startFineTuneJob      ┌──────────────────┐
│  Go API     │ ────────────────────────▶ │  MongoDB         │
│  (GraphQL)  │                             │  FineTuneJobs    │
└──────┬──────┘                             └──────────────────┘
       │ spawn / HTTP
       ▼
┌──────────────────┐     saves adapter     ┌──────────────────┐
│  Python Trainer  │ ─────────────────────▶ │  ./adapters/     │
│  (PEFT + QLoRA)  │                         │  {jobId}/        │
└──────────────────┘                         └──────────────────┘
       │
       ▼
┌──────────────────┐
│  Ollama / vLLM   │  ← load base + LoRA for Model B inference
└──────────────────┘
```

Go does **not** train models directly — it triggers Python and serves the adapter path to the LLM router.

---

## 3. Training stack (Python)

```
services/trainer/
  requirements.txt
  train.py          # QLoRA entrypoint
  dataset.py        # JSONL → HF dataset
  config.yaml       # rank, alpha, epochs, lr
  Dockerfile        # optional: cuda image
```

**Dependencies:**

```
torch
transformers>=4.40
peft
bitsandbytes
datasets
trl
accelerate
```

**QLoRA defaults (demo-friendly):**

```yaml
base_model: Qwen/Qwen2.5-1.5B-Instruct
method: qlora
lora_r: 16
lora_alpha: 32
lora_dropout: 0.05
target_modules: [q_proj, v_proj, k_proj, o_proj]
epochs: 2
batch_size: 4
learning_rate: 2e-4
max_seq_length: 2048
```

---

## 4. Training dataset format

JSONL with instruction-style rows from Azula incident domain:

```jsonl
{"instruction": "Analyze this training log and identify the root cause.", "input": "ERROR CUDA out of memory...\nWARNING schema mismatch customer_status", "output": "Root cause: batch_size too large (128) causing OOM; secondary: schema drift in customer_status. Fix: reduce batch_size to 32, re-encode customer_status as categorical."}
{"instruction": "What config change fixes this pipeline failure?", "input": "batch_size: 128\nlearning_rate: 0.1", "output": "Reduce batch_size to 32 and learning_rate to 0.001."}
```

**Sources for dataset (what we actually train on):**

- `data/finetune/incident_pairs.jsonl` — instruction/input/output rows derived from the sample broken pipeline (OOM, schema drift, target leak, config). This is the **training** set.
- `samples/broken-pipeline/` — the live demo files those pairs were written from (not dumped raw into the trainer).
- `samples/goldset/` — **hold-out** incidents (`gpu-oom`, `schema-drift`, `target-leak`) with `expected.json`. Do not train on these folders.

**Evaluation (before/after a merge):**

| Metric | Where | What it measures |
|--------|--------|------------------|
| Keyword recall | `internal/eval.KeywordScore` | Fraction of gold `causeKeywords` / `fixKeywords` present in the model text |
| Type match | `TypeMatch` | Fast `incidentType` vs gold |
| Fast vs Council | `TestGoldSetCouncilBeatsFast` | Council judgment must beat a weak Fast baseline on the gold set |

There is **no** published F1 on a large public pipeline-failure benchmark yet. The gold set is the measurable bar for this repo: if Council keyword recall does not beat Fast, the fine-tune (or the prompts) is not helping investigation.

**Before/after protocol:** run the same `samples/goldset` cases with Model B = `qwen2.5:1.5b` vs `azula-incident` and compare `CouncilScore`. Keep the gold folders out of `incident_pairs.jsonl`.

Minimum for jury: **≥50 quality pairs** in `incident_pairs.jsonl`.

---

## 5. Job lifecycle

```go
// FineTuneJob document (MongoDB)
{
  id: "...",
  userId: "...",
  workspaceId: "...",
  baseModel: "Qwen/Qwen2.5-1.5B-Instruct",
  method: "qlora",
  datasetPath: "./data/finetune/incident_pairs.jsonl",
  status: "queued | training | merging | ready | failed",
  adapterPath: "./adapters/{jobId}",
  metrics: { loss: 0.42, epochs: 2 },
  createdAt, completedAt
}
```

**Statuses:**

1. `queued` — Go receives mutation, validates dataset
2. `training` — Python `train.py` running
3. `merging` — optional: merge LoRA into base for Ollama
4. `ready` — adapter available; `ModelConfigs.modelB.adapterId = jobId`
5. `failed` — log error in job document

---

## 6. Go → Python trigger

```go
// internal/finetune/runner.go
func (r *Runner) Start(ctx context.Context, job *FineTuneJob) error {
    cmd := exec.CommandContext(ctx, "python", "services/trainer/train.py",
        "--job-id", job.ID,
        "--base-model", job.BaseModel,
        "--dataset", job.DatasetPath,
        "--output", filepath.Join(r.adapterRoot, job.ID),
    )
    cmd.Env = append(os.Environ(), "CUDA_VISIBLE_DEVICES=0")
    return cmd.Start() // async; poll status or watch stdout
}
```

Or HTTP: Python trainer as sidecar on `:8091` with `POST /train`.

---

## 7. Inference after fine-tune

### Option A — Ollama (simplest for demo)

```bash
cd adapters/azula-incident
ollama create azula-incident -f Modelfile
```

See `adapters/azula-incident/README.md`. Go Model B uses `LLM_MODEL_B_NAME=azula-incident`.

### Option B — vLLM / llama.cpp with LoRA adapter

Load base + `adapter_path` at runtime — better for production, more setup.

**Deadline recommendation:** Ollama after merge.

---

## 8. LLM dashboard integration

User can:

| Control | Effect |
|---------|--------|
| Select base model | Ollama tags listed in dashboard inputs |
| Start QLoRA job | If `adapters/azula-incident/merged-fp16` exists → job `ready` and Model B attached; else runs `train.py` |
| Attach adapter to Model B | `attachIncidentModel` sets Model B to `azula-incident` |
| Ollama health | Dashboard shows reachable / adapter on disk / `azula-incident` loaded |
| Switch Model A ↔ B | Fast (A) vs Deep (fine-tuned local B) |

---

## Hardware requirements

| Setup | Train time (50 rows, 2 epochs) |
|-------|-------------------------------|
| **Google Colab T4** | ~15–25 min — **recommended** |
| **Kaggle T4** | ~15–25 min |
| NVIDIA 8GB local | ~15–30 min |
| CPU only | Not recommended |

**Primary workflow:** Train on Colab/Kaggle → download `azula-adapter.zip` → Ollama locally.  
See **[COLAB_KAGGLE.md](COLAB_KAGGLE.md)** for step-by-step cells.

---

## 10. Four-day fine-tune schedule

| When | Task |
|------|------|
| **Day 1 evening** | Create `incident_pairs.jsonl` (50+ rows from sample pipeline) |
| **Day 2** | `train.py` works locally; one successful QLoRA run |
| **Day 2** | Ollama load + Go Model B points to `azula-incident` |
| **Day 3** | GraphQL `startFineTuneJob` + dashboard job status UI |
| **Day 4** | Demo: upload JSONL → train → run investigation with fine-tuned model |

---

## 11. Jury talking points

- Base model: **open source** (Qwen/Llama), not proprietary API
- Method: **QLoRA** — efficient, enterprise-friendly (low GPU cost)
- Data: incident logs/config/code → instruction pairs
- Result: Model B understands **ML pipeline failures** better than generic model
- LLMOps: job tracking, adapter registry, switch in dashboard

---

## 12. Repo layout

```
azula/
  services/
    trainer/
      train.py
      requirements.txt
      config.yaml
  data/
    finetune/
      incident_pairs.jsonl    # seed dataset
  adapters/                   # gitignored, runtime output
  docs/FINETUNE.md            # this file
```

---

## Related

- [DELIVERY_SPEC.md](DELIVERY_SPEC.md) — fine-tune is Tier A (mandatory)
- [PROMPTING.md](PROMPTING.md) — templates, few-shot, token budget
- [samples/broken-pipeline/](../samples/broken-pipeline/) — source for training data
- [samples/goldset/](../samples/goldset/) — hold-out evaluation incidents
- `.env.example` — `FINETUNE_DEMO_MODE=false` for real training
