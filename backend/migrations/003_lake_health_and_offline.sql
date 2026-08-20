-- 003_lake_health_and_offline.sql: Lake health intelligence and alert subscriptions

CREATE TABLE IF NOT EXISTS lake_health (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  snapshot_date DATE NOT NULL UNIQUE,
  health_score REAL NOT NULL,
  total_reports INTEGER NOT NULL DEFAULT 0,
  breakdown TEXT NOT NULL DEFAULT '{}',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS alert_subscriptions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  endpoint TEXT UNIQUE NOT NULL,
  auth_key TEXT,
  p256dh_key TEXT,
  user_id INTEGER REFERENCES users(id),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_lake_health_date ON lake_health(snapshot_date);
CREATE INDEX IF NOT EXISTS idx_alert_subscriptions_endpoint ON alert_subscriptions(endpoint);
