import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { buttonVariants } from '@/components/ui/button'
import { listVideos } from '@/shared/api/video-api'
import { cn } from '@/lib/utils'
import { VideoCard } from '@/entities/video'
import type { Video } from '@/entities/video'

export function HomeView() {
  const [items, setItems] = useState<Video[]>([])
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const v = await listVideos()
        if (!cancelled) setItems(Array.isArray(v) ? v : [])
      } catch (e) {
        if (!cancelled) setErr(e instanceof Error ? e.message : 'Failed to load')
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <div className="mx-auto max-w-6xl px-4 py-8 sm:px-6">
      <header className="mb-8 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">Videos</h1>
          <p className="mt-1 text-sm text-muted-foreground">Browse uploads — open a card to watch.</p>
        </div>
        <Link to="/upload" className={cn(buttonVariants({ variant: 'default' }), 'shrink-0')}>
          Upload
        </Link>
      </header>

      {loading ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <div
              key={i}
              className="overflow-hidden rounded-xl border border-border bg-card ring-1 ring-foreground/5"
            >
              <div className="aspect-video animate-pulse bg-muted" />
              <div className="space-y-2 p-4">
                <div className="h-4 w-3/4 animate-pulse rounded bg-muted" />
                <div className="h-3 w-1/2 animate-pulse rounded bg-muted" />
              </div>
            </div>
          ))}
        </div>
      ) : null}

      {err ? <p className="text-destructive">{err}</p> : null}

      {!loading && !err && items.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border bg-muted/30 px-6 py-16 text-center">
          <p className="text-muted-foreground">
            No videos yet.{' '}
            <Link to="/upload" className="font-medium text-primary underline-offset-4 hover:underline">
              Upload one
            </Link>{' '}
            to see it here.
          </p>
        </div>
      ) : null}

      {!loading && !err && items.length > 0 ? (
        <ul className="m-0 grid list-none grid-cols-1 gap-5 p-0 sm:grid-cols-2 lg:grid-cols-3">
          {items.map((v) => (
            <li key={v.id}>
              <VideoCard video={v} />
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  )
}
