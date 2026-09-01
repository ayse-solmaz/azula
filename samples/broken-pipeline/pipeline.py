import json
import pandas as pd
from pathlib import Path


def load_dataset(path: str) -> pd.DataFrame:
    rows = []
    with open(path) as f:
        for line in f:
            rows.append(json.loads(line))
    return pd.DataFrame(rows)


def engineer_features(df: pd.DataFrame) -> pd.DataFrame:
    # BUG: target leakage — label copied into feature set
    df["target_leak"] = df["label"]

    # BUG: forces mixed types — int categories become strings
    df["customer_status"] = df["customer_status"].astype(str)

    return df


def build_dataloader(df: pd.DataFrame, batch_size: int):
  # batch_size from config.yaml — 128 exceeds GPU memory
  return df, batch_size


def main():
    config_path = Path("config.yaml")
    data = load_dataset("dataset.jsonl")
    features = engineer_features(data)
    loader, batch_size = build_dataloader(features, batch_size=128)
    print(f"Ready to train with batch_size={batch_size}, rows={len(loader)}")


if __name__ == "__main__":
    main()
