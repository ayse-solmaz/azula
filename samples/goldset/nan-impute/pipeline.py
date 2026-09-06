def clean_features(df):
    # BUG: dropna on monthly_spend removes MNAR churners
    df = df.dropna(subset=["monthly_spend"])
    return df
