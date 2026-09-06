# Sample: Missing-value dropna (single root cause)

Upload this folder when you want a **clear, single-cause** case — unlike the composite onboarding demo in `samples/broken-pipeline/`.

**Expected root cause:** `dropna` on `monthly_spend` NaNs removes missing-not-at-random rows (mostly churners), flips class balance, and collapses validation AUC to chance.

There is **no** CUDA OOM, schema-mixed `customer_status`, or `target_leak` in this folder. Models should name this data-quality failure as **primary** and not blend in unrelated demo bugs.

| File | What it shows |
|------|----------------|
| `training.log` | NaN warning, row drop counts, class-balance flip, `val_auc` → ~0.50 |
| `config.yaml` | Intended `monthly_spend: median` impute; modest `batch_size` (not OOM) |
| `pipeline.py` | `df.dropna(subset=["monthly_spend"])` ignores the configured impute |
| `dataset.jsonl` | `monthly_spend: null` rows are almost all `label: 1` |

Scoring keywords: [`expected.json`](expected.json). Hold-out twin: [`samples/goldset/nan-impute/`](../goldset/nan-impute/).

## How to run in the UI

1. Create a **new** project (do not reuse `sample-broken-pipeline`).
2. Upload `training.log`, `config.yaml`, `pipeline.py`, and `dataset.jsonl` (allowed types: `.log`, `.yaml`, `.py`, `.jsonl`).
3. Start an investigation.

Council `finalJudgment.mostLikelyCause` / `recommendedAction` should hit: `dropna`, `monthly_spend`, `nan`, `balance`, `auc`, and a fix that **imputes** (median) instead of dropping rows.
