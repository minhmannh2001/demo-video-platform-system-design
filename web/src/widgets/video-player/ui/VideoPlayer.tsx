import Hls from 'hls.js'
import { useCallback, useEffect, useRef, useState } from 'react'

export type VideoPlayerRendition = {
  quality: string
  playlist_url: string
}

type Props = {
  manifestUrl: string | null
  /** From GET /watch — enables quality UI on Safari (native HLS) and labels when hls.js levels lack height. */
  renditions?: VideoPlayerRendition[]
}

type QualityOption = {
  value: string
  label: string
  /** hls.js output stream index; null for Auto or native-only */
  level: number | null
  playlistUrl?: string
}

function describeVideoError(el: HTMLVideoElement): string {
  const code = el.error?.code
  const map: Record<number, string> = {
    1: 'MEDIA_ERR_ABORTED',
    2: 'MEDIA_ERR_NETWORK',
    3: 'MEDIA_ERR_DECODE',
    4: 'MEDIA_ERR_SRC_NOT_SUPPORTED',
  }
  const label = code != null ? (map[code] ?? `code ${code}`) : 'unknown'
  return `${label}${el.error?.message ? `: ${el.error.message}` : ''}`
}

function playbackErrorForUser(technical: string): string {
  const t = technical.toLowerCase()
  if (
    t.includes('media_err_network') ||
    t.includes('networkerror') ||
    t.includes('fragload')
  )
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

function optionsFromRenditions(
  r: VideoPlayerRendition[],
  levelByIndex: number[] | null,
): QualityOption[] {
  const auto: QualityOption = {
    value: 'auto',
    label: 'Auto',
    level: null,
  }
  const rest = r.map((x, i) => ({
    value: x.quality,
    label: x.quality,
    level: levelByIndex != null ? (levelByIndex[i] ?? i) : null,
    playlistUrl: x.playlist_url,
  }))
  return [auto, ...rest]
}

function optionsFromHlsLevels(
  levels: Array<{ height?: number }>,
): QualityOption[] {
  const byHeight = new Map<number, { index: number; height: number }>()
  levels.forEach((lvl, idx) => {
    const h = lvl.height
    if (!h) return
    if (!byHeight.has(h)) byHeight.set(h, { index: idx, height: h })
  })
  const adaptive = Array.from(byHeight.values())
    .sort((a, b) => a.height - b.height)
    .map((v) => ({
      value: `${v.height}p`,
      label: `${v.height}p`,
      level: v.index,
    }))
  return [{ value: 'auto', label: 'Auto', level: null }, ...adaptive]
}

export function VideoPlayer({ manifestUrl, renditions }: Props) {
  const ref = useRef<HTMLVideoElement>(null)
  const hlsRef = useRef<Hls | null>(null)
  /** 'hls' | 'native' — set once per manifest lifecycle */
  const playbackRef = useRef<'hls' | 'native' | null>(null)
  const manifestRef = useRef<string | null>(null)
  const qualityOptionsRef = useRef<QualityOption[]>([])

  const [error, setError] = useState<string | null>(null)
  const [quality, setQuality] = useState<string>('auto')
  const [qualityOptions, setQualityOptions] = useState<QualityOption[]>([])

  qualityOptionsRef.current = qualityOptions

  const applyQuality = useCallback(
    (value: string) => {
      setQuality(value)
      const el = ref.current
      const manifest = manifestRef.current
      const mode = playbackRef.current
      const opts = qualityOptionsRef.current

      if (mode === 'native' && el && manifest) {
        if (value === 'auto') {
          el.src = manifest
          void el.load()
          return
        }
        const picked = opts.find((q) => q.value === value)
        if (picked?.playlistUrl) {
          el.src = picked.playlistUrl
          void el.load()
        }
        return
      }

      const hls = hlsRef.current
      if (!hls) return
      if (value === 'auto') {
        hls.currentLevel = -1
        return
      }
      const picked = opts.find((q) => q.value === value)
      if (!picked || picked.level == null) return
      hls.nextLevel = picked.level
      hls.currentLevel = picked.level
    },
    [],
  )

  useEffect(() => {
    const el = ref.current
    if (!el || !manifestUrl) return

    playbackRef.current = null
    manifestRef.current = manifestUrl
    setError(null)
    setQuality('auto')

    const r = renditions?.filter((x) => x.quality && x.playlist_url) ?? []
    const canShowFromApi = r.length >= 2
    if (canShowFromApi) {
      setQualityOptions(optionsFromRenditions(r, null))
    } else {
      setQualityOptions([])
    }

    const onVideoError = () => {
      setError(describeVideoError(el))
    }
    el.addEventListener('error', onVideoError)

    if (el.canPlayType('application/vnd.apple.mpegurl')) {
      playbackRef.current = 'native'
      el.src = manifestUrl
      return () => {
        el.removeEventListener('error', onVideoError)
        el.removeAttribute('src')
        el.load()
      }
    }

    if (Hls.isSupported()) {
      playbackRef.current = 'hls'
      const hls = new Hls({ enableWorker: true })
      hlsRef.current = hls

      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        if (canShowFromApi && hls.levels.length >= 2) {
          const n = Math.min(r.length, hls.levels.length)
          const sortedIdx = hls.levels
            .map((_, i) => i)
            .sort(
              (a, b) =>
                (hls.levels[a].height ?? 0) - (hls.levels[b].height ?? 0),
            )
          const levelByIndex: number[] = []
          for (let i = 0; i < n; i++) levelByIndex.push(sortedIdx[i] ?? i)
          const rUsed = r.slice(0, n)
          if (rUsed.length >= 2) {
            setQualityOptions(optionsFromRenditions(rUsed, levelByIndex))
            return
          }
        }
        const built = optionsFromHlsLevels(hls.levels)
        setQualityOptions(built.length > 1 ? built : [])
      })

      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (!data.fatal) return
        const detail =
          typeof data.details === 'string'
            ? data.details
            : `${data.type ?? 'unknown'}`
        setError(
          `HLS: ${detail}${data.response?.code ? ` (HTTP ${data.response.code})` : ''}`,
        )
      })
      hls.loadSource(manifestUrl)
      hls.attachMedia(el)
      return () => {
        el.removeEventListener('error', onVideoError)
        hlsRef.current = null
        hls.destroy()
      }
    }

    playbackRef.current = 'native'
    el.src = manifestUrl
    return () => {
      el.removeEventListener('error', onVideoError)
      el.removeAttribute('src')
      el.load()
    }
  }, [manifestUrl, renditions])

  useEffect(() => {
    if (error) console.warn('[VideoPlayer]', error)
  }, [error])

  if (!manifestUrl) return null

  const showQuality = qualityOptions.length > 1

  return (
    <div className="flex flex-col gap-0">
      <video
        ref={ref}
        className="w-full max-w-full bg-black"
        controls
        playsInline
        crossOrigin="anonymous"
        data-testid="video-player"
      />
      {showQuality ? (
        <div
          className="flex flex-wrap items-center gap-2 border-t border-border/50 bg-muted/40 px-3 py-2.5"
          data-testid="quality-toolbar"
          role="group"
          aria-label="Playback quality"
        >
          <span className="text-xs font-medium text-muted-foreground">
            Quality
          </span>
          <div className="flex flex-wrap gap-1.5">
            {qualityOptions.map((opt) => {
              const active = quality === opt.value
              return (
                <button
                  key={opt.value}
                  type="button"
                  data-testid={`quality-${opt.value}`}
                  aria-pressed={active}
                  className={
                    active
                      ? 'rounded-md border border-primary bg-primary/15 px-3 py-1.5 text-xs font-medium text-foreground shadow-sm'
                      : 'rounded-md border border-border bg-background px-3 py-1.5 text-xs font-medium text-muted-foreground transition-colors hover:border-primary/40 hover:text-foreground'
                  }
                  onClick={() => applyQuality(opt.value)}
                >
                  {opt.label}
                </button>
              )
            })}
          </div>
        </div>
      ) : null}
      {error ? (
        <p
          className="text-destructive text-sm"
          role="alert"
          data-testid="video-player-error"
        >
          {playbackErrorForUser(error)}
        </p>
      ) : null}
    </div>
  )
}
