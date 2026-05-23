-- HitLab schema — standard PostgreSQL (Supabase compatible).
-- Run this once in the Supabase SQL editor.

CREATE TABLE IF NOT EXISTS track_events (
    time          TIMESTAMPTZ NOT NULL,
    track_name    TEXT        NOT NULL,
    artist        TEXT        NOT NULL,
    country       TEXT        NOT NULL,
    listeners     BIGINT,
    playcount     BIGINT,
    deezer_rank   BIGINT,
    duration      INT,
    genre         TEXT,
    release_date  TIMESTAMPTZ,
    geo_spread    INT,
    hype_score    FLOAT
);

CREATE INDEX IF NOT EXISTS track_events_time
    ON track_events (time DESC);

CREATE INDEX IF NOT EXISTS track_events_name_artist_time
    ON track_events (track_name, artist, time DESC);

CREATE INDEX IF NOT EXISTS track_events_country_time
    ON track_events (country, time DESC);

CREATE INDEX IF NOT EXISTS track_events_hype_time
    ON track_events (hype_score DESC, time DESC);
