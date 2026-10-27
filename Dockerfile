# Single deployable image: Go API + built React app served same-origin (no CORS / third-party-cookie issues).
FROM node:22-alpine AS web
WORKDIR /web
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.22-alpine AS api
WORKDIR /src
COPY backend/ ./
RUN go mod tidy && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=api /out/server /server
COPY --from=web /web/dist /app/public
ENV STATIC_DIR=/app/public APP_ENV=production PORT=8080
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/server"]
