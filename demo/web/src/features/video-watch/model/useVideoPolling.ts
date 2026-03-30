import { useCallback, useEffect, useState } from 'react'
import { getVideo, getWatch } from '@/shared/api/video-api'
import { getApiBase } from '@/shared/config/env'
import type { Video, WatchResponse } from '@/entities/video'

export type UseVideoPollingResult = {
  video: Video | null
  watch: WatchResponse | null
  error: string | null
  loading: boolean
  refresh: () => Promise<void>
}

export function useVideoPolling(videoId: string | undefined, pollMs = 3000): UseVideoPollingResult {
  const [video, setVideo] = useState<Video | null>(null)
  const [watch, setWatch] = useState<WatchResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(Boolean(videoId))

  const refresh = useCallback(async () => {
    if (!videoId) return
    try {
      const base = getApiBase()
      const [v, w] = await Promise.all([getVideo(videoId, base), getWatch(videoId, base)])
      setVideo(v)
      setWatch(w)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }, [videoId])

  useEffect(() => {
    if (!videoId) {
      setVideo(null)
      setWatch(null)
      setError(null)
      setLoading(false)
      return
    }
    setLoading(true)
    void refresh()
    const h = window.setInterval(() => void refresh(), pollMs)
    return () => window.clearInterval(h)
  }, [videoId, pollMs, refresh])

  return { video, watch, error, loading, refresh }
}
