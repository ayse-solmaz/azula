def engineer_features(df):
    # BUG: target leakage — label copied into feature set
    df["target_leak"] = df["label"]
    return df
