# --- frontend build
FROM node:22-alpine AS web
WORKDIR /web
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# --- backend build (pure Go SQLite driver, so no CGO / gcc needed)
FROM golang:1.26-alpine AS api
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server && \
    CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 go build -o /out/ingest ./cmd/ingest

# --- runtime
FROM alpine:3.20
RUN adduser -D app && mkdir /data && chown app /data
USER app
WORKDIR /app
COPY --from=api /out/server /out/migrate /out/ingest ./
COPY --from=web /web/dist ./web
COPY backend/data/community_areas.csv backend/data/trip_hourly.csv ./seed/
COPY entrypoint.sh ./

ENV PORT=8080 \
    DB_PATH=/data/control_room.db \
    SEED_DIR=/app/seed \
    STATIC_DIR=/app/web \
    GIN_MODE=release
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
  CMD wget -qO- http://localhost:8080/healthz || exit 1
ENTRYPOINT ["./entrypoint.sh"]
