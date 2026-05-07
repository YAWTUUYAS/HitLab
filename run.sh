#!/usr/bin/env bash
# HitLab — one-shot launcher.
# Brings up Docker services, ingestion, transform, and dashboard.
# Ctrl+C stops everything.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

GREEN='\033[0;32m'; AMBER='\033[0;33m'; RED='\033[0;31m'; DIM='\033[2m'; NC='\033[0m'
log() { echo -e "${AMBER}⚗️  ${NC}$*"; }
ok()  { echo -e "${GREEN}✓  ${NC}$*"; }
err() { echo -e "${RED}✗  ${NC}$*" >&2; }

# ── Env ───────────────────────────────────────────────────────────
if [[ -f .env ]]; then
  set -a; source .env; set +a
fi
export KAFKA_BROKER="localhost:29092"
export KAFKA_TOPIC="${KAFKA_TOPIC:-track-events}"
export TIMESCALEDB_URL="postgres://hitlab:hitlab@localhost:5432/hitlab"

LOGS="$ROOT/.run_logs"
mkdir -p "$LOGS"

PIDS=()

cleanup() {
  echo
  log "shutting down..."
  for pid in "${PIDS[@]:-}"; do
    [[ -n "${pid:-}" ]] && kill -0 "$pid" 2>/dev/null && kill "$pid" 2>/dev/null || true
  done
  ok "stopped — logs in .run_logs/"
}
trap cleanup EXIT INT TERM

# ── 1. Docker services ────────────────────────────────────────────
log "starting kafka + timescaledb..."
docker compose up -d kafka timescaledb >/dev/null

log "waiting for kafka..."
for i in {1..40}; do
  if docker exec hitlab-kafka-1 kafka-topics --bootstrap-server localhost:9092 --list >/dev/null 2>&1; then
    ok "kafka ready"; break
  fi
  sleep 1
  [[ $i == 40 ]] && { err "kafka timeout"; exit 1; }
done

docker exec hitlab-kafka-1 kafka-topics --bootstrap-server localhost:9092 \
  --create --if-not-exists --topic "$KAFKA_TOPIC" \
  --partitions 3 --replication-factor 1 >/dev/null

log "waiting for timescaledb..."
for i in {1..40}; do
  if docker exec hitlab-timescaledb-1 pg_isready -U hitlab >/dev/null 2>&1; then
    ok "timescaledb ready"; break
  fi
  sleep 1
  [[ $i == 40 ]] && { err "db timeout"; exit 1; }
done

docker exec -i hitlab-timescaledb-1 psql -U hitlab -q >/dev/null 2>&1 <<'SQL' || true
CREATE TABLE IF NOT EXISTS track_events (
    time          TIMESTAMPTZ NOT NULL,
    track_name    TEXT NOT NULL,
    artist        TEXT NOT NULL,
    country       TEXT NOT NULL,
    listeners     BIGINT,
    playcount     BIGINT,
    deezer_rank   BIGINT,
    duration      INT,
    genre         TEXT,
    release_date  TIMESTAMPTZ,
    geo_spread    INT,
    hype_score    FLOAT
);
SELECT create_hypertable('track_events', 'time', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS track_events_name_artist_time
  ON track_events (track_name, artist, time DESC);
CREATE INDEX IF NOT EXISTS track_events_country_time
  ON track_events (country, time DESC);
CREATE INDEX IF NOT EXISTS track_events_hype_time
  ON track_events (hype_score DESC, time DESC);
SQL
ok "schema ready"

# ── 2. Go ingestion ───────────────────────────────────────────────
log "starting go ingestion..."
( cd ingestion && go run . ) > "$LOGS/ingestion.log" 2>&1 &
PIDS+=($!)
ok "ingestion pid $! → .run_logs/ingestion.log"

# ── 3. Python consumer ────────────────────────────────────────────
if [[ ! -d transform/venv ]]; then
  log "creating transform venv (first run, slow)..."
  python3 -m venv transform/venv
  transform/venv/bin/pip install --quiet --upgrade pip
  transform/venv/bin/pip install --quiet -r transform/requirements.txt
fi
log "starting python consumer..."
( cd transform && ./venv/bin/python consumer.py ) > "$LOGS/transform.log" 2>&1 &
PIDS+=($!)
ok "consumer pid $! → .run_logs/transform.log"

# ── 4. Streamlit dashboard (foreground) ───────────────────────────
if [[ ! -d dashboard/venv ]]; then
  log "creating dashboard venv (first run, slow)..."
  python3 -m venv dashboard/venv
  dashboard/venv/bin/pip install --quiet --upgrade pip
  dashboard/venv/bin/pip install --quiet -r dashboard/requirements.txt
fi
echo
ok "dashboard → http://localhost:8501"
echo -e "${DIM}   tail -f .run_logs/ingestion.log     # ingestion logs${NC}"
echo -e "${DIM}   tail -f .run_logs/transform.log     # consumer logs${NC}"
echo -e "${DIM}   ctrl+c to stop everything${NC}"
echo
cd dashboard && ./venv/bin/streamlit run app.py --server.headless true
