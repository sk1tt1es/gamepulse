# Deploying GamePulse for free

End-to-end recipe to take GamePulse from your laptop to a production URL
your friends (and Twilio's A2P 10DLC reviewers) can hit. Total cost: $0
plus whatever Twilio charges per SMS once you go live.

| Component  | Service     | Free tier                      |
| ---------- | ----------- | ------------------------------ |
| Database   | Neon        | 0.5 GB Postgres, no card       |
| Backend    | Fly.io      | 1 shared 256 MB VM, always on  |
| Frontend   | Vercel      | Unlimited static hosting       |
| SMS        | Twilio      | $15 trial credit, then per-msg |

The order below matters: provision the database first because the
backend needs `DATABASE_URL` at deploy time, then the frontend points at
the backend.

---

## 1. Postgres on Neon (5 min, 0 cards)

1. Sign up at <https://neon.tech>.
2. Create a project named `gamepulse`. Pick the region closest to where
   you'll deploy the backend (`us-east-1` matches Fly's `iad`).
3. From the dashboard → **Connection Details** → copy the
   `postgresql://…` connection string (use the **Pooled connection**
   variant for serverless safety).
4. Save it somewhere — you'll paste it into Fly secrets next.

> Free tier limits: 0.5 GB storage and 191 compute-hours/month — far
> more than this app needs. The DB auto-suspends after 5 min idle but
> Fly will wake it on the first query.

---

## 2. Backend on Fly.io (10 min)

```bash
# One-time install
brew install flyctl              # macOS
# OR: curl -L https://fly.io/install.sh | sh

cd backend
fly auth signup                  # or `fly auth login`
fly launch --copy-config --no-deploy
# Accept the suggested app name OR pick your own.
# When asked to add Postgres / Redis: NO (we use Neon).
# When asked to deploy now: NO.
```

If `fly launch` overwrites `fly.toml`, restore the version in this repo
(it has the right env vars and `min_machines_running = 1`, which is
what keeps your background workers ticking).

Then push your secrets and deploy:

```bash
fly secrets set \
  DATABASE_URL='postgresql://USER:PASS@…neon.tech/gamepulse?sslmode=require' \
  NEWS_API_KEY='…' \
  AI_API_KEY='sk-…' \
  TWILIO_ACCOUNT_SID='AC…' \
  TWILIO_AUTH_TOKEN='…' \
  TWILIO_FROM_NUMBER='+1XXXXXXXXXX'   # leave blank until step 5

fly deploy
fly logs                         # watch migrations + workers come online
```

The backend is now live at `https://<your-app>.fly.dev`. Verify:

```bash
curl https://<your-app>.fly.dev/health
# → {"status":"ok"}
curl https://<your-app>.fly.dev/api/v1/teams | head
```

> **Important**: update `frontend/vercel.json` and replace
> `gamepulse-backend.fly.dev` with your actual Fly hostname before
> deploying the frontend.

---

## 3. Frontend on Vercel (5 min)

The cleanest setup is to push this repo to GitHub, then import it on
Vercel — they auto-detect Vite and use `frontend/vercel.json` for
routing.

```bash
git init && git add . && git commit -m "GamePulse"
gh repo create gamepulse --private --source=. --push
# OR push to any GitHub repo
```

Then on Vercel:

1. <https://vercel.com/new> → import the repo.
2. **Root Directory**: `frontend`
3. **Framework**: Vite (auto-detected).
4. **Build & Output**: leave defaults — `vercel.json` has them.
5. **Environment Variables**: none required (the rewrite in
   `vercel.json` points at the Fly backend).
6. Deploy.

Your site will be at `https://<project>.vercel.app`. The `/api/*` and
`/sms/*` requests are transparently proxied to the Fly backend, so the
browser sees a single origin — no CORS, no extra config.

> If you want a custom domain (recommended for 10DLC review), add it in
> Vercel → **Domains** and update DNS. A custom domain looks
> significantly more legitimate to carrier reviewers than a generic
> `vercel.app` subdomain.

---

## 4. Twilio account (15 min admin, 1–3 days waiting)

### 4a. Create a Twilio account

1. <https://www.twilio.com/try-twilio> — sign up, verify your email and
   phone.
2. Console → **Phone Numbers** → **Buy a Number**. Pick a US local
   number ($1/month). For testing during trial, this is fine.
3. Grab the **Account SID** and **Auth Token** from the dashboard. Set
   them in Fly:
   ```bash
   fly secrets set TWILIO_ACCOUNT_SID=AC… TWILIO_AUTH_TOKEN=… TWILIO_FROM_NUMBER=+1…
   ```

### 4b. Wire the inbound webhook

So that STOP / HELP work end-to-end:

