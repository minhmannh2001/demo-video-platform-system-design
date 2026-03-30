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

/** Maps internal / technical messages to a short, user-facing string. */
function playbackErrorForUser(technical: string): string {
  const t = technical.toLowerCase()
  if (t.includes('media_err_network') || t.includes('networkerror') || t.includes('fragload'))
    return 'Could not load playback data. Check your connection and refresh the page.'
  if (t.includes('media_err_decode') || t.includes('bufferappend'))
    return 'This video could not be decoded. Refresh the page or try another video.'
  if (
    t.includes('media_err_src_not_supported') ||
    t.includes('media_element_error') ||
    t.includes('format error')
  ) {
    return 'This stream cannot be played in your browser (unsupported format or codec). Try another browser or refresh the page.'
  }
  if (t.includes('media_err_aborted'))
    return 'Playback was interrupted. Try again.'
  if (t.includes('manifest') || t.includes('level') || t.includes('playlist'))
    return 'Could not load the playlist. Try refreshing the page.'
  if (t.includes('http') && /\b(401|403|404|500|502|503)\b/.test(t))
    return 'The server rejected the request or the video was not found. Try again later.'
  return 'Playback failed. Refresh the page or try again later.'
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

  useEffect(() => {
    if (error) {
      console.warn('[VideoPlayer]', error)
    }
  }, [error])

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
          {playbackErrorForUser(error)}
        </p>
      ) : null}
    </div>
  )
}
