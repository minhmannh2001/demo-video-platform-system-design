import { useEffect, useRef } from 'react'
import Hls from 'hls.js'

type Props = {
  manifestUrl: string | null
}

export function VideoPlayer({ manifestUrl }: Props) {
  const ref = useRef<HTMLVideoElement>(null)

  useEffect(() => {
    const el = ref.current
    if (!el || !manifestUrl) return

    if (el.canPlayType('application/vnd.apple.mpegurl')) {
      el.src = manifestUrl
      return
    }

    if (Hls.isSupported()) {
      const hls = new Hls()
      hls.loadSource(manifestUrl)
      hls.attachMedia(el)
      return () => {
        hls.destroy()
      }
    }

    el.src = manifestUrl
  }, [manifestUrl])

  if (!manifestUrl) return null

  return (
    <video
      ref={ref}
      className="video-player"
      controls
      playsInline
      data-testid="video-player"
    />
  )
}
