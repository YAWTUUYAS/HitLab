"""
View renderers — one render_* function per section, one page_* function per page.
"""
from __future__ import annotations

from datetime import timedelta

import pandas as pd
import plotly.express as px
import plotly.graph_objects as go
import streamlit as st

from . import db, features, theme
from .assets import render_template, load_template


# ──────────────────────────────────────────────────────────────────────────
# HELPERS
# ──────────────────────────────────────────────────────────────────────────


def _section(step: str, title: str, sub: str) -> None:
    render_template("section", step=step, title=title, sub=sub)


def _track_row(rank: int, row) -> None:
    velo = float(row.get("velocity", 0) or 0)
    velo_class = "velo-up" if velo > 0 else "velo-down" if velo < 0 else "velo-flat"
    bar = max(0, min(100, int(row["hype_score"])))
    render_template(
        "track_row",
        rank=str(rank),
        track=str(row["track_name"]),
        artist=str(row["artist"]),
        markets=str(int(row["geo_spread"])),
        genre=str(row.get("genre") or "—"),
        bar=str(bar),
        hype=f"{row['hype_score']:.1f}",
        velo=f"{velo:+.2f}",
        velo_class=velo_class,
        velo_arrow="",
    )


# ──────────────────────────────────────────────────────────────────────────
# OVERVIEW
# ──────────────────────────────────────────────────────────────────────────


def render_overview(df: pd.DataFrame, latest: pd.DataFrame, latest_t) -> None:

    _section("01", "Top momentum", "Tracks ranked by hype score across all markets right now.")

    left, right = st.columns([1.4, 1])

    with left:
        top = (
            latest.sort_values(["hype_score", "velocity"], ascending=False)
            .head(12)
        )
        for i, row in enumerate(top.itertuples(), 1):
            _track_row(i, row._asdict())

    with right:
        st.markdown("<div class='page-side-head'>Regional activity</div>", unsafe_allow_html=True)
        geo = (
            df[df["time"] == latest_t]
            .groupby("country", as_index=False)["hype_score"].mean()
            .sort_values("hype_score", ascending=False)
        )
        fig = px.bar(geo, x="country", y="hype_score", text_auto=".0f")
        fig.update_traces(marker_color=theme.ACCENT)
        fig.update_layout(height=620, xaxis_title=None, yaxis_title=None, showlegend=False)
        st.plotly_chart(theme.apply_theme(fig), use_container_width=True)

    st.markdown("<div class='page-spacer'></div>", unsafe_allow_html=True)
    _section("02", "Momentum over time", "Top 5 tracks tracked across the selected window.")

    top5 = latest.sort_values("hype_score", ascending=False).head(5)[["track_name", "artist"]]
    evo = (
        df.merge(top5, on=["track_name", "artist"])
        .groupby(["time", "track_name"], as_index=False)["hype_score"].mean()
    )
    fig = px.line(evo, x="time", y="hype_score", color="track_name")
    fig.update_traces(line=dict(width=2.2), mode="lines+markers", marker=dict(size=5))
    fig.update_layout(height=420, xaxis_title=None, yaxis_title="Hype score")
    st.plotly_chart(theme.apply_theme(fig), use_container_width=True)


# ──────────────────────────────────────────────────────────────────────────
# FORECAST
# ──────────────────────────────────────────────────────────────────────────


