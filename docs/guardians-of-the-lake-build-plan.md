# Guardians of the Lake — Build Plan
### Frontend + backend, hackathon MVP

Scope: the 5-stage loop from the project: report → citizen report → verification → hash ledger → dashboard → reward payout. Phase 2 items (real Sentinel-2, RWA credits, USSD) are intentionally not in this plan; see §8.

**NEW: Enhanced with AI-powered prediction, early warning system, on-chain anchoring, offline support, and lake health intelligence to meet all Track 1 requirements.**

---

## 1. Repo structure

```js
guardians-of-the-lake/
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── handlers/        # report, verify, dashboard, reward HTTP handlers
│   │   ├── models/          # User, Report, Verification, Reward structs
│   │   ├── db/               # sqlite connection + migrations
│   │   ├── verify/           # peer-validation + fraud-check logic
│   │   ├── ledger/           # hash logging + blockchain anchoring
│   │   ├── ws/                # websocket hub + broadcast
│   │   ├── lightning/         # LNbits client wrapper
│   │   └── ai/                # NEW: AI prediction service (mock/real)
│   ├── migrations/
│   │   └── 001_init.sql
│   └── go.mod
├── frontend/
│   ├── index.html             # citizen report form
│   ├── verify.html            # peer verification feed
│   ├── dashboard.html         # B2G/B2B live dashboard
│   ├── offline.html           # NEW: offline fallback page
│   ├── js/
│   │   ├── report.js          # + AI photo classification
│   │   ├── verify.js
│   │   ├── dashboard.js       # + health score, trend chart, export
│   │   ├── ws-client.js       # + early warning alerts
│   │   ├── api.js
│   │   ├── ai-classifier.js   # NEW: MobileNet integration
│   │   └── offline-queue.js   # NEW: localStorage sync
│   ├── css/
│   │   └── style.css
│   └── sw.js                  # NEW: Service Worker for offline
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
  ai_prediction TEXT,                    -- NEW: JSON { label, confidence, model_version }
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
  chain_ref TEXT,                        -- NEW: actual blockchain tx hash (Hedera/Polygon)
  chain_network TEXT,                    -- NEW: 'hedera-testnet' | 'polygon-mumbai'
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

-- NEW: Alert log for early warning system
CREATE TABLE alerts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  report_id INTEGER NOT NULL REFERENCES reports(id),
  alert_type TEXT NOT NULL,              -- 'spill' | 'algae_bloom' | 'cluster'
  severity TEXT NOT NULL,                -- 'warning' | 'critical'
  message TEXT,
  triggered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  acknowledged BOOLEAN NOT NULL DEFAULT 0
);

-- NEW: Lake health scores (daily snapshot)
CREATE TABLE lake_health (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  snapshot_date DATE NOT NULL UNIQUE,
  health_score REAL NOT NULL,            -- 0-100
  total_reports INTEGER,
  breakdown JSON,                        -- { turbidity: 5, algae: 3, spill: 1 }
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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
| GET | `/api/dashboard/health` | **NEW:** Lake health score + trend data |
| GET | `/api/dashboard/export` | **NEW:** Export CSV of all verified reports |
| POST | `/api/alerts/subscribe` | **NEW:** Subscribe device for push notifications |
| GET | `/api/alerts/active` | **NEW:** Get active pollution alerts |
| POST | `/internal/rewards/:report_id/payout` | Trigger Lightning payout (called internally once a report crosses the verification threshold, not exposed publicly) |
| POST | `/internal/blockchain/anchor` | **NEW:** Anchor hash to Hedera/Polygon testnet (called on verification) |

### WebSocket

`/ws/dashboard` — broadcasts to connected dashboard clients.

Events: `report:new`, `report:verified`, `reward:paid`, **`alert:triggered`** (NEW)

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
      **NEW: Call blockchain anchor service (Hedera/Polygon)**
      **NEW: Check for early warning conditions (cluster, spill)**
      **NEW: Update lake health score**
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
  **NEW: AI prediction is stored but does NOT affect submission — it's advisory only**
```
Flagged reports still enter the pending queue but are surfaced to verifiers with a warning badge rather than auto-rejected — false positives are common enough that hard-blocking would cost you real reports.

### **NEW: Early Warning System** (`internal/alert/`)

