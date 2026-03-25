import type { VideoStatus } from './types'

export function isProcessing(status: VideoStatus): boolean {
  return status === 'processing'
}

export function isReady(status: VideoStatus): boolean {
  return status === 'ready'
}

export function isFailed(status: VideoStatus): boolean {
  return status === 'failed'
}
