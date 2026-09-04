# Azula — Google Colab & Kaggle Fine-Tune Guide

Train QLoRA on **Qwen2.5-1.5B-Instruct** (open source) in the cloud, download adapter, run locally with Ollama as **Model B**.

---

## Option A: Google Colab (recommended)

### 1. New notebook

[Google Colab](https://colab.research.google.com) → New notebook → **Runtime → Change runtime type → T4 GPU**

### 2. Clone repo or upload files

**Cell 1 — Setup:**
```python
!git clone https://github.com/ayse-solmaz/azula.git
%cd azula
```

Or upload `services/trainer/` + `data/finetune/incident_pairs.jsonl` manually.

**Cell 2 — Install:**
```python
!pip install -q -r services/trainer/requirements.txt
```

**Cell 3 — Hugging Face login** (if model is gated — Qwen2.5 is open):
```python
# Optional — only if download fails
# from huggingface_hub import login
# login()
```

**Cell 4 — Train:**
```python
!python services/trainer/train.py \
  --base-model Qwen/Qwen2.5-1.5B-Instruct \
  --dataset data/finetune/incident_pairs.jsonl \
  --output /content/azula-finetune \
  --epochs 2 \
  --batch-size 4
```

Training time: **~15–25 min** on T4.

**Cell 5 — Download:**
```python
from google.colab import files
files.download('/content/azula-finetune/azula-adapter.zip')
```

### 3. Save to Google Drive (optional)

```python
from google.colab import drive
drive.mount('/content/drive')
!cp -r /content/azula-finetune /content/drive/MyDrive/azula-finetune
```

---

## Option B: Kaggle

### 1. New notebook

[Kaggle Notebooks](https://www.kaggle.com/code) → New notebook → **Settings → Accelerator → GPU T4 x2**

### 2. Add dataset

- Upload `incident_pairs.jsonl` as Kaggle dataset, or
- Add Azula repo via Git clone in notebook

**Enable internet:** Settings → Internet → On (required for Hugging Face)

### 3. Train

```python
!git clone https://github.com/ayse-solmaz/azula.git
%cd azula
!pip install -q -r services/trainer/requirements.txt

!python services/trainer/train.py \
  --dataset data/finetune/incident_pairs.jsonl \
  --output /kaggle/working/azula-finetune \
  --epochs 2
```

### 4. Download

Output appears in **Output** tab → `azula-adapter.zip`  
Or: Kaggle → Save Version → Save output as dataset

---

## Local: Load fine-tuned model (Ollama)

### 1. Extract zip

```bash
unzip azula-adapter.zip -d ./adapters/azula-incident
```

### 2. Create Ollama model from fp16 merged weights

4-bit `merged/` from training cannot be imported by Ollama. Use the **fp16** export (`azula-merged-fp16.zip`) at `adapters/azula-incident/merged-fp16/`. The repo already has `adapters/azula-incident/Modelfile` (official Qwen2.5 chat template). `import-azula-incident.ps1` patches `config.json` so Ollama sees `rope_theta` (Transformers 5 nested it; without that, Ollama emits `@` tokens).

```powershell
# extract zip so model.safetensors is under adapters/azula-incident/merged-fp16/
powershell -File scripts/import-azula-incident.ps1
```

```bash
ollama run azula-incident "Analyze: CUDA OOM at epoch 3, batch_size 128"
```

### 3. Point Azula Go API to Ollama

`.env`:
```env
LLM_MODEL_B_PROVIDER=ollama
LLM_MODEL_B_NAME=azula-incident
OLLAMA_BASE_URL=http://localhost:11434
```

---

## Expand training data

Add more rows to `data/finetune/incident_pairs.jsonl`:

```json
{"instruction": "...", "input": "...", "output": "..."}
```

Aim for **50–200 rows** for stronger demo. Use `samples/broken-pipeline/` as source.

---

## Troubleshooting

| Problem | Fix |
|---------|-----|
| `unrecognized arguments: --torch_dtype` | Drop that flag. `train.py` already uses fp16. Or use `!python` (not `accelerate launch`). |
| `_amp_foreach_non_finite_check_and_unscale_cuda` / `BFloat16` | Qwen is bf16; `fp16=True` GradScaler cannot unscale those grads. Re-run with the patched `train.py` (`fp16=False`). |
| CUDA OOM in Colab | `--batch-size 2`, reduce `--lora-r` to 8 |
| HF download slow | `HF_HUB_ENABLE_HF_TRANSFER=1` or use mirror |
| Kaggle internet off | Enable internet in notebook settings |
| Merge fails (RAM) | Use `adapter/` only; load with PEFT locally |
| Ollama import fails | Use merged folder; or convert with llama.cpp |

---

## Jury demo script

1. Show Colab notebook with training run (screenshot or live)
2. Show `azula-adapter.zip` downloaded
3. Local: `ollama run azula-incident` with sample log question
4. Azula dashboard: Model B = `azula-incident`
5. Compare generic model vs fine-tuned response

---

## Related

- [FINETUNE.md](FINETUNE.md) — architecture
- [DELIVERY_SPEC.md](DELIVERY_SPEC.md) — deadline checklist
- [services/trainer/train.py](../services/trainer/train.py) — training script