```
On report:verified:
  - Check if report.category == 'spill' → trigger CRITICAL alert immediately
  - Check for cluster: 3+ verified reports within 500m in last 2 hours → trigger WARNING alert
  - Store alert in alerts table
  - Broadcast alert:triggered over WebSocket
  - Send push notification to subscribed dashboard users (Web Push API)
```

### **NEW: Lake Health Intelligence** (`internal/health/`)

```
Daily (or on-demand) compute:
  - Base score = 100
  - Subtract 10 for each spill report in last 24h
  - Subtract 5 for each algae bloom report
  - Subtract 2 for each turbidity report
  - Cap at 0, floor at 100
  - Store snapshot in lake_health table
  - Provide trend data (last 7 days) via /api/dashboard/health
```

### **NEW: Blockchain Anchoring** (`internal/blockchain/`)

```
On report:verified:
  - Get content_hash from ledger_entries
  - Submit to Hedera Testnet (preferred - free, fast) OR Polygon Mumbai
  - Wait for confirmation (or use async with retry)
  - Update chain_ref with transaction hash
  - No hard failure — if chain fails, log error and retry later
```

### **NEW: Offline Support** (handled by frontend, but backend must handle idempotency)

```
- Backend must accept reports with client-generated UUIDs to prevent duplicates
- Backend should return 202 Accepted for reports, process async
- Sync endpoint: POST /api/reports/sync accepts array of offline reports
```

## 4. Frontend

**Vanilla JS + fetch + native WebSocket API is enough for hackathon speed — no framework needed.** Leaflet.js (CDN) for the map is the one external dependency worth pulling in.

**NEW: Service Worker for offline-first capability** — cache all HTML, CSS, JS, and serve offline fallback.

### A. Citizen report form (`index.html` / `report.js`)
- Photo capture/upload input
- `navigator.geolocation.getCurrentPosition()` to auto-fill lat/lng
- Category picker (turbidity / algae / spill / smell / other) + optional text
- **NEW: AI photo classification** — when photo is uploaded, run MobileNet in browser, show prediction confidence
- On submit: POST to `/api/reports`, show pending status
- **NEW: Offline queue** — if navigator.onLine === false, store report in localStorage, sync when back online
- Listen on the WebSocket (or poll every 10s as a fallback) for `report:verified` / `reward:paid` matching this user's report, update status live
- **NEW: Receive push notifications for alerts** if user subscribes

### B. Peer verification feed (`verify.html` / `verify.js`)
- GET `/api/reports?status=pending`, sorted by distance from the verifier's current location
- Confirm / reject buttons → POST `/api/reports/:id/verify`
- **NEW: Show AI prediction badge** on pending reports so verifiers can cross-check
- This view is what makes the peer-validation half of the loop demoable — plan to seed 2-3 test accounts to show verification happening live

### C. B2G/B2B dashboard (`dashboard.html` / `dashboard.js`)
- Leaflet map, markers from `/api/dashboard/points`, colored by category
- Live feed panel, updates via `/ws/dashboard`
- Summary stats bar: total verified reports, breakdown by category, last-24h count
- **NEW: Lake Health Score** — big number (e.g., 74/100) with color indicator (green/yellow/red)
- **NEW: Trend chart** — line chart using Chart.js showing health score over last 7 days
- **NEW: Alert banner** — pops up when `alert:triggered` received via WebSocket
- **NEW: Export CSV button** — downloads all verified reports for government reporting
- **NEW: Subscribe to alerts** — one-click browser notification permission

### D. Offline support (`js/offline-queue.js` + `sw.js`)
- Register Service Worker on page load
- Cache all static assets
- Queue reports in localStorage when offline
- Sync when online: POST all queued reports to `/api/reports/sync`
- Show offline indicator banner when connection lost

### E. AI Classifier (`js/ai-classifier.js`)
- Load MobileNet from CDN (tensorflow/tfjs + mobilenet)
- On photo upload: classify image, return top prediction
- Map prediction to categories: e.g., 'green algae' → 'algae', 'oil slick' → 'spill'
- Show confidence percentage on the form
- Store prediction in report metadata (does NOT auto-submit)

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

