# Azula — Prompting and context budget

Investigation LLM calls go through `ModelRouter` (`internal/llm`). Resolvers never send prompts to a provider.

Executable strings: [`internal/llm/prompts.go`](../internal/llm/prompts.go). Short reader copy: [`internal/llm/prompts.md`](../internal/llm/prompts.md).

## Few-shot from the sample logs (not invented)

These lines are in `samples/broken-pipeline/` and `samples/broken-nan-impute/`. Fast classification few-shot quotes them:

```
WARNING Column 'customer_status' has unseen categories: ['premium', '2', 'active_new']
WARNING Schema validation failed: expected int, got string at row 1204
ERROR CUDA out of memory. Tried to allocate 2.00 GiB (GPU 0; 8.00 GiB total capacity)
```

```python
df["target_leak"] = df["label"]
df = df.dropna(subset=["monthly_spend"])
```

```
WARNING Column 'monthly_spend' has 3,108 NaN values (25.1%)
```

| Log / code | Fast `incidentType` |
|------------|---------------------|
| CUDA OOM + `batch_size: 128` | `memory_gpu` |
| Unseen `customer_status` categories | `schema_mismatch` |
| `target_leak = label` | `data_leakage` |
| `learning_rate: 0.1` + val accuracy drop | `config_error` |
| `dropna` on `monthly_spend` NaNs + class-balance flip / `val_auc` collapse | `data_quality` |

The onboarding project is **composite**: schema, leak, OOM, and config signals exist in one folder. Council is expected to **disagree** on the *primary* cause (schema vs leak vs OOM), then flag `needsReview` rather than fake a 95% consensus.

`samples/broken-nan-impute/` is a **single-cause** hold-out twin: dropna on `monthly_spend` NaNs flips class balance and kills val AUC. Fast/Deep/Judge must rank that as **primary** and must not blend in OOM / leak / schema when those signals are absent.

## Token budget

| Stage | What is sent | Budget |
|-------|----------------|--------|
| Fast | User question + **file names only** | Tiny (no file bodies) |
| Deep | Ranked MCP files, compacted | ~24k characters (~6k tokens at 4 chars/token) |
| Council Investigator | Prior Fast/Deep brief + **cited** file snippets only | ~2.5k snippet budget — no full Deep re-read |
| Council Challenger | Prior brief + compact files | ~8k characters (`AZULA_COUNCIL_CONTEXT_CHARS`) |
| Per file | Cap 8k chars (Deep) | Logs: hierarchical extract, not a blind prefix |

**Logs:** keep ERROR/WARNING/OOM/traceback lines plus the first and last 40 lines. Omitted regions are marked `...`. Secrets (keys, JWTs, private key blocks) are redacted. Packed files are wrapped as **untrusted retrieved data**.

## Templates

Workspace overrides live on `ModelConfig` (LLM dashboard Advanced).

### Fast / Deep / Council

See `SysFast`, `SysDeep`, `SysInvestigator`, `SysChallenger`, `SysJudge` in `prompts.go`.

Shared rules in those templates:

1. **Rank primary vs secondary.** Name one primary cause. Mention a secondary only with file evidence.
2. **file:line evidence.** Claims need a real path, a line range, and an excerpt copied from the file.
3. **Do not blend.** If one data-quality failure dominates, do not concatenate schema/OOM/leak into `mostLikelyCause`.
4. **Challenger is not forced to invent a second bug.** On a single-cause folder it may agree on the primary; on the composite demo it should still pick a different primary when several independent failures are in the files.

`AZULA_COUNCIL_FAST=true` (default) keeps Challenger on the small Fast model so a single Ollama GPU does not swap in a 7B+ diverse family during Council. Set `false` to restore family-diversity routing. Investigator reuses the Deep brief plus cited snippets (no second full-file pass). Challenger gets a compact ~8k pack. Both start in parallel; each agent has a 25s timeout (`AZULA_COUNCIL_AGENT_TIMEOUT`) and a 512-token cap. Partial hypotheses are persisted so the UI can render them before the Judge finishes.

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
| `TestNanImputeFilesRankAndPack` | No | `samples/broken-nan-impute` — packing keeps dropna / monthly_spend, not composite signals |
| `TestGoldSetCouncilBeatsFast` | No | **4** hold-out incidents in `samples/goldset/` — keyword recall on **canned** answers |
| `TestLiveOllamaAzulaIncident` | Yes, if `AZULA_LIVE_OLLAMA=1` | broken-pipeline against local Ollama |

Four gold folders are **not** an F1 benchmark. They catch “Council text names the cause keywords; a vague Fast summary does not.”

## User question

Default (when the UI sends none):

> Why did this training pipeline fail? Identify root cause with evidence.
