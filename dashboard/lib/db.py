"""
Database access layer — connection, queries, time bounds.
"""
from __future__ import annotations

import os
from datetime import datetime

import pandas as pd
import psycopg2
import streamlit as st


DB_URL = os.getenv(
    "TIMESCALEDB_URL",
    "postgres://hitlab:hitlab@localhost:5432/hitlab",
)


@st.cache_resource
def get_connection():
    conn = psycopg2.connect(DB_URL)
    conn.autocommit = True
    return conn


@st.cache_data(ttl=15)
def load_data(start: datetime, end: datetime) -> pd.DataFrame:
    conn = get_connection()
    query = """
        SELECT time, track_name, artist, country,
               listeners, deezer_rank, genre,
               geo_spread, hype_score
        FROM track_events
        WHERE time BETWEEN %s AND %s
        ORDER BY time
    """
    df = pd.read_sql(query, conn, params=(start, end))
    df["time"] = pd.to_datetime(df["time"])
    return df


@st.cache_data(ttl=60)
def db_bounds():
    conn = get_connection()
    with conn.cursor() as cur:
        cur.execute("SELECT MIN(time), MAX(time) FROM track_events")
        return cur.fetchone()
