# Guardians of the Lake 🌊

**Citizen-powered water quality monitoring platform for Lake Victoria.**  
Built for the Zone01 Kisumu GreenTech Hackathon 2026.

---

## 📖 Overview

Guardians of the Lake empowers local lakeside communities, fishermen, and citizens to capture, verify, and monitor water quality indicators (turbidity, algae blooms, chemical spills, foul smells) across Lake Victoria. Verified reports are logged into a verifiable hash ledger, surfaced on real-time dashboards for institutions/B2G, and rewarded instantly with Bitcoin Lightning Network micro-rewards (sats) convertible to mobile money (M-Pesa).

---

## 🛠️ Tech Stack

| Layer | Technology |
|:---|:---|
| **Frontend** | Vanilla JS, HTML5, CSS3, Leaflet.js, native WebSockets |
| **Backend** | Go (Fiber), SQLite (`modernc.org/sqlite` pure-Go), WebSockets |
| **Payments / Rewards** | LNbits (Lightning Network micro-rewards in sats) |
| **Verification & Trust** | Peer consensus algorithm with reputation weighting & SHA-256 Ledger |

---

## 🚀 Build Plan & Architecture

Detailed specification and build plan are documented in [guardians-of-the-lake-build-plan.md](guardians-of-the-lake-build-plan.md).

### 5-Stage Core Loop:
1. **Citizen Report:** Geo-tagged report with photo, category, and device metadata.
2. **Fraud Detection & Peer Verification:** Rate-limiting, speed checks, radius-based verification (within 500m), and reputation-weighted consensus.
3. **Hash Ledger:** SHA-256 content hashing of verified report payload.
4. **Live Dashboard & WebSockets:** Real-time GeoJSON map markers, aggregate statistics, and live activity streams over `/ws/dashboard`.
5. **Lightning Micro-rewards:** Automated satoshi reward payout via LNbits.

---

## 👥 Team & Contributing

Refer to [AGENTS.md](AGENTS.md) for team ownership, git commit attribution rules, coding standards, and developer credentials:
- **Wanja Rouwel** (`rouwel` / `rouwelngacha@gmail.com`) — Backend Lead (Architecture, Server, Database, WebSockets)
- **Bernadette** (`BernadetteAkinyi` / `bernadetteodongo9@gmail.com`) — Backend (Reports, Fraud Detection, Ledger)
- **Otieno Richard** (`ochola-rich` / `richardochola3@gmail.com`) — Backend (Consensus Scoring, Dashboard APIs, Lightning Payouts)
- **Koimett Benjamin** — Frontend Lead
- **Okul Mayan** — Frontend
