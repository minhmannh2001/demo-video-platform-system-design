import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { Search } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'
import { PageMain } from '@/shared/ui/PageChrome'
import { AppHeader } from '@/widgets/app-header'
import {
  getVideo,
  listVideos,
  searchPublishedVideos,
} from '@/shared/api/video-api'
import { useToastOnError } from '@/shared/lib/useToastOnError'
import { VideoCard } from '@/entities/video'
import type { Video, VideoSearchHit } from '@/entities/video'

const VIDEO_GRID_CLASS = 'grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3'
const SEARCH_DEBOUNCE_MS = 320

type SearchListRow = {
  video: Video
  titleHighlight?: string
  descriptionHighlight?: string
}

function firstSnippet(m?: string[]): string | undefined {
  if (!m || m.length === 0) return undefined
  const s = m[0]?.trim()
  return s || undefined
}

function placeholderVideo(hit: VideoSearchHit): Video {
  const short =
    hit.video_id.length > 8
      ? `${hit.video_id.slice(0, 8)}…`
      : hit.video_id
  return {
    id: hit.video_id,
    title: `Video ${short}`,
    description: '',
    uploader: '',
    raw_s3_key: '',
    status: 'ready',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  }
}

export function HomeView() {
  const [catalogItems, setCatalogItems] = useState<Video[]>([])
  const [catalogLoading, setCatalogLoading] = useState(true)
  const [catalogErr, setCatalogErr] = useState<string | null>(null)

  const [query, setQuery] = useState('')
  const [searchRows, setSearchRows] = useState<SearchListRow[]>([])
  const [searchTotal, setSearchTotal] = useState(0)
  const [searchLoading, setSearchLoading] = useState(false)
  const [searchErr, setSearchErr] = useState<string | null>(null)

  const searchSeq = useRef(0)

  const qTrim = query.trim()
  const inSearchMode = qTrim.length > 0

  const surfaceErr = inSearchMode ? searchErr : catalogErr
  useToastOnError(surfaceErr)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const v = await listVideos()
        if (!cancelled) setCatalogItems(Array.isArray(v) ? v : [])
      } catch (e) {
        if (!cancelled) {
          setCatalogErr(e instanceof Error ? e.message : 'Failed to load')
        }
      } finally {
        if (!cancelled) setCatalogLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (!qTrim) {
      searchSeq.current += 1
      setSearchRows([])
      setSearchTotal(0)
      setSearchErr(null)
      setSearchLoading(false)
      return
    }

    const runId = ++searchSeq.current
    setSearchLoading(true)
    setSearchErr(null)

    const t = window.setTimeout(() => {
      ;(async () => {
        try {
          const res = await searchPublishedVideos(qTrim, {
            size: 24,
            highlight: true,
          })
          if (runId !== searchSeq.current) return

          setSearchTotal(res.total)
          const enriched = await Promise.all(
            res.hits.map(async (hit) => {
              try {
                return await getVideo(hit.video_id)
              } catch {
                return placeholderVideo(hit)
              }
            }),
          )
          if (runId !== searchSeq.current) return
          const rows: SearchListRow[] = res.hits.map((hit, i) => ({
            video: enriched[i]!,
            titleHighlight: firstSnippet(hit.highlights?.title),
            descriptionHighlight: firstSnippet(hit.highlights?.description),
          }))
          setSearchRows(rows)
        } catch (e) {
          if (runId !== searchSeq.current) return
          setSearchRows([])
          setSearchTotal(0)
          setSearchErr(
            e instanceof Error ? e.message : 'Search failed',
          )
        } finally {
          if (runId === searchSeq.current) setSearchLoading(false)
        }
      })()
    }, SEARCH_DEBOUNCE_MS)

    return () => window.clearTimeout(t)
  }, [qTrim])

  const showCatalogSkeleton = !inSearchMode && catalogLoading
  const showSearchSkeleton = inSearchMode && searchLoading
  const showSkeleton = showCatalogSkeleton || showSearchSkeleton

  return (
    <div className="min-h-screen bg-background">
      <AppHeader />

      <PageMain>
        <div className="mb-8">
          <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">
            Videos
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Browse uploads — open a card to watch. Search uses the catalog index
            (public, ready videos).
          </p>
          <div className="relative mt-5 max-w-xl">
            <Search
              className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
              aria-hidden
            />
            <Input
              type="search"
              enterKeyHint="search"
              placeholder="Search title & description…"
              className="pl-9"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              aria-label="Search catalog"
            />
          </div>
        </div>

        {showSkeleton ? (
          <div
            className={VIDEO_GRID_CLASS}
            role="status"
            aria-live="polite"
            aria-label={inSearchMode ? 'Searching' : 'Loading videos'}
          >
            {Array.from({ length: 6 }).map((_, i) => (
              <Card
                key={i}
                className="h-full select-none gap-0 overflow-hidden py-0 pointer-events-none"
                aria-hidden
              >
                <div className="aspect-video animate-pulse bg-muted/60" />
                <CardContent className="flex flex-col gap-3 px-5 pb-5 pt-4">
                  <div className="space-y-2">
                    <div className="h-4 w-[90%] animate-pulse rounded-md bg-muted" />
                    <div className="h-4 w-[65%] animate-pulse rounded-md bg-muted" />
                  </div>
                  <div className="h-3.5 w-1/2 animate-pulse rounded-md bg-muted" />
                  <div className="h-6 w-[4.5rem] animate-pulse rounded-full bg-muted" />
                  <div className="border-t border-border/60 pt-3">
                    <div className="space-y-2">
                      <div className="h-3.5 w-full animate-pulse rounded-md bg-muted" />
                      <div className="h-3.5 w-[85%] animate-pulse rounded-md bg-muted" />
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        ) : null}

        {surfaceErr ? (
          <p className="text-destructive" role="alert">
            {surfaceErr}
          </p>
        ) : null}

        {!showSkeleton &&
        !surfaceErr &&
        inSearchMode &&
        searchRows.length === 0 ? (
          <div className="rounded-xl border border-dashed border-border bg-muted/20 px-6 py-16 text-center">
            <p className="text-muted-foreground">
              No matches for &quot;{qTrim}&quot;.
            </p>
          </div>
        ) : null}

        {!showSkeleton &&
        !surfaceErr &&
        !inSearchMode &&
        catalogItems.length === 0 ? (
          <div className="rounded-xl border border-dashed border-border bg-muted/20 px-6 py-16 text-center">
            <p className="text-muted-foreground">
              No videos yet.{' '}
              <Link
                to="/upload"
                className="font-medium text-primary underline-offset-4 hover:underline"
              >
                Upload one
              </Link>{' '}
              to see it here.
            </p>
          </div>
        ) : null}

        {!showSkeleton &&
        !surfaceErr &&
        inSearchMode &&
        searchRows.length > 0 ? (
          <>
            <p className="mb-4 text-sm text-muted-foreground">
              {searchTotal} result{searchTotal === 1 ? '' : 's'} for &quot;
              {qTrim}&quot;
            </p>
            <ul className={cn('m-0 list-none p-0', VIDEO_GRID_CLASS)}>
              {searchRows.map((row) => (
                <li key={row.video.id}>
                  <VideoCard
                    video={row.video}
                    titleHighlight={row.titleHighlight}
                    descriptionHighlight={row.descriptionHighlight}
                  />
                </li>
              ))}
            </ul>
          </>
        ) : null}

        {!showSkeleton &&
        !surfaceErr &&
        !inSearchMode &&
        catalogItems.length > 0 ? (
          <ul className={cn('m-0 list-none p-0', VIDEO_GRID_CLASS)}>
            {catalogItems.map((v) => (
              <li key={v.id}>
                <VideoCard video={v} />
              </li>
            ))}
          </ul>
        ) : null}
      </PageMain>
    </div>
  )
}
