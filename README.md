# HitLab ⚗️

> *What's the formula for a hit track?*
> *We don't know either — but we're getting close.*

```
 ___________
|  ≡  ≡  ≡  |   C₁₂H₂₂O₁₁
|           |   velocity × geo_spread × replay_rate
|   ~~~~~   |   ————————————————————————————————————
|___________|   hype_score = f(Δstreams, countries, t)
     | |
```

HitLab is a real-time data pipeline that ingests music trends across 10 countries,
computes a proprietary **hit formula** score, and surfaces emerging tracks before they blow up.

---

## Ingredients

| Component          | Role                                              | Dose                   |
|--------------------|---------------------------------------------------|------------------------|
| **Go**             | Concurrent API ingestion (Spotify + Last.fm)      | 50K+ events/day        |
| **Kafka**          | Message broker between ingestion & transform      | 1 topic · 3 partitions |
| **Python**         | Hit formula computation + normalization           | pandas · numpy         |
| **TimescaleDB**    | Time-series storage                               | 7-day retention        |
| **Streamlit**      | Live dashboard + time travel mode                 | —                      |
| **Docker Compose** | Full stack orchestration                          | 5 services             |

---

## The Formula

```python
hype_score = (
    velocity      * 0.40 +   # Δstreams / hour
    geo_spread    * 0.35 +   # number of countries trending
    replay_rate   * 0.25     # avg replays per listener
)
```

> *Disclaimer: the music industry has been trying to crack this formula for decades.
> HitLab is just more honest about it.*

---

## Experiment Setup

```bash
# Step 1 — clone the lab
git clone https://github.com/yourname/hitlab
cd hitlab

# Step 2 — add your reagents (API keys)
cp .env.example .env
# fill in SPOTIFY_CLIENT_ID, SPOTIFY_CLIENT_SECRET, LASTFM_API_KEY

# Step 3 — run the experiment
docker compose up

# Dashboard available at http://localhost:8501
```

---

## Architecture

```
Spotify API ──┐
              ├──▶  Go ingestion  ──▶  Kafka  ──▶  Python transform  ──▶  TimescaleDB  ──▶  Streamlit
Last.fm API ──┘    (goroutines)                    (hit formula)          (time-series)      (dashboard)
```

### Lab notes

- **Go service** fetches featured playlists across 10 countries in parallel using goroutines + `sync.WaitGroup`
- **Kafka** decouples ingestion from processing — the lab never loses a data point
- **Python consumer** normalizes track events and computes the hype score every 5 minutes
- **TimescaleDB** stores time-series data with automatic compression
- **Streamlit dashboard** shows live top tracks, hype score evolution, and a "time travel" mode to replay past trends

---

## Results

<!-- Add a screenshot of your dashboard here -->
> *Experiment no. 001 · Formula v0.1 · Results may vary*

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
│   └── spotify/
│       └── client.go
├── transform/              # Python consumer
│   ├── Dockerfile
│   ├── requirements.txt
│   └── consumer.py
└── dashboard/              # Streamlit app
    ├── Dockerfile
    └── app.py
```

---

## CV Description

**Short version**
> HitLab — Real-time music trend pipeline · Go microservice ingesting 50K+ events/day from Spotify & Last.fm APIs via concurrent goroutines · Kafka message broker · Python transformation layer computing a proprietary "hit formula" score · TimescaleDB · Streamlit dashboard · fully containerized with Docker Compose

**Narrative version**
> Built HitLab, a real-time data pipeline that decodes the formula behind viral music. A Go service fetches trending tracks across 10 countries in parallel (goroutines), feeds a Kafka topic, Python computes a hype score from velocity + geo spread, stored in TimescaleDB and visualized live on a Streamlit dashboard. Fully containerized with Docker Compose.

---

*Experiment no. 001 · Formula v0.1 · Results may vary*
