"""Make a Hugging Face Qwen2 config readable by Ollama's converter.

Transformers 5 nested rope_theta under rope_parameters. Ollama still
reads the top-level rope_theta; without it the GGUF RoPE is wrong and
generation collapses to '@' tokens.
"""

from __future__ import annotations

import json
from pathlib import Path


def patch_config_for_ollama(config_path: str | Path) -> None:
    path = Path(config_path)
    cfg = json.loads(path.read_text(encoding="utf-8"))
    rope = cfg.get("rope_parameters") or {}
    theta = cfg.get("rope_theta") or rope.get("rope_theta") or 1000000.0
    cfg["rope_theta"] = theta
    cfg["torch_dtype"] = cfg.get("torch_dtype") or cfg.get("dtype") or "float16"
    if cfg.get("sliding_window") in (None, 0):
        cfg["sliding_window"] = cfg.get("max_position_embeddings", 32768)
    cfg["model_type"] = "qwen2"
    path.write_text(json.dumps(cfg, indent=2) + "\n", encoding="utf-8")
