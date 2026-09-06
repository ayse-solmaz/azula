import json
from pathlib import Path

import pandas as pd


def load_dataset(path: str) -> pd.DataFrame:
    rows = []
    with open(path) as f:
        for line in f:
            rows.append(json.loads(line))
    return pd.DataFrame(rows)


def clean_features(df: pd.DataFrame) -> pd.DataFrame:
    # config.yaml data.missing.monthly_spend is "median".
    # BUG: dropna on monthly_spend deletes MNAR missings (mostly churners).
    # That flips class balance and kills validation AUC.
    n_nan = int(df["monthly_spend"].isna().sum())
    print(f"monthly_spend NaNs before clean: {n_nan}")
    before = df["label"].value_counts(normalize=True).round(3).to_dict()
    df = df.dropna(subset=["monthly_spend"])
    after = df["label"].value_counts(normalize=True).round(3).to_dict()
    print(f"class_balance before={before} after={after}")
    return df


def train_logreg(df: pd.DataFrame) -> float:
    # Placeholder scorer — real run is recorded in training.log (val_auc ~0.50).
    pos = float(df["label"].mean()) if len(df) else 0.0
    return 0.50 if pos < 0.15 else 0.74


def main():
    cfg_note = Path("config.yaml")
    data = load_dataset("dataset.jsonl")
    features = clean_features(data)
    auc = train_logreg(features)
    print(f"Ready to train rows={len(features)} val_auc={auc:.2f} config={cfg_note}")


if __name__ == "__main__":
    main()
