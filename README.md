# LivePoll — real-time polling

Create a poll → share the link → audience votes → everyone watching sees results update live, no refresh.

**Stack (all four do real work):** React (Vite + TypeScript) · Go + Gin · MongoDB · Redis (live counts + Pub/Sub) · WebSockets.

> **Status:** Live URL: _not deployed yet_ · GitHub: _add after publishing_ · See [`docs/EVALUATION_REPORT.md`](docs/EVALUATION_REPORT.md) for what has and hasn't been verified.

## Features
Signup/login (bcrypt + signed JWT in an HttpOnly cookie) · owner-only poll create/close/reopen/delete · public voting page (no account needed) · one vote per voter enforced by a DB unique index · live results with connection status, reconnect and resync · **extras:** poll expiry, QR + copy/native share, CSV export, dashboard pagination, dark mode.

## Architecture
```
 React (Vite)  ──REST (cookies)──►  Go / Gin  ──► MongoDB   (users, polls, votes = source of truth)
      ▲  │                            │  │
      │  └──WebSocket /api/ws/polls/:id  └──► Redis  HASH counts · Lua apply · Pub/Sub · rate limits
      └───────── events fan out ◄───────────── Pub/Sub ◄── every backend instance
```
**Vote path**
1. Validate poll exists, is open (not expired), and `optionId` belongs to it.
2. Insert into Mongo `votes` — unique index `(pollId, voterId)` rejects duplicates, even under concurrency (`409 already_voted`; Redis untouched).
3. Redis **Lua script** atomically: `SADD applied voteId` (skip if already counted) → `HINCRBY counts option 1` → `INCR seq`.
4. `PUBLISH poll:<id>` with the new results snapshot; every instance's hub (one pattern subscription) forwards it to its WebSocket clients.
5. Mark the vote `applied=true` in Mongo.

New connections get an authoritative **snapshot** first; later events carry a monotonically increasing `seq` so clients drop duplicates/stale events.

### Consistency (Mongo ↔ Redis — no distributed transaction is assumed)
- **Mongo is durable truth; Redis is a rebuildable cache + bus.**
- If Mongo commits but Redis apply/publish fails: the vote stays `applied=false`; a **reconciler** (every 15 s, any instance) replays it. The Lua `SADD` guard makes replay idempotent → no double counts.
- If Redis loses data: `Results()`/`Cast()` detect the missing key and **rebuild counts + applied-set from Mongo**.
- Never publish before Mongo accepted; never increment Redis for rejected/duplicate votes.
- Clients also resync via REST every 30 s while live (heals Pub/Sub gaps) and every 5 s while disconnected.

## Data model (MongoDB)
| Collection | Fields | Indexes |
|---|---|---|
| `users` | name, email (normalized), passwordHash, createdAt | **unique** `email` |
| `polls` | ownerId, question, description, options[{id,text}], status, expiresAt?, createdAt, updatedAt | `(ownerId, createdAt desc)` |
| `votes` | pollId, optionId, voterId, applied, createdAt | **unique** `(pollId, voterId)`; `(pollId, optionId)`; `(applied, createdAt)` |

## Redis keys
`poll:<id>:counts` HASH (option→count) · `poll:<id>:applied` SET (counted vote ids) · `poll:<id>:seq` STRING · channel `poll:<id>` · `rl:<scope>:<ip>` rate-limit counters.

## Duplicate-vote policy & limits
One vote per poll per **voter id**: a random 128-bit id in an HttpOnly cookie (`lp_voter`), enforced server-side by the unique index. **Limitation:** clearing cookies / another browser = new identity, so a determined user can vote again. Per-IP rate limits slow abuse; true one-person-one-vote needs authenticated voting (deliberately not required here).

## Security
bcrypt (cost 10) with 72-byte limit enforced · JWT HS256 only, expiry required · HttpOnly + Secure (prod) + SameSite cookies · CSRF: custom `X-Requested-With` header + Origin check on state-changing requests · CORS limited to `FRONTEND_ORIGIN` · WebSocket origin check · Redis-backed rate limits (signup, login, vote, ws) · request-body limit · ownership enforced in service **and** in the Mongo query · input validated server-side before DB · generic errors, no secrets in logs · CSV formula-injection guard · login timing equalised.

