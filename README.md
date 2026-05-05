# GamePulse

GamePulse is a sports text-update web app. Users pick a team in the NBA, NFL,
MLB or NHL, drop in a phone number, choose what they want (live scores,
AI-summarised news, or both) and how often they want it (realtime, hourly,
daily). The backend then polls live games, aggregates news, summarises it,
and dispatches SMS via the configured provider.

This repo contains a fully runnable implementation:

- **Backend** — Go 1.22, [Fiber](https://gofiber.io/), pgx/pgxpool, embedded
  SQL migrations, three goroutine-based workers (live tracker, news
  aggregator, digest builder).
- **Frontend** — React 18 + TypeScript, Vite, React Query.
- **Database** — PostgreSQL 16.
- **Tests** — Go unit + integration tests; Vitest + React Testing Library.
- **Deploy** — Multi-stage Dockerfiles, docker-compose for local dev.

Every external integration (sports data, news API, LLM, SMS) sits behind a
small interface and ships with a working **mock implementation**, so the
entire flow runs out of the box without any third-party accounts.

---

## Layout

```
GamePulse/
├── backend/                Go API + workers
│   ├── cmd/server/         entry point
│   ├── internal/
│   │   ├── api/            HTTP handlers
│   │   ├── config/         env-driven config
│   │   ├── db/             pgx pool + embedded migrations
│   │   ├── models/         shared types
│   │   ├── repo/           Postgres repositories
│   │   ├── services/       subscription + dispatcher
│   │   ├── workers/        live tracker, news aggregator, digest builder
│   │   └── providers/
│   │       ├── ai/         OpenAI + heuristic summariser
│   │       ├── news/       NewsAPI + mock
│   │       ├── sms/        Twilio + log-only sender
│   │       └── sports/     mock live game stream
│   └── Dockerfile
├── frontend/               React + Vite app
│   ├── src/
│   │   ├── components/     TeamDropdown, PhoneInput, …
│   │   ├── api/client.ts   typed fetch wrapper
│   │   └── App.tsx
│   └── Dockerfile
├── docker-compose.yml      one-shot local stack
├── .env.example            documented environment variables
└── README.md
```

---

## Going live

Want a real public deployment with real SMS? See **[DEPLOYMENT.md](./DEPLOYMENT.md)**
for the end-to-end recipe (Neon + Fly.io + Vercel + Twilio A2P 10DLC).
Total cost on the free tiers: **$1/mo** for a Twilio number while testing,
~$5/mo + per-message after 10DLC registration.

## Running with Docker (recommended for local dev)

```bash
git clone … gamepulse && cd gamepulse
cp .env.example .env       # fill in any creds you have; blanks → mocks
docker compose up --build
```

- Frontend → <http://localhost:5173>
- Backend  → <http://localhost:8080>
- Postgres → `localhost:5432` (user/pass/db all `gamepulse`)

The backend container runs migrations on startup and immediately begins
polling the (mock) sports data and news APIs. SMS messages — including the
welcome confirmation triggered by the form submit — are visible at
<http://localhost:8080/api/v1/debug/sms> when running with the log-only SMS
sender (i.e., no Twilio creds set).

## Running locally (without Docker)

Prerequisites: Go 1.22+, Node 20+, Postgres 16.

```bash
# Postgres (or use any existing instance)
createdb gamepulse
export DATABASE_URL=postgres://localhost:5432/gamepulse?sslmode=disable

# Backend
cd backend
go run ./cmd/server

# Frontend (in a second shell)
cd frontend
npm install
npm run dev
```

The Vite dev server proxies `/api` to `http://localhost:8080`.

---

## API

| Method | Path                          | Description                                        |
| ------ | ----------------------------- | -------------------------------------------------- |
| GET    | `/health`                     | Liveness probe.                                    |
| GET    | `/api/v1/teams`               | All teams grouped by league (NBA, NFL, MLB, NHL).  |
| POST   | `/api/v1/subscriptions`       | Create a subscription, send confirmation SMS.      |
| DELETE | `/api/v1/subscriptions/:id`   | Unsubscribe.                                       |
| GET    | `/api/v1/debug/sms`           | (Dev only) Recent SMS bodies when using log sender.|

`POST /api/v1/subscriptions` body:

```json
{
  "phone_number": "+14155550123",
  "team_id": "11111111-0000-0000-0000-000000000001",
  "update_type": "both",
  "frequency": "realtime"
}
```

Validation:

- Phone numbers must be E.164 (`/^\+[1-9]\d{1,14}$/`). Spaces, dashes, dots
  and parens are stripped before validation.
- `update_type` ∈ `live | news | both`.
- `frequency` ∈ `realtime | hourly | daily`. Choosing `news` + `realtime` is
  coerced to `news` + `hourly` per spec.
- Duplicate subscriptions (same phone, same team) → `409 Conflict`.

---

## Background workers

Each worker runs as a long-lived goroutine launched from `cmd/server`.

### Live game tracker (`internal/workers/live_tracker.go`)

Every `LIVE_TRACKER_INTERVAL` (default 8s):

1. List teams.
2. Skip teams with no live/both subscribers (saves API quota).
3. Pull live games from the sports provider.
4. Upsert each game; if home/away score changed, append a `score_update`
   row to `game_events` and fan out a notification.

Score messages look like:

```
Lakers Update: Q3 — Lakers 78, Celtics 75 (LeBron James scored)
```

### News aggregator (`internal/workers/news_aggregator.go`)

Every `NEWS_AGGREGATOR_INTERVAL` (default 15m, plus once at startup):

1. For each team with news/both subscribers: fetch articles.
2. Summarise via the AI provider; fall back to heuristic if it fails.
3. Insert with dedup on `(team_id, url)`.
4. New articles → fan out via dispatcher.

### Digest builder (`internal/workers/digest_builder.go`)

Every `DIGEST_INTERVAL` (default 1m): drains "pending" notifications stored
by the dispatcher and sends one combined SMS per hourly/daily subscriber
with all unsent updates from the relevant window.

### Dispatcher idempotency

The `notifications_log` table has a unique index on
`(subscription_id, dedupe_key)`. The live tracker's dedupe key includes the
exact score & period; the news worker uses the article's UUID. This makes
re-running a worker safe — duplicates are silently skipped.

---

## External providers

| Concern    | Real impl                          | Mock fallback                        |
| ---------- | ---------------------------------- | ------------------------------------ |
| Sports     | (pluggable)                        | `MockProvider` — simulated live games |
| News       | NewsAPI.org                        | `MockProvider` — rotating headlines   |
| AI         | OpenAI Chat Completions (`gpt-4o-mini`) | `HeuristicSummarizer` — first 1-2 sentences |
| SMS        | Twilio                             | `LogSender` — buffers + slogs messages |

Selection happens in `cmd/server/main.go`: if the relevant API key/env var is
missing, the bundled mock is used and logged at startup.

---

## Tests

### Backend

```bash
cd backend
go test ./...
```

Unit tests cover phone normalisation, subscription validation (including the
news + realtime coercion), the heuristic summariser & truncation rules, mock
sports/news providers, dispatcher message formatting, and digest body
construction.

The HTTP integration test in `internal/api/api_integration_test.go` is
auto-skipped unless `DATABASE_URL` points at a reachable Postgres. To run
it:

```bash
docker compose up -d postgres
DATABASE_URL=postgres://gamepulse:gamepulse@localhost:5432/gamepulse?sslmode=disable \
    go test ./internal/api/...
```

### Frontend

```bash
cd frontend
npm install
npm test
```

Vitest + React Testing Library cover phone validation, the team dropdown's
league grouping & change events, the phone input's invalid/valid states,
and the full App flow including:

- form disabled until valid input,
- successful submit → confirmation card,
- API error rendering,
- realtime option hidden when "news" is selected.

---

## Security & PII

- Phone numbers are stored only in the `users` table and never logged in the
  default Twilio sender path.
- The log SMS sender is for development only; it stores recent messages in
  process memory (capped at 1000) and exposes them at `/api/v1/debug/sms`.
  It must be replaced with a real provider before any public deployment.
- DB connection strings are read from `DATABASE_URL`. No credentials are
  baked into the image.

---

## Future work (per spec §13)

- Per-user accounts and login (Magic-link or OAuth).
- Multi-team subscriptions per user (the schema already supports this, the
  UI doesn't yet expose it).
- Push notifications (web push / mobile).
- Historical web dashboard of received messages.
- Real sports-data integration (Sportradar / ESPN / etc.).
- Prometheus metrics on each worker tick + dispatcher counts.
