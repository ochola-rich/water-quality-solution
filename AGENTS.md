# AGENTS.md

This file tells any AI coding agent (Claude, Copilot, Cursor, etc.) how to work in this repository. Every team member may have an agent open on this codebase, so consistency here keeps the git history and codebase clean regardless of who — or what — is committing.

---

## 1. Project Snapshot

**Guardians of the Lake** — a citizen-powered water quality monitoring platform for Lake Victoria. Built for the Zone01 Kisumu GreenTech Hackathon 2026.

| Layer | Technology |
|:---|:---|
| Frontend | Vanilla JS, HTML5, CSS3, Leaflet.js, Chart.js, TensorFlow.js (MobileNet) |
| Backend | Go (Fiber), SQLite, WebSocket |
| Blockchain | Hedera Testnet / Polygon Mumbai |
| Payments | LNbits (Lightning Network) |
| Offline | Service Worker, localStorage |
| Deployment | Docker / Fly.io / Local |

Full feature list and user flows live in `README.md` — agents should read that alongside this file before making changes.

---

## 2. Repository Layout

```
guardians-of-the-lake/
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── handlers/    # HTTP handlers
│   │   ├── models/      # Data models
│   │   ├── db/          # SQLite + migrations
│   │   ├── verify/      # Verification logic
│   │   ├── ledger/      # Hash + blockchain anchoring
│   │   ├── ws/           # WebSocket hub
│   │   ├── lightning/   # LNbits client
│   │   ├── ai/           # AI prediction service
│   │   └── alert/        # Early warning system
│   ├── migrations/
│   └── go.mod
├── frontend/
│   ├── index.html        # Citizen report form
│   ├── verify.html        # Peer verification feed
│   ├── dashboard.html    # B2G/B2B dashboard
│   ├── js/
│   ├── css/
│   └── sw.js              # Service Worker
├── .env.example
└── README.md
```

---

## 3. Team & Ownership Map

Agents **must** attribute commits to the appropriate team member using their specific Git configuration according to the domain/module touched.

| Name | Role | Git Username (`user.name`) | Git Email (`user.email`) | Primary Ownership Area & Commit Scope |
|:---|:---|:---|:---|:---|
| **Wanja Rouwel** | Backend (Lead) | `rouwel` | `rouwelngacha@gmail.com` | Architecture, Server (`cmd/server`), Fiber router, DB setup & migrations (`internal/db`), WebSocket hub (`internal/ws`), infra |
| **Bernadette** | Backend | `BernadetteAkinyi` | `bernadetteodongo9@gmail.com` | Report handling (`internal/handlers`), Data models (`internal/models`), Fraud detection & validation (`internal/verify`), Hash ledger (`internal/ledger`) |
| **Otieno Richard** | Backend | `ochola-rich` | `richardochola3@gmail.com` | Consensus verification scoring (`internal/verify`), Dashboard endpoints (`internal/handlers`), Lightning LNbits integration (`internal/lightning`, `internal/rewards`) |
| **Koimett Benjamin** | Frontend (Lead) | `Koimett` | *Frontend team* | Citizen report UI, Form validation, Leaflet map |
| **Okul Mayan** | Frontend | `OkulMayan` | *Frontend team* | Verification feed UI, Live Dashboard, WebSocket client |

---

## 4. Universal Git Commit Attribution Protocol for AI Agents

All AI agents (Antigravity, Claude, Copilot, Cursor, etc.) working on this repository **must distribute commits across the backend team** based on the area of code touched.

### Git Author Configuration per Commit

When making commits, agents must specify author/committer credentials per commit command:

```bash
# For Wanja Rouwel (Architecture, Server, DB, WebSockets):
git -c user.name="rouwel" -c user.email="rouwelngacha@gmail.com" commit -m "..."

# For Bernadette (Models, Report Handler, Fraud Detection, Ledger):
git -c user.name="BernadetteAkinyi" -c user.email="bernadetteodongo9@gmail.com" commit -m "..."

# For Otieno Richard (Consensus Scoring, Dashboard APIs, Lightning Payouts):
git -c user.name="ochola-rich" -c user.email="richardochola3@gmail.com" commit -m "..."
```

Or via environment variables:
```bash
# Example for a block of commits:
GIT_AUTHOR_NAME="rouwel" GIT_AUTHOR_EMAIL="rouwelngacha@gmail.com" GIT_COMMITTER_NAME="rouwel" GIT_COMMITTER_EMAIL="rouwelngacha@gmail.com" git commit -m "..."
```

