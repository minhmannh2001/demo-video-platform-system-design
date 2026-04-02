export type VideoStatus = 'processing' | 'ready' | 'failed' | string

export type VideoVisibility = 'public' | 'unlisted' | 'private' | string

export type Video = {
  id: string
  title: string
  description: string
  uploader: string
  /** Catalog/search visibility; omitted in older API responses defaults to public. */
  visibility?: VideoVisibility
  raw_s3_key: string
  encoded_prefix?: string
  /** Poster / thumbnail URL when API provides one (optional). */
  thumbnail_url?: string
  /** Ready-state labels for UI, e.g. ["360p","720p","auto"]. */
  qualities?: string[]
  /** Per-variant playlists when API enriches ready videos. */
  playback_renditions?: WatchPlaybackRendition[]
  status: VideoStatus
  duration_sec?: number
  created_at: string
  updated_at: string
}

export type WatchPlaybackRendition = {
  quality: string
  width?: number
  height?: number
  bitrate?: number
  playlist_url: string
}

export type WatchResponse = {
  video_id: string
  status: string
  manifest_url?: string
  thumbnail_url?: string
  qualities?: string[]
  renditions?: WatchPlaybackRendition[]
  message?: string
}

export type UploadResponse = {
  id: string
  status: string
}

/** One hit from GET /videos/search (Elasticsearch). */
export type VideoSearchHit = {
  video_id: string
  score?: number
  highlights?: Record<string, string[]>
}

export type VideoSearchResponse = {
  total: number
  from: number
  size: number
  hits: VideoSearchHit[]
}
