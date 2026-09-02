#!/usr/bin/env python3
"""
Azula QLoRA fine-tune — run on Google Colab or Kaggle.

Colab:  Upload this folder + data/finetune/incident_pairs.jsonl
Kaggle: Add dataset as input, set paths below

After training: downloads azula-adapter.zip → use with Ollama locally.
"""

from __future__ import annotations

import argparse
import json
import os
import shutil
import zipfile
from pathlib import Path

import torch
from datasets import Dataset
from peft import LoraConfig, get_peft_model, prepare_model_for_kbit_training
from transformers import (
    AutoModelForCausalLM,
    AutoTokenizer,
    BitsAndBytesConfig,
    TrainingArguments,
)
from trl import SFTTrainer


DEFAULT_BASE = "Qwen/Qwen2.5-1.5B-Instruct"


def _make_sft_trainer(model, tokenizer, dataset, training_args):
    """SFTTrainer kwargs differ across TRL versions (Colab vs local)."""
    base = {"model": model, "args": training_args, "train_dataset": dataset}
    variants = [
        {"processing_class": tokenizer, "dataset_text_field": "text"},
        {"tokenizer": tokenizer, "dataset_text_field": "text", "max_seq_length": 1024},
        {"processing_class": tokenizer},
        {"tokenizer": tokenizer, "dataset_text_field": "text"},
    ]
    errors: list[str] = []
    for extra in variants:
        try:
            return SFTTrainer(**base, **extra)
        except TypeError as exc:
            errors.append(str(exc))
    raise TypeError("SFTTrainer API mismatch:\n" + "\n".join(errors))


def load_jsonl(path: str) -> Dataset:
    rows = []
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line:
                rows.append(json.loads(line))
    return Dataset.from_list(rows)


def format_prompt(example: dict) -> dict:
    text = (
        f"<|im_start|>system\n"
        f"You are Azula, an ML incident investigator. Analyze pipeline failures with evidence and suggest fixes.\n"
        f"<|im_start|>user\n"
        f"{example['instruction']}\n\n"
        f"Context:\n{example['input']}\n"
        f"<|im_start|>assistant\n"
        f"{example['output']}"
    )
    return {"text": text}


def train(
    base_model: str,
    dataset_path: str,
    output_dir: str,
    epochs: int = 2,
    batch_size: int = 4,
    lora_r: int = 16,
) -> None:
    out = Path(output_dir)
    out.mkdir(parents=True, exist_ok=True)

    bnb_config = BitsAndBytesConfig(
        load_in_4bit=True,
        bnb_4bit_quant_type="nf4",
        bnb_4bit_compute_dtype=torch.float16,
        bnb_4bit_use_double_quant=True,
    )

    tokenizer = AutoTokenizer.from_pretrained(base_model, trust_remote_code=True)
    tokenizer.pad_token = tokenizer.eos_token

    model = AutoModelForCausalLM.from_pretrained(
        base_model,
        quantization_config=bnb_config,
        device_map="auto",
        trust_remote_code=True,
    )
    model = prepare_model_for_kbit_training(model)

    lora_config = LoraConfig(
        r=lora_r,
        lora_alpha=lora_r * 2,
        lora_dropout=0.05,
        bias="none",
        task_type="CAUSAL_LM",
        target_modules=["q_proj", "k_proj", "v_proj", "o_proj", "gate_proj", "up_proj", "down_proj"],
    )
    model = get_peft_model(model, lora_config)
    model.print_trainable_parameters()

    dataset = load_jsonl(dataset_path).map(format_prompt)

    training_args = TrainingArguments(
        output_dir=str(out / "checkpoints"),
        num_train_epochs=epochs,
        per_device_train_batch_size=batch_size,
        gradient_accumulation_steps=4,
        learning_rate=2e-4,
        fp16=True,
        logging_steps=5,
        save_strategy="epoch",
        optim="paged_adamw_8bit",
        report_to="none",
    )

    trainer = _make_sft_trainer(model, tokenizer, dataset, training_args)

    trainer.train()

    adapter_dir = out / "adapter"
    model.save_pretrained(adapter_dir)
    tokenizer.save_pretrained(adapter_dir)

    # Merge for Ollama / local inference (optional, needs RAM)
    merged_dir = out / "merged"
    try:
        merged = model.merge_and_unload()
        merged.save_pretrained(merged_dir)
        tokenizer.save_pretrained(merged_dir)
        print(f"Merged model saved to {merged_dir}")
    except Exception as e:
        print(f"Merge skipped (adapter still usable): {e}")

    # Metadata for Go API
    meta = {
        "base_model": base_model,
        "method": "qlora",
        "adapter_path": str(adapter_dir),
        "merged_path": str(merged_dir) if merged_dir.exists() else None,
        "epochs": epochs,
    }
    with open(out / "azula-finetune-meta.json", "w", encoding="utf-8") as f:
        json.dump(meta, f, indent=2)

    zip_path = out / "azula-adapter.zip"
    with zipfile.ZipFile(zip_path, "w", zipfile.ZIP_DEFLATED) as zf:
        for folder in [adapter_dir, merged_dir]:
            if folder.exists():
                for file in folder.rglob("*"):
                    if file.is_file():
                        zf.write(file, file.relative_to(out))
        zf.write(out / "azula-finetune-meta.json", "azula-finetune-meta.json")

    print(f"Done. Adapter: {adapter_dir}")
    print(f"Download: {zip_path}")


def main() -> None:
    parser = argparse.ArgumentParser(description="Azula QLoRA fine-tune")
    parser.add_argument("--base-model", default=os.getenv("AZULA_BASE_MODEL", DEFAULT_BASE))
    parser.add_argument("--dataset", default="data/finetune/incident_pairs.jsonl")
    parser.add_argument("--output", default="./output/azula-finetune")
    parser.add_argument("--epochs", type=int, default=2)
    parser.add_argument("--batch-size", type=int, default=4)
    parser.add_argument("--lora-r", type=int, default=16)
    args = parser.parse_args()

    if not Path(args.dataset).exists():
        raise FileNotFoundError(f"Dataset not found: {args.dataset}")

    train(
        base_model=args.base_model,
        dataset_path=args.dataset,
        output_dir=args.output,
        epochs=args.epochs,
        batch_size=args.batch_size,
        lora_r=args.lora_r,
    )


if __name__ == "__main__":
    main()
