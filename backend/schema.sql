-- claude-top D1 schema
-- Run with: wrangler d1 execute claude-top-db --file=schema.sql

-- devices: one row per unique (github_id, device_id) pair.
CREATE TABLE IF NOT EXISTS devices (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  github_id     INTEGER NOT NULL,
  github_login  TEXT    NOT NULL,
  avatar_url    TEXT    NOT NULL DEFAULT '',
  device_id     TEXT    NOT NULL,  -- UUID generated on client
  device_name   TEXT    NOT NULL DEFAULT '',
  registered_at TEXT    NOT NULL DEFAULT (datetime('now')),
  UNIQUE (github_id, device_id)
);

CREATE INDEX IF NOT EXISTS idx_devices_github_id ON devices (github_id);

-- uploads: one row per (device_id, period, source). Upsert on conflict.
-- Aggregation across devices is done at query time by summing over github_id.
-- source: "claude" or "codex" — identifies the data origin tool.
CREATE TABLE IF NOT EXISTS uploads (
  id                 INTEGER PRIMARY KEY AUTOINCREMENT,
  github_id          INTEGER NOT NULL,
  device_id          TEXT    NOT NULL,
  period             TEXT    NOT NULL,  -- YYYY-MM
  source             TEXT    NOT NULL DEFAULT 'claude',  -- "claude" or "codex"
  total_cost_usd     REAL    NOT NULL DEFAULT 0,
  total_tokens       INTEGER NOT NULL DEFAULT 0,
  input_tokens       INTEGER NOT NULL DEFAULT 0,
  output_tokens      INTEGER NOT NULL DEFAULT 0,
  cache_read_tokens  INTEGER NOT NULL DEFAULT 0,
  cache_write_tokens INTEGER NOT NULL DEFAULT 0,
  session_count      INTEGER NOT NULL DEFAULT 0,
  model_breakdown    TEXT    NOT NULL DEFAULT '{}',  -- JSON
  uploaded_at        TEXT    NOT NULL DEFAULT (datetime('now')),
  UNIQUE (device_id, period, source)
);

CREATE INDEX IF NOT EXISTS idx_uploads_github_period        ON uploads (github_id, period);
CREATE INDEX IF NOT EXISTS idx_uploads_period               ON uploads (period);
CREATE INDEX IF NOT EXISTS idx_uploads_period_source        ON uploads (period, source);
CREATE INDEX IF NOT EXISTS idx_uploads_github_period_source ON uploads (github_id, period, source);
