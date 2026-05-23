"""
HitLab — real-time music trend intelligence.
"""
from __future__ import annotations

import streamlit as st
from dotenv import load_dotenv

from lib import views
from lib.assets import load_css, render_template

load_dotenv()

st.set_page_config(
    page_title="HitLab",
    layout="wide",
    initial_sidebar_state="expanded",
)

load_css("main.css")

_pages = [
    st.Page(views.page_overview,      title="Overview"),
    st.Page(views.page_forecast,      title="Forecast"),
    st.Page(views.page_track_details, title="Track details"),
    st.Page(views.page_how_it_works,  title="How it works"),
]

with st.sidebar:
    render_template("sidebar_brand")
    for p in _pages:
        st.page_link(p)

pg = st.navigation(_pages, position="hidden")
pg.run()