### Commit Distribution Rules:
1. **Module-aligned authorship:** If a commit implements a specific module, use the designated owner's git credentials.
2. **Cross-cutting changes:** If a commit spans multiple domains (e.g. end-to-end integration or repo-wide refactor), rotate the commit attribution fairly across the 3 backend developers.
3. **Atomic commits:** Always make single-purpose, atomic Conventional Commits so that authorship remains clean and modular.

---

## 5. Environment & Secrets

- Never commit `.env`. Only `.env.example` is tracked.
- Never write real values for `LNBITS_API_KEY`, `HEDERA_PRIVATE_KEY`, or `POLYGON_PRIVATE_KEY` into any file, commit message, log, or PR description.
- If a new environment variable is introduced, add it to `.env.example` with a placeholder value and document it.

---

## 6. Setup & Common Commands

**Backend (Go):**
```bash
cd backend
go mod download
go run cmd/server/main.go --migrate   # run migrations
go run cmd/server/main.go             # start server
gofmt -l .                            # check formatting
go vet ./...                          # static checks
go test ./...                         # run tests
```

**Frontend (vanilla JS, no build step):**
```bash
# served as static files by the backend, or open directly:
open frontend/index.html
```

If a linter/formatter config (e.g. `.golangci.yml`, `.eslintrc`) is added later, agents should run it before every PR and this file should be updated to reference it.

---

## 7. Coding Conventions

- **Go:** standard `gofmt` formatting, package-per-concern under `internal/` (don't add new top-level packages without discussion), errors wrapped with context (`fmt.Errorf("...: %w", err)`), pure-Go SQLite (`modernc.org/sqlite`) to avoid CGO hurdles, no unused imports/vars left behind.
- **JS:** ES6+, one concern per file under `frontend/js/` (mirrors the existing `report.js` / `verify.js` / `dashboard.js` split), no inline `<script>` blocks in HTML — keep logic in `js/`.
- Keep functions small and named for what they do (e.g. `computeHealthScore`, `calculateHaversineDistance`, not `doStuff`).

---

## 8. Git Workflow & Conventional Commits

### Branching
- All backend work should happen on appropriate branches (e.g. `backend` or feature branch `feat/<scope>-<description>`).
- Branch naming: `<type>/<scope>-<short-description>`
  - `feat/report-api`
  - `feat/verification-consensus`
  - `feat/lightning-payout`

### Commit Messages — Conventional Commits (required)

Format:
```
<type>(<scope>): <short summary, imperative mood, no trailing period>

[optional body — the "why", wrapped at ~72 chars]

[optional footer — e.g. Closes #12, BREAKING CHANGE: ...]
```

**Types:**

| Type | Use for |
|:---|:---|
| `feat` | a new feature |
| `fix` | a bug fix |
| `chore` | tooling, deps, config, non-code upkeep |
| `docs` | README/AGENTS.md/comment-only changes |
| `refactor` | code change that isn't a fix or a feature |
| `test` | adding or fixing tests |
| `style` | formatting only, no logic change |
| `perf` | performance improvement |
| `ci` | CI/CD pipeline changes |
| `build` | build system, Docker, packaging |
| `revert` | reverting a previous commit |

**Suggested scopes** (match the folder/feature touched): `report`, `verify`, `dashboard`, `ws`, `ledger`, `lightning`, `rewards`, `db`, `models`, `infra`, `docs`, `deps`

---

## 9. Universal Agent Guidelines & Do's / Don'ts

**Universal Agent Best Practices:**
- Read `AGENTS.md` and `guardians-of-the-lake-build-plan.md` (or `README.md`) before making any architectural decisions.
- Maintain consistency across all files and follow idiomatic Go patterns.
- Ensure every endpoint handles edge cases and returns standard JSON responses with error handling.
- Verify tests pass using `go test ./...` after implementing changes.

**Do:**
- Use the specific Git author credentials according to the ownership table when committing.
- Keep commits atomic and Conventional-Commit formatted.
- Update documentation and `.env.example` when commands, structure, or env vars change.

**Don't:**
- Don't push or merge directly to `main` without review.
- Don't commit `.env`, keys, or credentials.
- Don't mix unrelated changes in one commit.
- Don't introduce heavy external dependencies when standard Go packages or existing libraries suffice.
