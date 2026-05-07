"""
Feature engineering — derives velocity, acceleration, and the latest snapshot.
"""
from __future__ import annotations

import pandas as pd


def engineer(df: pd.DataFrame) -> tuple[pd.DataFrame, pd.DataFrame, pd.Timestamp]:
    """
    Add per-(track, artist, country) velocity & acceleration to the raw event log.

    Returns
    -------
    df       : same dataframe with added columns (sorted)
    latest   : one row per (track, artist) at the most recent timestamp
    latest_t : the latest timestamp
    """
    df = df.sort_values(["track_name", "artist", "country", "time"])

    grp = df.groupby(["track_name", "artist", "country"])
    df["velocity"] = grp["hype_score"].diff()
    df["acceleration"] = grp["velocity"].diff()

    latest_t = df["time"].max()

    latest = (
        df[df["time"] == latest_t]
        .groupby(["track_name", "artist"], as_index=False)
        .agg(
            hype_score=("hype_score", "max"),
            velocity=("velocity", "mean"),
            acceleration=("acceleration", "mean"),
            geo_spread=("geo_spread", "max"),
            listeners=("listeners", "sum"),
            deezer_rank=("deezer_rank", "max"),
            genre=("genre", "first"),
        )
    )
    latest["velocity"] = latest["velocity"].fillna(0)
    latest["acceleration"] = latest["acceleration"].fillna(0)

    return df, latest, latest_t
