# Guardians of the Lake — Build Plan
### Frontend + backend, hackathon MVP

Scope: the 5-stage loop from the project report — citizen report → verification → hash ledger → dashboard → reward payout. Phase 2 items (real Sentinel-2, on-chain anchoring, RWA credits, USSD) are intentionally not in this plan; see §8.

---

## 1. Repo structure

```
guardians-of-the-lake/
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── handlers/        # report, verify, dashboard, reward HTTP handlers
│   │   ├── models/          # User, Report, Verification, Reward structs
│   │   ├── db/               # sqlite connection + migrations
│   │   ├── verify/           # peer-validation + fraud-check logic
│   │   ├── ledger/           # hash logging
│   │   ├── ws/                # websocket hub + broadcast
│   │   └── lightning/         # LNbits client wrapper
│   ├── migrations/
│   │   └── 001_init.sql
│   └── go.mod
├── frontend/
│   ├── index.html             # citizen report form
│   ├── verify.html            # peer verification feed
│   ├── dashboard.html         # B2G/B2B live dashboard
│   ├── js/
│   │   ├── report.js
│   │   ├── verify.js
│   │   ├── dashboard.js
│   │   ├── ws-client.js
│   │   └── api.js
│   └── css/style.css
└── README.md
```

## 2. Database schema (SQLite)

```sql
CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  phone_hash TEXT UNIQUE NOT NULL,       -- hash of phone number, not raw number
  display_name TEXT,
  role TEXT NOT NULL DEFAULT 'citizen',  -- citizen | institution | admin
  reputation_score REAL NOT NULL DEFAULT 1.0,
  tier TEXT NOT NULL DEFAULT 'water_scout',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE reports (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES users(id),
  lat REAL NOT NULL,
  lng REAL NOT NULL,
  photo_path TEXT,
  category TEXT NOT NULL,                -- turbidity | algae | spill | smell | other
  description TEXT,
  device_meta TEXT,                      -- JSON: cell_tower_id, gps_accuracy, prev_report_delta
  status TEXT NOT NULL DEFAULT 'pending', -- pending | verified | rejected | flagged
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE verifications (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  report_id INTEGER NOT NULL REFERENCES reports(id),
  verifier_id INTEGER NOT NULL REFERENCES users(id),
  vote TEXT NOT NULL,                    -- confirm | reject
  distance_m REAL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(report_id, verifier_id)
);

CREATE TABLE ledger_entries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  report_id INTEGER NOT NULL UNIQUE REFERENCES reports(id),
  content_hash TEXT NOT NULL,            -- sha256 of report payload
  chain_ref TEXT,                        -- null in MVP, filled in phase 2
  verified_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE rewards (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  report_id INTEGER NOT NULL REFERENCES reports(id),
  user_id INTEGER NOT NULL REFERENCES users(id),
  amount_sats INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending', -- pending | paid | failed
  lightning_invoice_id TEXT,
  paid_at DATETIME
);
```

## 3. Backend — Go + Fiber

### REST routes

| Method | Path | Purpose |
|---|---|---|
| POST | `/api/reports` | Submit a report (multipart: photo + lat/lng + category + description) |
| GET | `/api/reports?status=pending` | List reports awaiting peer verification, nearest-first |
| POST | `/api/reports/:id/verify` | Submit a peer verification vote (confirm/reject) |
| GET | `/api/dashboard/summary` | Aggregated counts for the paying dashboard |
| GET | `/api/dashboard/points` | GeoJSON of verified reports for the map |
| POST | `/internal/rewards/:report_id/payout` | Trigger Lightning payout (called internally once a report crosses the verification threshold, not exposed publicly) |

### WebSocket

`/ws/dashboard` — broadcasts to connected dashboard clients.

Events: `report:new`, `report:verified`, `reward:paid`

### Verification logic (core of `internal/verify`)

```
On new vote for report R from verifier V:
  reject if V has already voted on R
  reject if distance(V.location, R.location) > 500m
  reject if V has exceeded N verifications in the last hour (rate limit)
  weight = V.reputation_score
  accumulate weighted confirm/reject totals on R
  if weighted_confirm total >= threshold:
      R.status = verified
      write sha256(R payload) to ledger_entries
      broadcast report:verified over websocket
      enqueue reward payout for R.user_id
```

