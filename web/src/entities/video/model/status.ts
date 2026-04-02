import type { VideoStatus, WatchResponse } from './types'

/**
 * Prefer terminal playback status from GET /watch when present.
 * GET /videos can be briefly stale (e.g. Redis cache) while /watch reads Mongo directly.
 */
export function effectiveVideoStatus(
  videoStatus: VideoStatus,
  watch: Pick<WatchResponse, 'status'> | null | undefined,
): VideoStatus {
  const w = watch?.status
  if (w === 'failed' || w === 'ready') return w
  return videoStatus
}

export function isProcessing(status: VideoStatus): boolean {
  return status === 'processing'
}

export function isReady(status: VideoStatus): boolean {
  return status === 'ready'
}

export function isFailed(status: VideoStatus): boolean {
  return status === 'failed'
}
