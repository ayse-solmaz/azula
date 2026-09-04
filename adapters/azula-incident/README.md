# azula-incident (Model B)

Local QLoRA merge used as Azula **Model B** (Deep / Council).

| Path | Git | What |
|------|-----|------|
| `Modelfile` | tracked | Ollama import recipe |
| `merged-fp16/` | ignored (~3 GB) | Hugging Face fp16 merge from Colab |
| `adapter/` | ignored | LoRA adapter only |

## Import into Ollama

1. Put the fp16 merge at `adapters/azula-incident/merged-fp16/` (`config.json` + `model.safetensors` + tokenizer files).
2. From this directory:

```powershell
ollama create azula-incident -f Modelfile
```

3. Azula `.env`:

```env
LLM_MODEL_B_PROVIDER=ollama
LLM_MODEL_B_NAME=azula-incident
OLLAMA_BASE_URL=http://localhost:11434
```

Ollama’s converter ignores Transformers 5 `rope_parameters`; `import-azula-incident.ps1` writes top-level `rope_theta`. The Modelfile uses the official Qwen2.5 chat template (`{{ .Messages }}` / `{{ .Response }}`). Without those, generation was `@` tokens.
