export type VideoStatus = 'processing' | 'ready' | 'failed' | string

export type Video = {
  id: string
  title: string
  description: string
  uploader: string
  raw_s3_key: string
  encoded_prefix?: string
  status: VideoStatus
  duration_sec?: number
  created_at: string
  updated_at: string
}

export type WatchResponse = {
  video_id: string
  status: string
  manifest_url?: string
  message?: string
}

export type UploadResponse = {
  id: string
  status: string
}
