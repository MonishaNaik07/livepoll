# Evaluation report (honest status)

Status key: **Pass** = implemented and verified · **Fail** = known defect · **Not verified** = exists, not yet verified.

Build environment had **no Go toolchain, Docker, MongoDB or Redis**, so nothing on the Go side has been compiled or run yet.

| Criterion | Evidence so far | Status |
|---|---|---|
| Actually works | Code written for the full flow. No end-to-end run with two real clients yet. | **Not verified** |
| Real use of the stack | Mongo (users/polls/votes, unique indexes), Redis (hash counts, Lua idempotent apply, Pub/Sub, rate limits), Gin service layer all written. Never run against real Mongo/Redis. | **Not verified** |
| Code quality and security | Frontend: `tsc` clean, `vite build` OK, 23 Vitest tests pass. Backend: unit + integration tests written, **never executed**; code never compiled. | **Not verified** |
| UI/UX and polish | Responsive, dark mode, a11y labels/focus, states implemented. Never viewed in a browser or on a phone. | **Not verified** |
| Extra features | Written: expiry, QR code, copy/native share, CSV export, dark mode, pagination. None exercised against a live backend. | **Not verified** |
| Live link | Not deployed. | **Not verified** |

## Before you submit — run these and update this table
1. `cd backend && go mod tidy && gofmt -l . && go vet ./... && go test ./...` (fix any compile errors first)
2. `docker compose up -d` then `TEST_MONGO_URI=mongodb://localhost:27017 TEST_REDIS_ADDR=localhost:6379 go test -tags integration ./tests/...`
3. Two browsers (one incognito): create → share → vote → watch the first update with no refresh.
4. Kill Redis mid-session (`docker compose stop redis`), vote, restart Redis, confirm counts recover.
5. Deploy, repeat step 3 on the public URL, then mark rows **Pass** only with evidence.
