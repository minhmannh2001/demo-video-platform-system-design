import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Card, CardContent } from '@/components/ui/card'
import { cn } from '@/lib/utils'
import { PageMain } from '@/shared/ui/PageChrome'
import { AppHeader } from '@/widgets/app-header'
import { listVideos } from '@/shared/api/video-api'
import { useToastOnError } from '@/shared/lib/useToastOnError'
import { VideoCard } from '@/entities/video'
import type { Video } from '@/entities/video'

const VIDEO_GRID_CLASS = 'grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3'

export function HomeView() {
  const [items, setItems] = useState<Video[]>([])
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  useToastOnError(err)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const v = await listVideos()
        if (!cancelled) setItems(Array.isArray(v) ? v : [])
      } catch (e) {
        if (!cancelled) {
          setErr(e instanceof Error ? e.message : 'Failed to load')
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <div className="min-h-screen bg-background">
      <AppHeader />

      <PageMain>
        <div className="mb-8">
          <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">
            Videos
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Browse uploads — open a card to watch.
          </p>
        </div>

        {loading ? (
          <div
            className={VIDEO_GRID_CLASS}
            role="status"
            aria-live="polite"
            aria-label="Loading videos"
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

        {err ? <p className="text-destructive">{err}</p> : null}

        {!loading && !err && items.length === 0 ? (
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

        {!loading && !err && items.length > 0 ? (
          <ul className={cn('m-0 list-none p-0', VIDEO_GRID_CLASS)}>
            {items.map((v) => (
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
