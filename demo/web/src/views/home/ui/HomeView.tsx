import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { buttonVariants } from '@/components/ui/button'
import { listVideos } from '@/shared/api/video-api'
import { cn } from '@/lib/utils'
import { StatusBadge } from '@/entities/video'
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
    <div className="mx-auto max-w-2xl px-6 py-6">
      <header className="mb-6 flex items-center justify-between gap-4">
        <h1 className="text-xl font-semibold">Video demo</h1>
        <Link to="/upload" className={cn(buttonVariants({ variant: 'outline' }))}>
          Upload
        </Link>
      </header>
      {loading ? <p className="text-muted-foreground">Loading…</p> : null}
      {err ? <p className="text-destructive">{err}</p> : null}
      {!loading && !err && items.length === 0 ? (
        <p className="text-muted-foreground">
          No videos yet.{' '}
          <Link to="/upload" className="text-primary underline-offset-4 hover:underline">
            Upload one
          </Link>
          .
        </p>
      ) : null}
      <ul className="m-0 list-none p-0">
        {items.map((v) => (
          <li
            key={v.id}
            className="flex items-center justify-between gap-3 border-b border-border py-2 last:border-b-0"
          >
            <Link to={`/watch/${v.id}`} className="text-primary underline-offset-4 hover:underline">
              {v.title}
            </Link>
            <StatusBadge status={v.status} />
          </li>
        ))}
      </ul>
    </div>
  )
}
