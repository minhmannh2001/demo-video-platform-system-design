import Hls from 'hls.js'
import { useEffect, useRef, useState } from 'react'

type Props = {
  manifestUrl: string | null
}

function describeVideoError(el: HTMLVideoElement): string {
  const code = el.error?.code
  const map: Record<number, string> = {
    1: 'MEDIA_ERR_ABORTED',
    2: 'MEDIA_ERR_NETWORK',
    3: 'MEDIA_ERR_DECODE',
    4: 'MEDIA_ERR_SRC_NOT_SUPPORTED',
  }
  const label = code != null ? map[code] ?? `code ${code}` : 'unknown'
  return `${label}${el.error?.message ? `: ${el.error.message}` : ''}`
}

export function VideoPlayer({ manifestUrl }: Props) {
  const ref = useRef<HTMLVideoElement>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const el = ref.current
    if (!el || !manifestUrl) return

    setError(null)

    const onVideoError = () => {
      setError(describeVideoError(el))
    }
    el.addEventListener('error', onVideoError)

    if (el.canPlayType('application/vnd.apple.mpegurl')) {
      el.src = manifestUrl
      return () => {
        el.removeEventListener('error', onVideoError)
        el.removeAttribute('src')
        el.load()
      }
    }

    if (Hls.isSupported()) {
      const hls = new Hls({
        enableWorker: true,
      })
      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (!data.fatal) return
        const detail =
          typeof data.details === 'string' ? data.details : `${data.type ?? 'unknown'}`
        setError(`HLS: ${detail}${data.response?.code ? ` (HTTP ${data.response.code})` : ''}`)
      })
      hls.loadSource(manifestUrl)
      hls.attachMedia(el)
      return () => {
        el.removeEventListener('error', onVideoError)
        hls.destroy()
      }
    }

    el.src = manifestUrl
    return () => {
      el.removeEventListener('error', onVideoError)
      el.removeAttribute('src')
      el.load()
    }
  }, [manifestUrl])

  if (!manifestUrl) return null

  return (
    <div className="space-y-2">
      <video
        ref={ref}
        className="w-full max-w-full rounded-lg bg-black"
        controls
        playsInline
        crossOrigin="anonymous"
        data-testid="video-player"
      />
      {error ? (
        <p className="text-destructive text-sm" role="alert" data-testid="video-player-error">
          {error}. Check DevTools → Network → failed requests to <code>/stream/</code>. CORS:{' '}
          <code className="rounded bg-muted px-1 py-0.5 text-xs">CORS_ORIGINS</code> must include your
          exact page origin (e.g. <code className="rounded bg-muted px-1 py-0.5 text-xs">http://127.0.0.1:5173</code>).
        </p>
      ) : null}
    </div>
  )
}