def render_forecast(latest: pd.DataFrame) -> None:

    _section("01", "Velocity × Acceleration", "Each bubble is a track. Position reveals momentum direction; size reveals current hype.")

    fig = px.scatter(
        latest,
        x="velocity", y="acceleration",
        size="hype_score", color="hype_score",
        hover_data={"track_name": True, "artist": True, "geo_spread": True,
                    "hype_score": ":.1f", "velocity": ":.2f", "acceleration": ":.2f"},
        color_continuous_scale=theme.ACCENT_SCALE,
        size_max=44,
    )
    fig.add_hline(y=0, line_dash="dot", line_color=theme.DASH)
    fig.add_vline(x=0, line_dash="dot", line_color=theme.DASH)

    if not latest.empty:
        x_pad = (latest["velocity"].abs().max() or 1) * 0.6
        y_pad = (latest["acceleration"].abs().max() or 1) * 0.7
        fig.add_annotation(x=x_pad, y=y_pad, text="EMERGING",
                           showarrow=False, font=dict(color=theme.ACCENT, size=11))
        fig.add_annotation(x=-x_pad, y=-y_pad, text="COOLING",
                           showarrow=False, font=dict(color="#6B7280", size=11))

    fig.update_layout(
        height=540,
        xaxis_title="Velocity (Δ hype / cycle)",
        yaxis_title="Acceleration (Δ velocity / cycle)",
        coloraxis_showscale=False,
    )
    st.plotly_chart(theme.apply_theme(fig), use_container_width=True)

    render_template("quadrant_legend")

    st.markdown("<div class='page-spacer'></div>", unsafe_allow_html=True)
    _section("02", "Emerging vs declining", "Tracks ranked by acceleration on one side, by velocity on the other.")

    left, right = st.columns(2)

    with left:
        st.markdown("<div class='page-side-head page-side-up'>Emerging</div>", unsafe_allow_html=True)
        emerging = (
            latest[(latest["acceleration"] > 0) & (latest["hype_score"] < 70)]
            .sort_values("acceleration", ascending=False)
            .head(8)
        )
        if emerging.empty:
            st.info("No emerging tracks in this window.")
        else:
            for i, row in enumerate(emerging.itertuples(), 1):
                _track_row(i, row._asdict())

    with right:
        st.markdown("<div class='page-side-head page-side-down'>Declining</div>", unsafe_allow_html=True)
        cooling = (
            latest[(latest["velocity"] < 0) & (latest["acceleration"] < 0)]
            .sort_values("velocity")
            .head(8)
        )
        if cooling.empty:
            st.info("No declining tracks in this window.")
        else:
            for i, row in enumerate(cooling.itertuples(), 1):
                _track_row(i, row._asdict())


# ──────────────────────────────────────────────────────────────────────────
# TRACK DETAILS
# ──────────────────────────────────────────────────────────────────────────


def render_track_details(df: pd.DataFrame, latest: pd.DataFrame) -> None:

    _section("01", "Pick a track", "Drill into a single track's hype curve, momentum, and country distribution.")

    options = (
        latest.sort_values("hype_score", ascending=False)
        .apply(lambda r: f"{r['track_name']} — {r['artist']}", axis=1)
        .tolist()
    )
    selected = st.selectbox("Track", options, label_visibility="collapsed")
    sel_name, sel_artist = selected.split(" — ", 1)

    track_df = df[(df["track_name"] == sel_name) & (df["artist"] == sel_artist)]
    track_now = latest[(latest["track_name"] == sel_name) & (latest["artist"] == sel_artist)].iloc[0]

    render_template(
        "track_card",
        track_name=track_now["track_name"],
        artist=track_now["artist"],
        genre=(track_now["genre"] or "Unknown genre"),
        markets=str(int(track_now["geo_spread"])),
        listeners=f"{int(track_now['listeners']):,}",
        hype=f"{track_now['hype_score']:.1f}",
        velo=f"{track_now['velocity']:+.2f}",
        accel=f"{track_now['acceleration']:+.2f}",
    )

    st.markdown("<div class='page-spacer'></div>", unsafe_allow_html=True)
    _section("02", "Hype + momentum", "How the track has evolved over the selected window.")

    left, right = st.columns(2)

    with left:
        st.markdown("<div class='page-side-head'>Hype evolution</div>", unsafe_allow_html=True)
        evo = track_df.groupby("time", as_index=False)["hype_score"].mean()
        fig = px.area(evo, x="time", y="hype_score")
        fig.update_traces(line=dict(color=theme.ACCENT, width=2.2),
                          fillcolor=theme.ACCENT_FILL,
                          mode="lines+markers", marker=dict(size=4))
        fig.update_layout(height=360, xaxis_title=None, yaxis_title="Hype score")
        st.plotly_chart(theme.apply_theme(fig), use_container_width=True)

    with right:
        st.markdown("<div class='page-side-head'>Momentum profile</div>", unsafe_allow_html=True)
        derv = track_df.groupby("time", as_index=False)[["velocity", "acceleration"]].mean()
        fig = go.Figure()
        fig.add_trace(go.Scatter(x=derv["time"], y=derv["velocity"], name="Velocity",
                                 line=dict(width=2.2, color=theme.ACCENT),
                                 mode="lines+markers", marker=dict(size=4)))
        fig.add_trace(go.Scatter(x=derv["time"], y=derv["acceleration"], name="Acceleration",
                                 line=dict(width=2, dash="dot", color=theme.MUTED),
                                 mode="lines+markers", marker=dict(size=4)))
        fig.add_hline(y=0, line_dash="dot", line_color=theme.DASH)
        fig.update_layout(height=360, xaxis_title=None, yaxis_title=None)
        st.plotly_chart(theme.apply_theme(fig), use_container_width=True)

    st.markdown("<div class='page-spacer'></div>", unsafe_allow_html=True)
    _section("03", "Regional distribution", "Where the track is currently trending.")

    geo = (
        track_df.groupby("country", as_index=False)["hype_score"].mean()
        .sort_values("hype_score", ascending=False)
    )
    fig = px.bar(geo, x="country", y="hype_score", text_auto=".0f")
    fig.update_traces(marker_color=theme.ACCENT)
    fig.update_layout(height=320, xaxis_title=None, yaxis_title="Hype score")
    st.plotly_chart(theme.apply_theme(fig), use_container_width=True)


