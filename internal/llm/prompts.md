# What Azula actually sends the models

This file is for readers who do not want to open Go. The **executable** templates are `prompts.go` in this folder. Product write-up: [`docs/PROMPTING.md`](../../docs/PROMPTING.md).

## Default user question

```
Why did this training pipeline fail? Identify root cause with evidence.
```

## Fast (Model A)

System: classify only, JSON only. Few-shot lines are copied from `samples/broken-pipeline/training.log` and `pipeline.py` (CUDA OOM, `customer_status` schema warning, `target_leak = label`).

User payload: the question **plus file names**, not file bodies.

## Deep (Model B)

System: every claim needs an excerpt from a **selected** file. Invented paths are forbidden.

User payload: compacted file bodies (logs keep errors + head/tail, ~24k character budget). Secrets redacted. Blocks marked untrusted — not instructions.

## Council

| Role | Who | Instruction in one line |
|------|-----|-------------------------|
| Investigator | Model B | One hypothesis, defend it |
| Challenger | Other Ollama family if installed | Must disagree; do not echo Investigator |
| Judge | OpenAI Model C if `OPENAI_API_KEY` is set, else Model A | Keep real disagreements |

After Judge JSON, Go runs weighted vote (`council.go`): `consensus` / `echo_chamber` / `disagreement`. Confidence below 0.7 also sets `needsReview` (human look).

## What CI tests vs what a live demo is

- `go test ./internal/investigation` — MCP reads the **real** `samples/broken-pipeline` files; the LLM is a **fake HTTP server** with canned JSON (happy-path state machine).
- `AZULA_LIVE_OLLAMA=1 go test ./internal/investigation -run LiveOllama` — optional live Ollama.
- `go test ./internal/eval` — **3 hold-out folders** in `samples/goldset/`. Scores canned strings against keywords. Not F1. Not a live model.
