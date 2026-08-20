# 🌊 Guardians of the Lake

### Lake Victoria Water Quality Monitoring & Intelligence Platform

---

## 📌 Overview

**Guardians of the Lake** is a citizen-powered environmental intelligence platform that enables communities, environmental agencies, and policymakers to monitor, verify, and respond to water quality threats in Lake Victoria.

Citizens submit pollution reports with photos and GPS location. Peer verifiers validate submissions. Verified reports trigger real-time alerts, are anchored on blockchain for immutability, and reward contributors with Lightning sats — creating a trusted, transparent, and incentivized water quality monitoring ecosystem.

---

## 🏆 Hackathon Track

**Zone01 Kisumu GreenTech Hackathon 2026 - Track 1: Lake Victoria Water Quality Monitoring & Intelligence Platform**

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
git clone https://github.com/ochola-rich/guardians-of-the-lake.git
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
| **Lead Frontend** | Koimett Benjamin |
| **Frontend** | Okul Mayan |
| **Lead Backend** | Wanja Rouwel |
| **Backend** | Bernadette |
| **Backend** | Otieno Richard |

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

- **Live Demo:** TBD
- **GitHub:** https://github.com/ochola-rich/water-quality-solution.git
- **Pitch Deck:** TBD

---

*Built with ❤️ for Lake Victoria during the Zone01 Kisumu GreenTech Hackathon 2026*

