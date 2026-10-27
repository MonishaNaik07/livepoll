# Backend (Go + Gin)
`cmd/server` entrypoint · `internal/{config,handlers,middleware,models,repositories,services,routes,validation,database,redis,websocket,apperr}`.
Handlers validate + delegate; **services** hold business rules; **repositories** own Mongo; `redis` owns counters/Pub/Sub; `websocket` is the hub.
Setup, API and tests: see the root README.