### Fraud check on submit (`internal/verify`, runs before a report is even queued)

```
On new report from user U at (lat, lng, t):
  prev = U's last report
  if prev exists:
      implied_speed = distance(prev.location, new.location) / (t - prev.t)
      if implied_speed > 120 km/h: mark device_meta.flag = "impossible_movement"
  compare lat/lng against device_meta.cell_tower_id's known coverage area (if available)
  if mismatch: mark device_meta.flag = "location_mismatch"
```
Flagged reports still enter the pending queue but are surfaced to verifiers with a warning badge rather than auto-rejected — false positives are common enough that hard-blocking would cost you real reports.

## 4. Frontend

**Vanilla JS + fetch + native WebSocket API is enough for hackathon speed — no framework needed.** Leaflet.js (CDN) for the map is the one external dependency worth pulling in.

### A. Citizen report form (`index.html` / `report.js`)
- Photo capture/upload input
- `navigator.geolocation.getCurrentPosition()` to auto-fill lat/lng
- Category picker (turbidity / algae / spill / smell / other) + optional text
- On submit: POST to `/api/reports`, show pending status
- Listen on the WebSocket (or poll every 10s as a fallback) for `report:verified` / `reward:paid` matching this user's report, update status live

### B. Peer verification feed (`verify.html` / `verify.js`)
- GET `/api/reports?status=pending`, sorted by distance from the verifier's current location
- Confirm / reject buttons → POST `/api/reports/:id/verify`
- This view is what makes the peer-validation half of the loop demoable — plan to seed 2-3 test accounts to show verification happening live

### C. B2G/B2B dashboard (`dashboard.html` / `dashboard.js`)
- Leaflet map, markers from `/api/dashboard/points`, colored by category
- Live feed panel, updates via `/ws/dashboard`
- Summary stats bar: total verified reports, breakdown by category, last-24h count

## 5. Lightning reward integration

- Point at your existing LNbits regtest/testnet instance (same one from Grouper)
- On `report:verified`: create an invoice via the LNbits API for the reward amount, pay it from the platform wallet, mark the `rewards` row `paid`, broadcast `reward:paid`
- Mock the M-Pesa leg for the demo — a "converted to KES via M-Pesa" success screen is enough; wiring real Safaricom Daraja API access isn't worth the hackathon time

## 6. Build order

**Phase 0 — Setup**
Go module + Fiber skeleton, SQLite connection, run migrations, serve the static frontend from Fiber.

**Phase 1 — Core loop**
Report submission endpoint + form. Verification endpoint + feed UI. Hash written to `ledger_entries` on verification. At the end of this phase the report → verify → ledger chain works, even with no live UI polish.

**Phase 2 — Live dashboard**
WebSocket hub + broadcast on verify. Dashboard map + live feed + stats.

**Phase 3 — Reward payout**
LNbits integration wired to the verification threshold. Reward status visible on the citizen side.

**Phase 4 — Polish + demo script**
Seed realistic demo data (a handful of pre-loaded reports across real Lake Victoria coordinates reads much better than empty state), rehearse the pitch against the actual running app, fix whatever breaks under a live click-through.

## 7. Dependencies

- **Go:** `gofiber/fiber`, `modernc.org/sqlite` (pure-Go, avoids CGO setup pain), `gofiber/contrib/websocket`, `google/uuid`
- **JS:** none required — `fetch`, native `WebSocket`, `leaflet` via CDN for the map
- **Lightning:** existing LNbits regtest/testnet instance + API key

## 8. Explicitly out of scope for this build

Don't spend hackathon time on these — they're Phase 2 roadmap items from the project report, not part of the MVP:
- Real Sentinel-2 API calls (use a canned dataset if you want to gesture at it)
- Live EVM chain deployment (Polygon/Celo)
- Real M-Pesa Daraja integration (mock the final leg)
- USSD gateway for feature phones
- Tokenized RWA credits
