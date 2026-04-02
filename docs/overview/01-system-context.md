# 1. System context

## Business goals (demo scope)

Users can:

1. **Upload** a video file; the system stores the original, writes metadata, and **processes in the background** (multi-bitrate encoding).
2. **Watch** after processing completes: playback over **HLS** (adaptive / manual quality).
3. **Browse and search** the catalog (lists + full-text search when Elasticsearch is enabled).
4. **Track encode status** (processing / ready / failed) via the API and optionally **WebSockets**.

This is a **learning / reference demo**, not global CDN scale, database sharding, or millions of QPS.

## Notable technical traits

| Trait | Meaning in this demo |
|-------|----------------------|
| **Split blobs & metadata** | Large files in object storage; titles, status, playlist paths in MongoDB. |
| **Heavy work off the write path** | Upload returns quickly; encode runs **asynchronously** (SQS + worker). |
| **Read-heavy** | More reads than writes; **Redis** caches video documents; search responses may be cached. |
| **Streaming** | **HLS** output (`.ts` segments + playlists); no need to download the whole file before play starts. |
| **Observable** | Optional tracing (OTel), JSON logs, Filebeat → ES (env vars in `.env.example`; Jaeger/ES/Kibana in Compose). |

## Diagram: main components (logical)

```mermaid
flowchart TB
  subgraph clients["Clients"]
    WEB["Web UI (React)"]
  end

  subgraph edge["API layer"]
    API["HTTP API (Go / chi)"]
    WS["WebSocket /ws"]
  end

  subgraph data["Data & storage"]
    MONGO[(MongoDB)]
    REDIS[(Redis)]
    ES[(Elasticsearch)]
    S3RAW["S3 raw bucket"]
    S3ENC["S3 encoded bucket"]
  end

  subgraph async["Async processing"]
    SQS[[SQS encode queue]]
    WORKER["Encoder worker (FFmpeg)"]
  end

  WEB -->|REST JSON| API
  WEB -->|subscribe| WS
  API --> MONGO
  API --> REDIS
  API --> ES
  API --> S3RAW
  API --> S3ENC
  API --> SQS
  WORKER --> SQS
  WORKER --> S3RAW
  WORKER --> S3ENC
  WORKER --> MONGO
  WORKER -->|optional pub| REDIS
  API <-->|optional sub| REDIS
  WS --> API
```

## Diagram: local deployment (conceptual)

```mermaid
flowchart LR
  subgraph host["Dev machine"]
    VITE["Vite :5173"]
    GOAPI["go run API :8080"]
    GOW["go run worker"]
  end

  subgraph compose["docker compose"]
    LS[LocalStack]
    MG[MongoDB]
    RD[Redis]
    JG[Jaeger]
    ESK[Elasticsearch / Kibana / Filebeat]
  end

  VITE --> GOAPI
  GOAPI --> LS
  GOAPI --> MG
  GOAPI --> RD
  GOW --> LS
  GOW --> MG
  GOW --> RD
  GOAPI --> ESK
  GOW --> ESK
  GOAPI --> JG
  GOW --> JG
```

In production-like setups, API and worker often **also run in containers**; the diagram above reflects a common dev pattern: UI + Go on the host, infrastructure in Compose.

## Next

- **Upload & encode flow**: [02-upload-and-encoding.md](./02-upload-and-encoding.md)
