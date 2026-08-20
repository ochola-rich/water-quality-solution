-- 001_init.sql: Core schema for Guardians of the Lake

CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  phone_hash TEXT UNIQUE NOT NULL,
  display_name TEXT,
  role TEXT NOT NULL DEFAULT 'citizen',  -- citizen | institution | admin
  reputation_score REAL NOT NULL DEFAULT 1.0,
  tier TEXT NOT NULL DEFAULT 'water_scout',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS reports (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES users(id),
  lat REAL NOT NULL,
  lng REAL NOT NULL,
  photo_path TEXT,
  category TEXT NOT NULL,                -- turbidity | algae | spill | smell | other
  description TEXT,
  device_meta TEXT,                      -- JSON string with metadata and fraud flags
  status TEXT NOT NULL DEFAULT 'pending', -- pending | verified | rejected | flagged
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS verifications (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  report_id INTEGER NOT NULL REFERENCES reports(id),
  verifier_id INTEGER NOT NULL REFERENCES users(id),
  vote TEXT NOT NULL,                    -- confirm | reject
  distance_m REAL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(report_id, verifier_id)
);

CREATE TABLE IF NOT EXISTS ledger_entries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  report_id INTEGER NOT NULL UNIQUE REFERENCES reports(id),
  content_hash TEXT NOT NULL,            -- sha256 of report payload
  chain_ref TEXT,                        -- null in MVP, filled in phase 2
  verified_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS rewards (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  report_id INTEGER NOT NULL REFERENCES reports(id),
  user_id INTEGER NOT NULL REFERENCES users(id),
  amount_sats INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending', -- pending | paid | failed
  lightning_invoice_id TEXT,
  paid_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_reports_status ON reports(status);
CREATE INDEX IF NOT EXISTS idx_reports_user_id ON reports(user_id);
CREATE INDEX IF NOT EXISTS idx_verifications_report_id ON verifications(report_id);
CREATE INDEX IF NOT EXISTS idx_verifications_verifier_id ON verifications(verifier_id);
CREATE INDEX IF NOT EXISTS idx_rewards_report_id ON rewards(report_id);
