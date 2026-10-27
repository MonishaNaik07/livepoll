# Frontend (React + Vite + TypeScript)
`npm install && npm run dev` (proxies `/api` and WebSocket to :8080) · `npm test` · `npm run build`.
Key files: `hooks/usePollSocket.ts` (live results, reconnect, resync), `utils/results.ts` (percentages + event dedupe), `services/api.ts`.
