"""
Asset loaders — read CSS / HTML templates from disk.
"""
from __future__ import annotations

from pathlib import Path

import streamlit as st


ROOT = Path(__file__).resolve().parent.parent


def load_css(filename: str = "main.css") -> None:
    """Inject a CSS file as a <style> block."""
    css = (ROOT / "styles" / filename).read_text(encoding="utf-8")
    st.markdown(f"<style>{css}</style>", unsafe_allow_html=True)


def load_template(name: str, **fields: str) -> str:
    """Read an HTML template and substitute {placeholders}."""
    raw = (ROOT / "templates" / f"{name}.html").read_text(encoding="utf-8")
    # Strip whitespace per line — Streamlit treats indented lines as code blocks.
    flat = "".join(line.strip() for line in raw.splitlines())
    return flat.format(**fields) if fields else flat


def render_template(name: str, **fields: str) -> None:
    """Render an HTML template directly into the page."""
    st.markdown(load_template(name, **fields), unsafe_allow_html=True)