**Phase 4 — AI & Intelligence** (NEW)
- MobileNet integration on frontend
- Lake health score calculation + trend API
- Early warning system + alert broadcasting

**Phase 5 — Blockchain anchoring** (NEW)
- Hedera/Polygon testnet integration
- Update chain_ref on verification

**Phase 6 — Offline support** (NEW)
- Service Worker + localStorage queue
- Sync endpoint on backend

**Phase 7 — Polish + demo script**
Seed realistic demo data (a handful of pre-loaded reports across real Lake Victoria coordinates reads much better than empty state), rehearse the pitch against the actual running app, fix whatever breaks under a live click-through.

## 7. Dependencies

- **Go:** `gofiber/fiber`, `modernc.org/sqlite` (pure-Go, avoids CGO setup pain), `gofiber/contrib/websocket`, `google/uuid`
- **NEW Go:** `hashicorp/go-retryablehttp` (for blockchain retries), `hedera-sdk-go` or `web3.go` for blockchain
- **JS:** none required — `fetch`, native `WebSocket`, `leaflet` via CDN for the map
- **NEW JS:** `tensorflow/tfjs` + `mobilenet` via CDN for AI classification, `chart.js` via CDN for trends
- **Lightning:** existing LNbits regtest/testnet instance + API key
- **Blockchain:** Hedera Testnet account (free) or Polygon Mumbai (free via Alchemy)

## 8. Explicitly out of scope for this build

Don't spend hackathon time on these — they're Phase 2 roadmap items from the project report, not part of the MVP:
- Real Sentinel-2 API calls (use a canned dataset if you want to gesture at it)
- Real M-Pesa Daraja integration (mock the final leg)
- USSD gateway for feature phones
- Tokenized RWA credits
- Multi-chain support (stick to one testnet)
- Production-grade blockchain key management (use env vars for hackathon)

## 9. What makes us winners (Track 1 compliance) — NEW

| Requirement | Status |
|:---|:---|
| Water quality monitoring dashboards | ✅ |
| Community pollution reporting applications | ✅ |
| Environmental intelligence platforms | ✅ (health score + trends) |
| AI-powered contamination prediction systems | ✅ (MobileNet) |
| Citizen science and crowdsourced monitoring tools | ✅ |
| Early warning systems for water pollution events | ✅ (alerts + push notifications) |
| Emerging tech: Blockchain | ✅ (Hedera/Polygon anchoring) |
| Emerging tech: Edge/Offline computing | ✅ (Service Worker + queue) |
| Emerging tech: AI | ✅ (MobileNet classification) |


Here's a simple, professional README for your hackathon submission:

