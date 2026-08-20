-- 002_alerts.sql: Early Warning System schema for Guardians of the Lake

CREATE TABLE IF NOT EXISTS alerts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL,
  category TEXT NOT NULL,                -- turbidity | algae | spill | smell | general
  severity TEXT NOT NULL DEFAULT 'moderate', -- moderate | high | critical
  cluster_lat REAL NOT NULL,
  cluster_lng REAL NOT NULL,
  radius_m REAL NOT NULL DEFAULT 2000.0,
  report_count INTEGER NOT NULL DEFAULT 1,
  status TEXT NOT NULL DEFAULT 'active', -- active | resolved
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  resolved_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status);
CREATE INDEX IF NOT EXISTS idx_alerts_category ON alerts(category);
