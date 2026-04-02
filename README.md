# Video platform (system design demo)

This repository is a **working implementation** of ideas from the system design article **[How Would You Build YouTube? A Beginner’s Guide to System Design](https://minhmannh2001.github.io/2025/12/19/system-design-youtube-architecture-en.html)**. The blog outlines the high-level architecture for a read-heavy video platform (object storage, async encoding, metadata store, caching, streaming). This project turns those concepts into a **local, end-to-end demo**: upload, background transcode to **HLS**, metadata in **MongoDB**, optional **search** and **observability** hooks.

It is **not** production YouTube scale; it is a **learning and reference codebase** aligned with the narrative in that post.

## How the blog maps to this repo

| Idea from the article | What you see here |
|----------------------|-------------------|
| Object storage for raw & encoded video | **S3-compatible** buckets via **LocalStack** (`S3_RAW_BUCKET`, `S3_ENCODED_BUCKET`) |
| Metadata separate from blobs | **MongoDB** `videos` documents (title, status, renditions, keys, …) |
| Async encoding via a queue | **SQS** job after upload; **Go worker** pulls messages, **FFmpeg** → multi-bitrate **HLS** + thumbnail, upload back to S3 |
| Read-heavy metadata path | **Redis** cache on hot video reads; **Elasticsearch** for `GET /videos/search` when configured |
| Low-latency playback | **HLS** (`master.m3u8` + variants); API serves stream URLs; **React** player uses **hls.js** / native Safari HLS |
| Real-time status (optional) | **WebSockets** (+ optional **Redis pub/sub** so worker events reach browsers) |

## Stack

- **API:** Go (`chi`), MongoDB, Redis, AWS SDK v2 → LocalStack (S3, SQS)
- **Worker:** Go, FFmpeg (360p / 720p HLS), same storage/queue
- **Web:** React, Vite, TypeScript, Tailwind, **Feature-Sliced Design**–style layout under `web/src`
- **Observability (optional):** OpenTelemetry → Jaeger; JSON logs → Elasticsearch/Filebeat/Kibana (see `docs/`)

## Prerequisites

- **Go** (see `go.mod` for the toolchain version)
- **Docker** / Docker Compose (for MongoDB, Redis, LocalStack, Jaeger, Elasticsearch, …)
- **Node.js** (for `web/`)
- **FFmpeg** and **ffprobe** on the host if you run the **worker** with `go run` (not inside a container that already includes them)

## Quick start (local)

1. **Environment**

   ```bash
   cp .env.example .env
   # Edit .env if ports or LocalStack URLs differ on your machine.
   ```

2. **Infrastructure** (from repo root — directory with `go.mod`):

   ```bash
   make compose-up-local
   ```

   Initialize LocalStack buckets/queues (once per fresh volume):

   ```bash
   ./scripts/init-localstack.sh
   ```

3. **API & worker** (two terminals, loads `.env`):

   ```bash
   make run-api
   make run-worker
   ```

   Or use **`make run-api-dev`** / **`make run-worker-dev`** for OTLP + log file (see `Makefile`).

4. **Web UI**

   ```bash
   cd web
   cp .env.example .env   # if needed
   npm install
   npm run dev
   ```

   Open the dev URL (typically `http://localhost:5173`) with `CORS_ORIGINS` in API `.env` matching your origin.

5. **Tests**

   ```bash
   go test ./...
   cd web && npm test
   ```

## Documentation

- **`docs/TRACING.md`** — Jaeger / OpenTelemetry
- **`docs/LOGGING.md`** — structured logs, Filebeat, Kibana
- **`docs/SEARCH_BACKFILL.md`** — Mongo → Elasticsearch backfill
- **`docs/WEBSOCKET_SCOPE.md`** — WebSocket usage
- **`docs/VIDEO_METADATA.md`** — metadata indexing pipeline

## Author

Companion code and article: **[System design: YouTube-style architecture](https://minhmannh2001.github.io/2025/12/19/system-design-youtube-architecture-en.html)** — *Nguyễn Minh Mạnh*.
