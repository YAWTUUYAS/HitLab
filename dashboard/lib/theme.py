"""
Plot theme + brand color tokens.
Mirrors styles/main.css :root variables so Plotly charts stay on-brand.
"""

# Brand palette — single source of truth for Plotly figures
BG          = "#F5F2EB"
SURFACE     = "#FFFFFF"
TEXT        = "#111111"
MUTED       = "#6B7280"
BORDER      = "#E7E2D9"
GRID        = "#F1ECE4"
ACCENT      = "#9A3412"
ACCENT_SOFT = "#C2410C"
ACCENT_FILL = "rgba(154,52,18,0.12)"
DASH        = "#D1D5DB"

ACCENT_SCALE = ["#E7E2D9", ACCENT_SOFT]


def apply_theme(fig):
    """Apply the HitLab brand theme to a Plotly figure."""
    fig.update_layout(
        paper_bgcolor=BG,
        plot_bgcolor=SURFACE,
        font=dict(family="Inter", color=TEXT, size=12),
        margin=dict(l=20, r=20, t=20, b=20),
        legend=dict(orientation="h", y=-0.2, bgcolor="rgba(0,0,0,0)"),
    )
    fig.update_xaxes(gridcolor=GRID, zeroline=False, linecolor=BORDER)
    fig.update_yaxes(gridcolor=GRID, zeroline=False, linecolor=BORDER)
    return fig
