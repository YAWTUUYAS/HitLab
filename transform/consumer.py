"""
HitLab transform — reads track events from Kafka, computes hype_score, writes to TimescaleDB.

Formula:
    hype_score = reach * 0.30 + geo_spread * 0.25 + deezer_rank * 0.25 + velocity * 0.20

For now velocity is left at 0 (requires historical comparison — added in v0.2).
"""

import json
import logging
import math
import os
import time
from collections import defaultdict
from datetime import datetime, timezone

import psycopg2
from dotenv import load_dotenv
from kafka import KafkaConsumer
from psycopg2.extras import execute_values

load_dotenv()

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger("hitlab.transform")


KAFKA_BROKER = os.getenv("KAFKA_BROKER", "localhost:29092")
KAFKA_TOPIC = os.getenv("KAFKA_TOPIC", "track-events")
DB_URL = os.getenv("TIMESCALEDB_URL", "postgres://hitlab:hitlab@localhost:5432/hitlab")

BATCH_SIZE = 50          # flush every N messages...
BATCH_TIMEOUT_S = 10     # ...or every T seconds, whichever comes first


def compute_hype_score(listeners: int, geo_spread: int, deezer_rank: int) -> float:
    """
    Normalize each component to [0, 1] then weight + scale to [0, 100].

    - reach:        log10(listeners + 1) / 7   (~7 = log10(10M))
    - geo_spread:   geo_spread / 10            (max 10 countries)
    - deezer_rank:  deezer_rank / 1_000_000    (Deezer caps at 1M)
    - velocity:     0 for now (v0.1 — needs history)
    """
    reach = min(math.log10(listeners + 1) / 7.0, 1.0)
    geo = min(geo_spread / 10.0, 1.0)
    rank = min(deezer_rank / 1_000_000.0, 1.0) if deezer_rank else 0.0

    raw = reach * 0.30 + geo * 0.25 + rank * 0.25
    return round(raw * 100, 2)


def parse_release_date(s: str | None):
    if not s:
        return None
    try:
        return datetime.fromisoformat(s.replace("Z", "+00:00"))
    except ValueError:
        return None


def flush(batch: list[dict], conn) -> None:
    if not batch:
        return

    # Compute geo_spread per (track_name, artist) across this batch
    spread = defaultdict(set)
    for t in batch:
        spread[(t["name"], t["artist"])].add(t["country"])

    rows = []
    for t in batch:
        key = (t["name"], t["artist"])
        geo = len(spread[key])
        score = compute_hype_score(
            t.get("listeners", 0),
            geo,
            t.get("deezer_rank", 0),
        )
        rows.append((
            datetime.fromtimestamp(t["timestamp"], tz=timezone.utc),
            t["name"],
            t["artist"],
            t["country"],
            t.get("listeners", 0),
            t.get("playcount", 0),
            t.get("deezer_rank") or None,
            t.get("duration") or None,
            t.get("genre") or None,
            parse_release_date(t.get("release_date")),
            geo,
            score,
        ))

    with conn.cursor() as cur:
        execute_values(
            cur,
            """
            INSERT INTO track_events
                (time, track_name, artist, country, listeners, playcount,
                 deezer_rank, duration, genre, release_date, geo_spread, hype_score)
            VALUES %s
            """,
            rows,
        )
    conn.commit()
    log.info("wrote %d events to timescaledb", len(rows))


def main():
    consumer = KafkaConsumer(
        KAFKA_TOPIC,
        bootstrap_servers=KAFKA_BROKER,
        auto_offset_reset="earliest",
        enable_auto_commit=True,
        group_id="hitlab-transform",
        value_deserializer=lambda m: json.loads(m.decode("utf-8")),
    )
    log.info("kafka consumer connected to %s", KAFKA_BROKER)

    conn = psycopg2.connect(DB_URL)
    log.info("timescaledb connected")

    batch: list[dict] = []
    last_flush = time.time()

    for message in consumer:
        batch.append(message.value)

        if len(batch) >= BATCH_SIZE or (time.time() - last_flush) > BATCH_TIMEOUT_S:
            flush(batch, conn)
            batch = []
            last_flush = time.time()


if __name__ == "__main__":
    main()
