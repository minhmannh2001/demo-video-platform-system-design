import { useCallback, useEffect, useRef, useState } from 'react'
import type { WatchResponse } from '@/entities/video'
import { getWebSocketUrl } from '@/shared/config/env'
import { useVideoPolling, type UseVideoPollingResult } from './useVideoPolling'

function parseServerMessage(raw: unknown): { type?: string; payload?: unknown } {
  if (raw === null || typeof raw !== 'object') return {}
  const o = raw as Record<string, unknown>
  return { type: typeof o.type === 'string' ? o.type : undefined, payload: o.payload }
}

function readStringArray(v: unknown): string[] | undefined {
  if (!Array.isArray(v)) return undefined
  const out: string[] = []
  for (const x of v) {
    if (typeof x === 'string') out.push(x)
  }
  return out.length ? out : undefined
}

function readRenditions(v: unknown): WatchResponse['renditions'] {
  if (!Array.isArray(v)) return undefined
  const out: NonNullable<WatchResponse['renditions']> = []
  for (const item of v) {
    if (item === null || typeof item !== 'object') continue
    const r = item as Record<string, unknown>
    const quality = typeof r.quality === 'string' ? r.quality : ''
    const playlist_url =
      typeof r.playlist_url === 'string' ? r.playlist_url : ''
    if (!quality || !playlist_url) continue
    out.push({
      quality,
      playlist_url,
      width: typeof r.width === 'number' ? r.width : undefined,
      height: typeof r.height === 'number' ? r.height : undefined,
      bitrate: typeof r.bitrate === 'number' ? r.bitrate : undefined,
    })
  }
  return out.length ? out : undefined
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
    thumbnail_url:
      typeof o.thumbnail_url === 'string' ? o.thumbnail_url : undefined,
    qualities: readStringArray(o.qualities),
    renditions: readRenditions(o.renditions),
    message: typeof o.message === 'string' ? o.message : undefined,
  }
}

/**
 * Watch + upload-detail: HTTP polling + WebSocket `video.updated` for the same video_id.
 * When the socket is up, polling slows down as a fallback; push updates merge into watch and trigger one REST refresh so `video` (Mongo) stays in sync with status badges.
 */
export function useVideoWatchFeed(
  videoId: string | undefined,
): UseVideoPollingResult {
  const [wsLive, setWsLive] = useState(false)
  const pollMs = wsLive ? 30_000 : 3000
  const poll = useVideoPolling(videoId, pollMs)
  const [wsWatch, setWsWatch] = useState<WatchResponse | null>(null)
  const refreshRef = useRef(poll.refresh)
  refreshRef.current = poll.refresh

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
        if (w) {
          setWsWatch(w)
          void refreshRef.current()
        }
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
