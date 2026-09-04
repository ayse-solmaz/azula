#!/usr/bin/env python3
"""Reload QLoRA adapter onto an fp16 base and save a clean HF folder for Ollama."""

from __future__ import annotations

import argparse
from pathlib import Path
import sys

import torch
from peft import PeftModel
from transformers import AutoModelForCausalLM, AutoTokenizer

sys.path.insert(0, str(Path(__file__).resolve().parent))
from ollama_config import patch_config_for_ollama

DEFAULT_BASE = "Qwen/Qwen2.5-1.5B-Instruct"


def merge(base_model: str, adapter_dir: str, output_dir: str) -> None:
    adapter = Path(adapter_dir)
    out = Path(output_dir)
    if not (adapter / "adapter_model.safetensors").exists() and not (adapter / "adapter_model.bin").exists():
        raise FileNotFoundError(f"No adapter weights in {adapter}")
    out.mkdir(parents=True, exist_ok=True)

    print(f"Loading base {base_model} (fp16, CPU)...")
    tokenizer = AutoTokenizer.from_pretrained(adapter if (adapter / "tokenizer.json").exists() else base_model)
    base = AutoModelForCausalLM.from_pretrained(
        base_model,
        dtype=torch.float16,
        device_map="cpu",
        trust_remote_code=True,
    )
    print(f"Loading adapter {adapter}...")
    model = PeftModel.from_pretrained(base, str(adapter))
    print("Merging LoRA into base...")
    merged = model.merge_and_unload()
    if hasattr(merged.config, "quantization_config"):
        merged.config.quantization_config = None
    merged.save_pretrained(out, safe_serialization=True)
    tokenizer.save_pretrained(out)
    patch_config_for_ollama(out / "config.json")
    print(f"Ollama-ready weights: {out}")


def main() -> None:
    parser = argparse.ArgumentParser(description="Merge Azula LoRA adapter for Ollama")
    parser.add_argument("--base-model", default=DEFAULT_BASE)
    parser.add_argument("--adapter", default="adapters/azula-incident/adapter")
    parser.add_argument("--output", default="adapters/azula-incident/merged-fp16")
    args = parser.parse_args()
    merge(args.base_model, args.adapter, args.output)


if __name__ == "__main__":
    main()