## API
All responses `{ "data": … }` or `{ "error": { code, message, fields? } }`. Mutations need headers `X-Requested-With: livepoll`.
| Method & path | Auth | Purpose |
|---|---|---|
| POST `/api/auth/signup` · `/login` · `/logout` | – | account / session |
| GET `/api/auth/me` | ✔ | current user |
| POST `/api/polls` | ✔ | create (question 3–200, 2–10 unique options, optional description/expiresAt) |
| GET `/api/polls?page&pageSize` | ✔ | my polls + totals |
| GET `/api/polls/:id` | – | public poll + results + `hasVoted` + `isOwner` |
| GET `/api/polls/:id/results` | – | live snapshot |
| GET `/api/polls/:id/share` | – | public URL |
| POST `/api/polls/:id/votes` | – | `{optionId}` → 201 / 409 / 422 / 404 |
| POST `/api/polls/:id/close` · `/reopen` · DELETE `/api/polls/:id` | owner | manage |
| GET `/api/polls/:id/export` | owner | CSV |
| WS `/api/ws/polls/:id` | – | `{type:"snapshot"\|"update", data:Results}` |
| GET `/healthz` · `/readyz` | – | liveness / deps (Mongo + Redis) |

`PATCH /api/polls/:id` is **not implemented** (polls are immutable once created; close/reopen/delete cover management).

## Run locally
```bash
git clone <repo> && cd live-polling-app
docker compose up -d                       # MongoDB + Redis
cp backend/.env.example backend/.env       # set JWT_SECRET (32+ random chars)
cd backend && go mod tidy && set -a && . ./.env && set +a && go run ./cmd/server     # :8080
# new terminal
cd frontend && cp .env.example .env && npm install && npm run dev                    # :5173 (proxies /api + WS)
```
Open http://localhost:5173 in two browsers (one incognito).

## Tests
```bash
cd backend && go test ./...                                   # unit (validation, auth, status, Redis via miniredis)
TEST_MONGO_URI=mongodb://localhost:27017 TEST_REDIS_ADDR=localhost:6379 \
  go test -tags integration ./tests/...                       # real Mongo+Redis: auth, ownership, CSRF, voting, concurrency, recovery, reconcile
cd frontend && npm test                                       # 23 tests: results math, dedupe, WS reconnect/resync, forms, voting UI
```
Automated browser E2E (Playwright) is **not included**; do the two-browser check manually.

## Production build / deploy
`docker build -t livepoll .` builds one image: Go API serving the built React app (`STATIC_DIR`) **same-origin**, which avoids CORS and Safari third-party-cookie problems and supports WebSockets natively.
1. MongoDB Atlas (free) → allow your host's IPs → copy SRV URI.
2. A Redis with Pub/Sub + Lua (Render Key Value, Redis Cloud, Upstash TCP) → `REDIS_ADDR`, `REDIS_PASSWORD`, `REDIS_TLS=true`.
3. Render: New → Blueprint from this repo (`render.yaml`), fill the `sync:false` vars. Set `FRONTEND_ORIGIN` and `PUBLIC_APP_URL` to the service's https URL.
4. Verify `/readyz`, then repeat the two-browser test on the public URL.

| Env (backend) | Meaning |
|---|---|
| `PORT` `APP_ENV` | port, `production` enforces `COOKIE_SECURE=true` |
| `FRONTEND_ORIGIN` `PUBLIC_APP_URL` | allowed CORS/WS origin; base for share links |
| `MONGO_URI` `MONGO_DB` | MongoDB |
| `REDIS_ADDR` `REDIS_PASSWORD` `REDIS_TLS` | Redis |
| `JWT_SECRET` `JWT_TTL_HOURS` | signing key (≥32 chars), session length |
| `COOKIE_SECURE` `COOKIE_SAMESITE` | cookie flags |
| `MAX_BODY_BYTES` `STATIC_DIR` | body limit; serve built SPA |

## Troubleshooting
`JWT_SECRET must be at least 32 characters` → set it · `redis: connection refused` → `docker compose up -d` · WS closes immediately → `FRONTEND_ORIGIN` must exactly equal the browser origin · 403 `csrf` → send `X-Requested-With: livepoll` · cookies missing cross-site → use the same-origin Docker deploy.

## Known limitations
- **Go backend has not been compiled or run by the author of this scaffold** (no toolchain available when written). Run `go mod tidy && go vet ./... && go test ./...` first and fix anything it reports; `go.sum` is not committed.
- Anonymous voter identity is cookie-based (see above).
- `ClientIP()` trusts proxy headers by default → configure trusted proxies before relying on IP rate limits behind a proxy.
- A vote racing a poll close can land just after closing. Redis rebuild racing in-flight votes self-heals via reconcile only for unapplied votes.
- Per-poll `applied` set grows with votes (fine at this scale).
- No Playwright E2E, no per-instance connection caps, no live participant count, no dashboard search.

## Key decisions
Redis **Lua** for atomic idempotent counting · Mongo unique index as the single duplicate-vote arbiter · outbox-style `applied` flag + reconciler instead of pretending Mongo/Redis share a transaction · one Pub/Sub pattern subscription per instance with in-process fan-out · snapshot-then-delta WebSocket protocol with `seq` · cookie sessions + CSRF header rather than localStorage tokens · single same-origin container for deployment.
