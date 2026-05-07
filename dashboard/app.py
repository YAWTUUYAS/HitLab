"""
HitLab — real-time music trend intelligence.

Entry point. Wires together styles + templates + DB + views.
Each concern lives in its own module under `lib/`.

  styles/main.css      → design tokens + component CSS
  templates/*.html     → static HTML fragments (header, cards, insights)
  lib/assets.py        → load_css / render_template
  lib/db.py            → connection + cached queries
  lib/theme.py         → Plotly theme + color tokens
  lib/features.py      → velocity / acceleration computation
  lib/views.py         → one render_* function per dashboard mode
"""
from __future__ import annotations

from datetime import timedelta

import streamlit as st
from dotenv import load_dotenv

from lib import db, features, views
from lib.assets import load_css, render_template


# ──────────────────────────────────────────────────────────────────────────
# Config
# ──────────────────────────────────────────────────────────────────────────

load_dotenv()

st.set_page_config(
    page_title="HitLab",
    layout="wide",
    initial_sidebar_state="collapsed",
)

load_css("main.css")
render_template("header")


# ──────────────────────────────────────────────────────────────────────────
# Time range
# ──────────────────────────────────────────────────────────────────────────

bounds = db.db_bounds()
if not bounds or not bounds[0]:
    st.warning("No data available.")
    st.stop()

db_min = bounds[0].replace(tzinfo=None)
db_max = bounds[1].replace(tzinfo=None)

st.markdown("## Historical range")
start, end = st.slider(
    "Time range",
    min_value=db_min,
    max_value=db_max,
    value=(max(db_min, db_max - timedelta(hours=1)), db_max),
    format="MMM DD • HH:mm",
)

raw = db.load_data(start, end)
if raw.empty:
    st.warning("No data found in this range.")
    st.stop()


# ──────────────────────────────────────────────────────────────────────────
# Feature engineering
# ──────────────────────────────────────────────────────────────────────────

df, latest, latest_t = features.engineer(raw)


# ──────────────────────────────────────────────────────────────────────────
# KPIs
# ──────────────────────────────────────────────────────────────────────────

k1, k2, k3, k4, k5 = st.columns(5)
k1.metric("Events",       f"{len(df):,}")
k2.metric("Tracks",       f"{latest.shape[0]:,}")
k3.metric("Countries",    f"{df['country'].nunique()}")
k4.metric("Average hype", f"{latest['hype_score'].mean():.1f}")
k5.metric("Updated",      latest_t.strftime("%H:%M:%S"))

st.divider()


# ──────────────────────────────────────────────────────────────────────────
# Mode router
# ──────────────────────────────────────────────────────────────────────────

mode = st.radio(
    "Mode",
    ["Overview", "Forecast", "Track details", "How it works"],
    horizontal=True,
    label_visibility="collapsed",
)

if mode == "Overview":
    views.render_overview(df, latest, latest_t)
elif mode == "Forecast":
    views.render_forecast(latest)
elif mode == "Track details":
    views.render_track_details(df, latest)
elif mode == "How it works":
    views.render_how_it_works()


# ──────────────────────────────────────────────────────────────────────────
# Insights + footer
# ──────────────────────────────────────────────────────────────────────────

if mode != "How it works":
    st.divider()
    views.render_insights(latest)

st.caption(f"Last updated • {latest_t.strftime('%Y-%m-%d %H:%M:%S UTC')}")