1. Console → **Phone Numbers** → click your number.
2. Under **Messaging Configuration**:
   - **A MESSAGE COMES IN** → **Webhook**, `HTTP POST`,
     `https://<your-app>.fly.dev/sms/inbound`.
3. Save.

Test by texting `HELP` to your Twilio number. You should get a reply
within seconds.

### 4c. A2P 10DLC registration (REQUIRED for US production)

Without 10DLC, US carriers will silently filter most of your messages.
This is the step that requires a public website (which you now have).

1. Console → **Messaging** → **Regulatory Compliance** → **A2P 10DLC**.
2. **Brand Registration**:
   - Use your real or DBA business name.
   - Provide a public website URL — this is your Vercel URL (or your
     custom domain).
3. **Campaign Registration** — pick **Low Volume Mixed** for personal
   projects:
   - **Use case**: Mixed (combine notifications + marketing).
   - **Sample messages** (paste these — they match what GamePulse
     actually sends):
     ```
     Lakers Update: Q3 5:23 — Lakers 78, Boston Celtics 75
     Lakers News: Anthony Davis expected to return next week after
     injury recovery.
     GamePulse: You're subscribed to live scores and weekly news
     summaries for the Los Angeles Lakers. Reply STOP to unsubscribe.
     ```
   - **Opt-in flow**: paste a screenshot of your subscribe page showing
     the consent checkbox and the explicit disclosure. Reviewers
     specifically check for "frequency varies", "Msg & data rates may
     apply", and the STOP keyword — all of which GamePulse already
     surfaces.
   - **Opt-in keywords**: `START`, `UNSTOP`, `YES`
   - **Opt-out keywords**: `STOP`, `STOPALL`, `UNSUBSCRIBE`, `CANCEL`,
     `END`, `QUIT`
   - **Help keywords**: `HELP`, `INFO`
   - **Help message**: `GamePulse: live sports score and news updates.
     Msg & data rates may apply. Reply STOP to cancel. Support:
     support@<your-domain>`
   - **Privacy policy URL**: `https://<your-domain>/#privacy`
   - **Terms URL**: `https://<your-domain>/#terms`
4. Submit. Brand approval is usually <1 day; campaign approval can take
   1–3 business days.
5. Once approved, link your phone number to the campaign in **Phone
   Numbers** → **Manage** → select the number → **Messaging Service**.

You can now send to any verified or live phone number at full carrier
delivery rates.

---

## 5. Smoke test

```bash
# From your real phone:
# 1. Open https://<your-vercel-url>
# 2. Pick NBA → Lakers → enter your phone → check consent → submit.
# 3. You should receive a confirmation SMS within ~10 seconds.
# 4. Wait until any Lakers game is live; you'll get score updates.
# 5. Reply STOP to your Twilio number — you'll get the unsubscribe ack.
```

If anything misbehaves:

```bash
fly logs            # backend logs (workers, dispatcher, errors)
fly ssh console     # shell into the VM
```

---

## 6. Day-2 hardening (recommended once it's working)

- **Monitoring**: Fly auto-collects metrics; for app-level metrics add a
  Prometheus exporter to the Go code and ship to Grafana Cloud (free
  tier).
- **Alerts**: Fly → **Health Checks** → email-on-fail, plus Twilio
  delivery-failure webhook.
- **Backups**: Neon does point-in-time recovery on free tier. Confirm
  the retention window matches your tolerance.
- **Domain**: pointing a real domain at Vercel (and using it for
  10DLC) significantly improves carrier trust. ~$12/year at any
  registrar.
- **Disable `/api/v1/debug/sms`**: it leaks recent messages from
  in-process memory when running with the log SMS sender. With Twilio
  it returns nothing useful — you can delete the route from `api.go`
  for production builds.
- **Rate limit `POST /api/v1/subscriptions`**: add Fiber's limiter
  middleware to prevent enumeration. ~5 lines of code.

---

## Cost estimate at small scale

| Item                                | Cost                       |
| ----------------------------------- | -------------------------- |
| Vercel (frontend)                   | $0                         |
| Fly.io (backend, 1× shared 256 MB)  | $0 (free tier)             |
| Neon (Postgres, 0.5 GB)             | $0                         |
| ESPN scoreboard                     | $0 (no key)                |
| NewsAPI.org developer plan          | $0 / 100 req/day           |
| OpenAI gpt-4o-mini summarisation    | ~$0.0001 per summary       |
| Twilio US local number              | $1 / month                 |
| Twilio outbound SMS (US, A2P 10DLC) | ~$0.0079 per message       |
| Twilio 10DLC brand registration     | $4 one-time + $2 / month   |
| Twilio 10DLC campaign (Low Volume)  | $10 one-time + $1.50 / mo  |

So: **$1/mo** while testing on the trial number, **~$5/mo + per-message
costs** once 10DLC-registered for live US traffic.
