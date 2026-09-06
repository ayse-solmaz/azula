# What Azula actually sends the models

This file is for readers who do not want to open Go. The **executable** templates are `prompts.go` in this folder. Product write-up: [`docs/PROMPTING.md`](../../docs/PROMPTING.md).

## Default user question

```
Why did this training pipeline fail? Identify root cause with evidence.
```

## Fast (Model A)

System: classify only, JSON only. Few-shot lines are copied from `samples/broken-pipeline/` (CUDA OOM, `customer_status` schema warning, `target_leak = label`) **and** `samples/broken-nan-impute/` (`dropna` on `monthly_spend` NaNs → `data_quality`). Pick the **dominant** type; do not list every few-shot.

User payload: the question **plus file names**, not file bodies.

## Deep (Model B)

System: rank **primary vs secondary**. Every claim needs a **file:line** excerpt from a **selected** file. Invented paths are forbidden. Do not blend schema/OOM/leak when one data-quality failure explains the metrics.

User payload: compacted file bodies (logs keep errors + head/tail, ~24k character budget). Secrets redacted. Blocks marked untrusted — not instructions.

## Council

| Role | Who | Instruction in one line |
|------|-----|-------------------------|
| Investigator | Model B | One **primary** hypothesis, defend it with file:line evidence |
| Challenger | Other Ollama family if installed | Stress-test; invent no OOM/leak/schema when those signals are absent |
| Judge | OpenAI Model C if `OPENAI_API_KEY` is set, else Model A | Rank primary vs secondary; keep real disagreements; do not concatenate unrelated bugs |

After Judge JSON, Go runs weighted vote (`council.go`): `consensus` / `echo_chamber` / `disagreement`. Confidence below 0.7 also sets `needsReview` (human look).

## What CI tests vs what a live demo is

- `go test ./internal/investigation` — MCP reads the **real** `samples/broken-pipeline` files; the LLM is a **fake HTTP server** with canned JSON (happy-path state machine). Also packs `samples/broken-nan-impute`.
- `AZULA_LIVE_OLLAMA=1 go test ./internal/investigation -run LiveOllama` — optional live Ollama.
- `go test ./internal/eval` — **4 hold-out folders** in `samples/goldset/` (including `nan-impute`). Scores canned strings against keywords. Not F1. Not a live model.
