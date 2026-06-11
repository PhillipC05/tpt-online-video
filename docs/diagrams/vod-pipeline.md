# VOD Pipeline

```mermaid
sequenceDiagram
    participant Browser
    participant API
    participant Redis
    participant Worker
    participant FFmpeg
    participant Storage
    participant DB

    Browser->>API: Create upload session
    API->>DB: Insert upload session
    API-->>Browser: Session ID + presigned URL

    loop Chunk upload
        Browser->>API: Upload chunk
        API->>Storage: Store chunk
        API-->>Browser: Chunk acknowledged
    end

    Browser->>API: Complete upload
    API->>Storage: Finalize/assemble file
    API->>DB: Create video record (status: uploading)
    API->>DB: Create transcode job (status: queued)
    API->>Redis: Enqueue transcode job
    API-->>Browser: Video ID + status URL

    Worker->>Redis: Claim job
    Redis-->>Worker: Job payload
    Worker->>DB: Update job (status: processing)
    Worker->>DB: Update video (status: transcoding)

    Worker->>Storage: Download raw file
    Storage-->>Worker: Raw video data

    Worker->>FFmpeg: Start transcode pipeline
    Note over Worker,FFmpeg: 1080p, 720p, 480p, 360p + thumbnails

    loop Each rendition
        FFmpeg-->>Worker: Progress stderr
        Worker->>DB: Update job progress %
    end

    FFmpeg-->>Worker: Transcode complete

    Worker->>Storage: Upload HLS manifests + segments
    Worker->>DB: Insert rendition records
    Worker->>DB: Update video (status: ready, duration)
    Worker->>DB: Update job (status: completed)
    Worker->>Redis: Ack job

    Browser->>API: Get watch page
    API->>DB: Query video + renditions
    API-->>Browser: HLS manifest URL + metadata
    Browser->>Browser: Shaka Player loads HLS