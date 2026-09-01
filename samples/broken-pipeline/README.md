# Sample: Broken ML Pipeline

Pre-loaded project for Azula onboarding demo.

**Expected root cause:** Schema drift in `customer_status` + batch size causing GPU OOM.

| File | Issue |
|------|-------|
| `training.log` | OOM error, schema warnings, declining val accuracy |
| `config.yaml` | `batch_size: 128` too large, `learning_rate: 0.1` too high |
| `pipeline.py` | Target leakage (`target_leak`) + forced string cast on `customer_status` |
| `dataset.jsonl` | Mixed int/string values in `customer_status` |
| `metrics.json` | Train accuracy up, val accuracy down |

Used by onboarding flow — see [docs/ONBOARDING.md](../docs/ONBOARDING.md).
