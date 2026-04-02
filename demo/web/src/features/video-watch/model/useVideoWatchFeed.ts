import { useCallback, useEffect, useState } from 'react'
import type { WatchResponse } from '@/entities/video'
import { getWebSocketUrl } from '@/shared/config/env'
import { useVideoPolling, type UseVideoPollingResult } from './useVideoPolling'

function parseServerMessage(raw: unknown): { type?: string; payload?: unknown } {
  if (raw === null || typeof raw !== 'object') return {}
  const o = raw as Record<string, unknown>
  return { type: typeof o.type === 'string' ? o.type : undefined, payload: o.payload }
}

function payloadToWatch(p: unknown, videoId: string): WatchResponse | null {
  if (p === null || typeof p !== 'object') return null
  const o = p as Record<string, unknown>
  const vid = typeof o.video_id === 'string' ? o.video_id : ''
  if (vid !== videoId) return null
  return {
    video_id: vid,
    status: typeof o.status === 'string' ? o.status : '',
    manifest_url:
      typeof o.manifest_url === 'string' ? o.manifest_url : undefined,
    message: typeof o.message === 'string' ? o.message : undefined,
  }
}

/**
 * Watch page: HTTP polling + WebSocket `video.updated` for the same video_id.
 * When the socket is up, polling slows down as a fallback; push updates override watch state until the next full refresh.
 */
export function useVideoWatchFeed(
  videoId: string | undefined,
): UseVideoPollingResult {
  const [wsLive, setWsLive] = useState(false)
  const pollMs = wsLive ? 30_000 : 3000
  const poll = useVideoPolling(videoId, pollMs)
  const [wsWatch, setWsWatch] = useState<WatchResponse | null>(null)

  useEffect(() => {
    setWsWatch(null)
  }, [videoId])

  useEffect(() => {
    if (!videoId) {
      setWsLive(false)
      return
    }
    const url = getWebSocketUrl()
    let closed = false
    const socket = new WebSocket(url)

    socket.onopen = () => {
      if (closed) return
      setWsLive(true)
      socket.send(
        JSON.stringify({ type: 'subscribe', v: 1, video_id: videoId }),
      )
    }

    socket.onmessage = (ev) => {
      if (closed || typeof ev.data !== 'string') return
      try {
        const msg = JSON.parse(ev.data) as unknown
        const { type, payload } = parseServerMessage(msg)
        if (type !== 'video.updated') return
        const w = payloadToWatch(payload, videoId)
        if (w) setWsWatch(w)
      } catch {
        /* ignore */
      }
    }

    socket.onerror = () => {
      /* browser may also close */
    }

    socket.onclose = () => {
      if (!closed) setWsLive(false)
    }

    return () => {
      closed = true
      socket.close()
      setWsLive(false)
    }
  }, [videoId])

  const refresh = useCallback(async () => {
    setWsWatch(null)
    await poll.refresh()
  }, [poll])

  const watch = wsWatch ?? poll.watch

  return {
    video: poll.video,
    watch,
    error: poll.error,
    loading: poll.loading,
    refresh,
  }
}
