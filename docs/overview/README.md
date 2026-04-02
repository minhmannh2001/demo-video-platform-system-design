# Domain overview & technical characteristics

This set of documents describes **product context**, **business flows**, and **technical traits** of the video platform demo — aimed at understanding **why** the architecture looks like this and how data flows end-to-end. Operational details (tracing, logging, Compose services) live in code comments, `docker-compose.yml`, and `.env.example`.

## Contents

| Doc | Topics |
|-----|--------|
| [1. System context](./01-system-context.md) | Demo scope, component roles, simple deployment sketch |
| [2. Upload & encoding](./02-upload-and-encoding.md) | Ingest, async pipeline, `processing → ready/failed` |
| [3. Playback & streaming](./03-playback-and-streaming.md) | HLS, manifest, multi-bitrate, read path |
| [4. Metadata, search & cache](./04-metadata-search-and-cache.md) | MongoDB, Redis, Elasticsearch and each layer’s role |
| [5. Realtime & status](./05-realtime-and-status.md) | WebSocket, polling, Redis pub/sub between worker and API |

## Related blog post

Overall design aligns with [System Design: YouTube-style architecture](https://minhmannh2001.github.io/2025/12/19/system-design-youtube-architecture-en.html): split blobs vs metadata, encode queues, read-heavy access, chunked streaming.
