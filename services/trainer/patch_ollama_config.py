#!/usr/bin/env python3
"""CLI: patch a HF config.json so Ollama's converter sees rope_theta."""

from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from ollama_config import patch_config_for_ollama

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("usage: patch_ollama_config.py <config.json>", file=sys.stderr)
        sys.exit(2)
    patch_config_for_ollama(sys.argv[1])
    print("patched", sys.argv[1])