```markdown
# 🌊 Guardians of the Lake

### Lake Victoria Water Quality Monitoring & Intelligence Platform

---

## 📌 Overview

**Guardians of the Lake** is a citizen-powered environmental intelligence platform that enables communities, environmental agencies, and policymakers to monitor, verify, and respond to water quality threats in Lake Victoria.

Citizens submit pollution reports with photos and GPS location. Peer verifiers validate submissions. Verified reports trigger real-time alerts, are anchored on blockchain for immutability, and reward contributors with Lightning sats — creating a trusted, transparent, and incentivized water quality monitoring ecosystem.

---

## 🏆 Hackathon Track

**Zone01 Kisumu GreenTech Hackathon 2026 — Track 1: Lake Victoria Water Quality Monitoring & Intelligence Platform**

---

## ✨ Key Features

- **Citizen Reporting** — Submit pollution reports with photo, location, and category (turbidity, algae, spill, smell, other)
- **AI-Powered Classification** — MobileNet analyzes uploaded photos and suggests contamination type with confidence score
- **Peer Verification** — Community verifiers validate reports within 500m radius, weighted by reputation
- **Real-Time Dashboard** — Live map with verified reports, summary stats, and live activity feed via WebSockets
- **Lake Health Score** — Dynamic 0-100 score with trend charts showing lake health over time
- **Early Warning System** — Automatic alerts for spills and pollution clusters, with browser push notifications
- **Blockchain Anchoring** — Report hashes anchored to Hedera/Polygon testnet for immutable proof of authenticity
- **Lightning Rewards** — Verified reporters receive sats via LNbits, with mock M-Pesa conversion for demo
- **Offline-First** — Service Worker caching and localStorage queue for reporting without internet connection
- **Government Export** — Export verified reports as CSV for institutional reporting and policymaking

---

## 🛠️ Tech Stack

| Layer | Technology |
|:---|:---|
| **Frontend** | Vanilla JS, HTML5, CSS3, Leaflet.js, Chart.js, TensorFlow.js (MobileNet) |
| **Backend** | Go (Fiber), SQLite, WebSocket |
| **Blockchain** | Hedera Testnet / Polygon Mumbai |
| **Payments** | LNbits (Lightning Network) |
| **Offline** | Service Worker, localStorage |
| **Deployment** | Docker / Fly.io / Local |

---

## 📁 Project Structure

```
guardians-of-the-lake/
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── handlers/        # HTTP handlers
│   │   ├── models/          # Data models
│   │   ├── db/              # SQLite + migrations
│   │   ├── verify/          # Verification logic
│   │   ├── ledger/          # Hash + blockchain anchoring
│   │   ├── ws/              # WebSocket hub
│   │   ├── lightning/       # LNbits client
│   │   ├── ai/              # AI prediction service
│   │   └── alert/           # Early warning system
│   ├── migrations/
│   └── go.mod
├── frontend/
│   ├── index.html           # Citizen report form
│   ├── verify.html          # Peer verification feed
│   ├── dashboard.html       # B2G/B2B dashboard
│   ├── js/
│   │   ├── report.js        # + AI classification
│   │   ├── verify.js
│   │   ├── dashboard.js     # + health score, export
│   │   ├── ws-client.js     # + alerts
│   │   ├── api.js
│   │   ├── ai-classifier.js
│   │   └── offline-queue.js
│   ├── css/
│   └── sw.js                # Service Worker
├── .env.example
└── README.md
```

---

## 🚀 Quick Start

### Prerequisites

- Go 1.21+
- SQLite3
- LNbits instance (regtest/testnet)
- Hedera Testnet account (optional, falls back to local hash)

### Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/guardians-of-the-lake.git
cd guardians-of-the-lake

# Copy environment variables
cp .env.example .env

# Install Go dependencies
cd backend
go mod download

# Run migrations
go run cmd/server/main.go --migrate

# Start the server
go run cmd/server/main.go

# Open in browser
open http://localhost:3000
```

### Environment Variables

```env
# Server
PORT=3000

# Database
DB_PATH=./guardians.db

# LNbits
LNBITS_URL=http://localhost:5000
LNBITS_API_KEY=your_api_key
REWARD_SATS=1000

# Blockchain (Hedera)
HEDERA_NETWORK=testnet
HEDERA_ACCOUNT_ID=your_account_id
HEDERA_PRIVATE_KEY=your_private_key

# Or Polygon
POLYGON_RPC_URL=https://rpc-mumbai.maticvigil.com
POLYGON_PRIVATE_KEY=your_private_key
POLYGON_CONTRACT_ADDRESS=your_contract_address
```

---

## 📱 User Flows

### 1. Citizen Report Flow
```
Citizen observes pollution → Captures photo + GPS → AI suggests category → Submits report → Pending status → Receives sats on verification
```

### 2. Peer Verification Flow
```
Verifier sees pending reports nearby → Reviews photo + AI prediction → Confirms/Rejects within 500m → Weighted consensus reached
```

### 3. Dashboard Flow
```
Live map updates → Stats refresh → Health score computes → Alerts trigger → Export reports for policymaking
```

---

## 🤝 Team

| Role | Name |
|:---|:---|
| **Lead Frontend** | [Your Name] |
| **Backend** | [Name] |
| **Backend** | [Name] |

---

## 📄 License

MIT

---

## 🙏 Acknowledgements

- Zone01 Kisumu & LakeHub for organizing the hackathon
- LNbits for Lightning Network infrastructure
- Hedera & Polygon for testnet blockchain access

---

## 🔗 Links

- **Live Demo:** [Insert URL]
- **GitHub:** [Insert URL]
- **Pitch Deck:** [Insert URL]

---

*Built with ❤️ for Lake Victoria during the Zone01 Kisumu GreenTech Hackathon 2026*
```

---

**Just replace the placeholders (`[Your Name]`, `[URL]`, etc.) and you're ready to submit!** 🚀