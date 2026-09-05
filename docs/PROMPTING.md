# Azula — Prompting and context budget

Investigation LLM calls go through `ModelRouter` (`internal/llm`). Resolvers never send prompts to a provider.

Executable strings: [`internal/llm/prompts.go`](../internal/llm/prompts.go). Short reader copy: [`internal/llm/prompts.md`](../internal/llm/prompts.md).

## Few-shot from the demo log (not invented)

These lines are in `samples/broken-pipeline/training.log` and `pipeline.py`. Fast classification few-shot quotes them:

```
WARNING Column 'customer_status' has unseen categories: ['premium', '2', 'active_new']
WARNING Schema validation failed: expected int, got string at row 1204
ERROR CUDA out of memory. Tried to allocate 2.00 GiB (GPU 0; 8.00 GiB total capacity)
```

```python
df["target_leak"] = df["label"]
```

| Log / code | Fast `incidentType` |
|------------|---------------------|
| CUDA OOM + `batch_size: 128` | `memory_gpu` |
| Unseen `customer_status` categories | `schema_mismatch` |
| `target_leak = label` | `data_leakage` |
| `learning_rate: 0.1` + val accuracy drop | `config_error` |

The onboarding project is **composite**: all four signals exist in one folder. Council is expected to **disagree** on the *primary* cause (schema vs leak vs OOM), then flag `needsReview` rather than fake a 95% consensus.

## Token budget

| Stage | What is sent | Budget |
|-------|----------------|--------|
| Fast | User question + **file names only** | Tiny (no file bodies) |
| Deep / Council | Ranked MCP files, compacted | ~24k characters (~6k tokens at 4 chars/token) |
| Per file | Cap 8k chars | Logs: hierarchical extract, not a blind prefix |

**Logs:** keep ERROR/WARNING/OOM/traceback lines plus the first and last 40 lines. Omitted regions are marked `...`. Secrets (keys, JWTs, private key blocks) are redacted. Packed files are wrapped as **untrusted retrieved data**.

## Templates

Workspace overrides live on `ModelConfig` (LLM dashboard Advanced).

### Fast / Deep / Council

See `SysFast`, `SysDeep`, `SysInvestigator`, `SysChallenger`, `SysJudge` in `prompts.go`.

After the Judge JSON returns, **Go applies weighted voting** (`internal/llm/council.go`):

1. Score = confidence × (1 + 0.12 × min(evidence count, 4))
2. Same-family + similar hypotheses → `echo_chamber`, `needsReview`
3. Different families + similar hypotheses → `consensus`, confidence boosted
4. Divergent hypotheses → `disagreement`, confidence damped, `needsReview`
5. Final confidence **< 0.7** → `needsReview` (human look before closing)

## What “tests passed” means

| Test | Live LLM? | Data |
|------|-----------|------|
| `TestPipelineMCPSampleAndCouncil` | No — mocked Ollama JSON | Real files from `samples/broken-pipeline` |
| `TestBrokenPipelineFilesRankAndPack` | No | Same folder: packing must keep OOM, schema, leak |
| `TestGoldSetCouncilBeatsFast` | No | **3** hold-out incidents in `samples/goldset/` — keyword recall on **canned** answers |
| `TestLiveOllamaAzulaIncident` | Yes, if `AZULA_LIVE_OLLAMA=1` | broken-pipeline against local Ollama |

Three gold folders are **not** an F1 benchmark. They catch “Council text names the cause keywords; a vague Fast summary does not.”

## User question

Default (when the UI sends none):

> Why did this training pipeline fail? Identify root cause with evidence.
