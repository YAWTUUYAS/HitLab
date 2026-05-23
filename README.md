<p align="center">
  <img src="assets/logo.svg" alt="HitLab" width="180"/>
</p>

# HitLab ⚗️

> *What's the formula for a hit track?*
> *We don't know either — but we're getting close.*

```
 ___________
|  ≡  ≡  ≡  |   C₁₂H₂₂O₁₁
|           |   listeners × geo_spread × velocity
|   ~~~~~   |   ————————————————————————————————————
|___________|   hype_score = f(reach, countries, t)
     | |
```

HitLab is a real-time data pipeline that ingests music trends across 10 countries,
computes a proprietary **hit formula** score, and surfaces emerging tracks before they blow up.

---

## Ingredients

| Component          | Role                                              | Dose                   |
|--------------------|---------------------------------------------------|------------------------|
| **Go**             | Concurrent ingestion (Last.fm) + multi-source enrichment (Deezer, iTunes) | 100+ events / 5 min |
| **Kafka**          | Message broker between ingestion & transform      | 1 topic · 3 partitions |
| **Python**         | Hit formula computation + normalization           | pandas · numpy         |
| **TimescaleDB**    | Time-series storage                               | 7-day retention        |
| **Streamlit**      | Live dashboard + time travel mode                 | —                      |
| **Docker Compose** | Full stack orchestration                          | 4 services             |

---

## The Formula

```python
hype_score = (
    reach         * 0.30 +   # log-scaled listeners (Last.fm)
    geo_spread    * 0.25 +   # number of countries trending
    deezer_rank   * 0.25 +   # streaming popularity (Deezer)
    velocity      * 0.20     # change in reach over time
)
```

---

## Experiment Setup

```bash
# Step 1 — clone the lab
git clone https://github.com/yourname/hitlab
cd hitlab

# Step 2 — add your reagents (API keys)
cp .env.example .env
# fill in LASTFM_API_KEY

# Step 3 — run the experiment
docker compose up

# Dashboard available at http://localhost:8501
```

---

## Architecture

```
Last.fm API ──┐
Deezer API  ──┼──▶  Go ingestion  ──▶  Kafka  ──▶  Python transform  ──▶  TimescaleDB  ──▶  Streamlit
iTunes API  ──┘    (goroutines +                   (hit formula)          (time-series)      (dashboard)
                    worker pool)
```

### Lab notes

- **Go service** fetches `geo.getTopTracks` from Last.fm for 10 countries in parallel using goroutines + `sync.WaitGroup`
- **Multi-source enrichment** — each track is enriched in parallel via Deezer (popularity rank, duration, preview URL) and iTunes (genre, release date) using a worker pool
- **Kafka** decouples ingestion from processing — the lab never loses a data point
- **Python consumer** aggregates events, computes the hype score, and persists results
- **TimescaleDB** stores time-series data with automatic compression and hypertable partitioning
- **Streamlit dashboard** shows live top tracks, hype score evolution, and a "time travel" mode to replay past trends

---

## Lab Structure

```
hitlab/
├── docker-compose.yml
├── .env.example
├── ingestion/              # Go service
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   ├── lastfm/             # primary source
│   │   └── client.go
│   ├── deezer/             # popularity enrichment
│   │   └── client.go
│   ├── itunes/             # metadata enrichment
│   │   └── client.go
│   └── track/              # canonical Track type
│       └── track.go
├── transform/              # Python consumer
│   ├── Dockerfile
│   ├── requirements.txt
│   └── consumer.py
└── dashboard/              # Streamlit app
    ├── Dockerfile
    └── app.py
```

---

## Results

<!-- Add a screenshot of your dashboard here -->

---

*Experiment no. 001 · Formula v0.1 · Results may vary*
