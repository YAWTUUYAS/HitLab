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
    "DATABASE_URL",
    "postgres://hitlab:hitlab@localhost:5432/hitlab",
)


@st.cache_resource
def _connection_holder() -> dict:
    """Holds a single mutable connection dict so we can replace it on reconnect."""
    return {"conn": psycopg2.connect(DB_URL)}


def get_connection():
    """Return a live psycopg2 connection, reconnecting if Neon closed it."""
    holder = _connection_holder()
    conn = holder["conn"]
    try:
        # Lightweight ping — catches stale connections before callers use them.
        with conn.cursor() as cur:
            cur.execute("SELECT 1")
    except psycopg2.Error:
        try:
            conn.close()
        except Exception:
            pass
        holder["conn"] = psycopg2.connect(DB_URL)
        holder["conn"].autocommit = True
        conn = holder["conn"]
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