# ──────────────────────────────────────────────────────────────────────────
# INSIGHTS
# ──────────────────────────────────────────────────────────────────────────


def render_insights(latest: pd.DataFrame) -> None:
    st.markdown("## Insights")

    items: list[tuple[str, str]] = []

    top_accel = latest.nlargest(1, "acceleration").iloc[0]
    top_velo = latest.nlargest(1, "velocity").iloc[0]
    top_drop = latest.nsmallest(1, "velocity").iloc[0]
    widest = latest.nlargest(1, "geo_spread").iloc[0]

    if top_accel["acceleration"] > 0:
        items.append((
            "Strongest acceleration",
            f"{top_accel['track_name']} is showing the fastest acceleration "
            f"({top_accel['acceleration']:+.2f}).",
        ))
    if top_velo["velocity"] > 0:
        items.append((
            "Fastest momentum growth",
            f"{top_velo['track_name']} currently has the highest velocity "
            f"({top_velo['velocity']:+.2f}).",
        ))
    if top_drop["velocity"] < 0:
        items.append((
            "Momentum decline",
            f"{top_drop['track_name']} is losing momentum "
            f"({top_drop['velocity']:+.2f}).",
        ))
    if widest["geo_spread"] >= 6:
        items.append((
            "Largest regional reach",
            f"{widest['track_name']} is active across "
            f"{int(widest['geo_spread'])} markets.",
        ))

    if not items:
        st.info("No anomalies detected in the current window.")
        return

    for title, body in items:
        render_template("insight", title=title, body=body)


# ──────────────────────────────────────────────────────────────────────────
# HOW IT WORKS
# ──────────────────────────────────────────────────────────────────────────


def render_how_it_works() -> None:
    render_template("how_it_works")


# ──────────────────────────────────────────────────────────────────────────
# PAGE WRAPPERS  (called by st.navigation in app.py)
# ──────────────────────────────────────────────────────────────────────────


def _load_data() -> tuple[pd.DataFrame, pd.DataFrame, pd.Timestamp]:
    """Shared data loading block rendered at the top of every data page."""
    bounds = db.db_bounds()
    if not bounds or not bounds[0]:
        st.warning("No data available yet — the pipeline may still be warming up.")
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

    df, latest, latest_t = features.engineer(raw)

    k1, k2, k3, k4, k5 = st.columns(5)
    k1.metric("Events",       f"{len(df):,}")
    k2.metric("Tracks",       f"{latest.shape[0]:,}")
    k3.metric("Countries",    f"{df['country'].nunique()}")
    k4.metric("Average hype", f"{latest['hype_score'].mean():.1f}")
    k5.metric("Updated",      latest_t.strftime("%H:%M:%S"))

    st.divider()
    return df, latest, latest_t


def _footer(latest_t: pd.Timestamp, latest: pd.DataFrame) -> None:
    st.divider()
    render_insights(latest)
    st.caption(f"Last updated • {latest_t.strftime('%Y-%m-%d %H:%M:%S UTC')}")


def page_overview() -> None:
    df, latest, latest_t = _load_data()
    render_overview(df, latest, latest_t)
    _footer(latest_t, latest)


def page_forecast() -> None:
    df, latest, latest_t = _load_data()
    render_forecast(latest)
    _footer(latest_t, latest)


def page_track_details() -> None:
    df, latest, latest_t = _load_data()
    render_track_details(df, latest)
    _footer(latest_t, latest)


def page_how_it_works() -> None:
    render_how_it_works()
