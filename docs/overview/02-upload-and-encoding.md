# 2. Upload & encoding (write path)

## Business flow

1. The user selects a video file, enters title / description (per form), and submits.
2. The system **accepts quickly**: stores the raw object, creates a video record in **processing** state.
3. A **worker** pulls work from the queue, downloads the original, **transcodes** (e.g. 360p + 720p HLS), uploads results to storage, then updates metadata to **ready** or **failed**.

The user **does not** wait for step 3 before getting the HTTP response from upload — matching the “decouple with queues” idea from read/write design.

## Technical details

| Aspect | In this repo |
|--------|----------------|
| Raw object | **PUT** to S3 (presigned or API-mediated upload, depending on handler) + key stored in MongoDB. |
| Metadata | One `videos` document with `status`, `raw_s3_key`, etc. |
| Encode trigger | **SQS** message carrying `video_id`. |
| Worker | Separate Go process: receive from SQS → FFmpeg → upload HLS + thumbnail → `MarkReady` / `MarkFailed`. |
| Renditions | Quality list (label + playlist path) **persisted in MongoDB** after encode, aligned with on-disk HLS layout. |

## Sequence: upload → enqueue

```mermaid
sequenceDiagram
  participant U as User / Web
  participant API as API
  participant S3 as Object storage (S3)
  participant DB as MongoDB
  participant Q as SQS

  U->>API: Upload video + metadata
  API->>S3: Store raw object
  API->>DB: insert video (status processing)
  API->>Q: SendMessage (video_id)
  API-->>U: 200 + id, processing

  Note over Q,DB: Worker continues this flow (next diagram)
```

## Sequence: worker encode

```mermaid
sequenceDiagram
  participant Q as SQS
  participant W as Worker
  participant S3 as S3
  participant DB as MongoDB
  participant R as Redis / WS bridge

  Q->>W: ReceiveMessage (video_id)
  W->>S3: GetObject raw
  W->>W: FFmpeg HLS + thumbnail
  W->>S3: PutObject segments + playlists
  alt success
    W->>DB: MarkReady + renditions + duration
    W->>R: optional publish status
  else error
    W->>DB: MarkFailed
    W->>R: optional publish failed
  end
  W->>Q: DeleteMessage
```

## Video status state machine (conceptual)

```mermaid
stateDiagram-v2
  [*] --> processing: Upload accepted
  processing --> ready: Encode OK
  processing --> failed: Encode error
  ready --> [*]: Can watch / stream
  failed --> [*]: Playback unavailable (UI shows error)
```

## Operations notes

- **Idempotency / duplicate jobs**: depends on queue policy; the demo usually assumes one job per successful upload.
- **Search index fan-out**: after status updates, a separate queue may sync **Elasticsearch** (`SQS_VIDEO_METADATA_QUEUE_URL` and the metadata consumer in the codebase).

## See also

- Read path to play the result: [03-playback-and-streaming.md](./03-playback-and-streaming.md)
- Realtime when `ready`: [05-realtime-and-status.md](./05-realtime-and-status.md)
