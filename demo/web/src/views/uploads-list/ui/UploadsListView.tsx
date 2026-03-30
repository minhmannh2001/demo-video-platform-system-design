import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { buttonVariants } from '@/components/ui/button'
import { StatusBadge } from '@/entities/video'
import type { Video } from '@/entities/video'
import { listVideos } from '@/shared/api/video-api'
import { useToastOnError } from '@/shared/lib/useToastOnError'
import { PageMain } from '@/shared/ui/PageChrome'
import { AppHeader } from '@/widgets/app-header'
import { cn } from '@/lib/utils'

function shortId(id: string): string {
  return id.length <= 14 ? id : `${id.slice(0, 8)}…${id.slice(-4)}`
}

export function UploadsListView() {
  const [videos, setVideos] = useState<Video[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  useToastOnError(error)

  const load = useCallback(async () => {
    try {
      const list = await listVideos()
      setVideos(Array.isArray(list) ? list : [])
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load videos')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
    const t = window.setInterval(() => void load(), 5000)
    return () => window.clearInterval(t)
  }, [load])

  return (
    <div className="min-h-screen bg-background">
      <AppHeader />

      <PageMain>
        <div className="mb-6">
          <h1 className="text-2xl font-semibold tracking-tight">
            Upload queue
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            All videos from the API (same list as Home). Open a row for live
            encoding status.
          </p>
        </div>

        {loading && videos.length === 0 ? (
          <p className="text-muted-foreground">Loading…</p>
        ) : null}
        {error ? (
          <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-destructive text-sm">
            {error}
          </div>
        ) : null}

        {!loading && !error && videos.length === 0 ? (
          <div className="rounded-xl border border-dashed border-border bg-muted/20 px-6 py-12 text-center">
            <p className="text-muted-foreground">
              No videos in the catalog yet.
            </p>
            <Link
              to="/upload"
              className={cn(
                buttonVariants({ variant: 'outline' }),
                'mt-4 inline-flex',
              )}
            >
              Upload a video
            </Link>
          </div>
        ) : null}

        {videos.length > 0 ? (
          <div className="overflow-x-auto rounded-xl border border-border bg-card ring-1 ring-foreground/5">
            <table className="w-full min-w-[640px] border-collapse text-left text-sm">
              <thead>
                <tr className="border-b border-border bg-muted/40">
                  <th className="px-4 py-3 font-medium">Title</th>
                  <th className="px-4 py-3 font-medium">Video ID</th>
                  <th className="hidden px-4 py-3 font-medium sm:table-cell">
                    Uploader
                  </th>
                  <th className="px-4 py-3 font-medium">Status</th>
                  <th className="hidden px-4 py-3 font-medium md:table-cell">
                    Updated
                  </th>
                </tr>
              </thead>
              <tbody>
                {videos.map((v) => (
                  <tr
                    key={v.id}
                    className={cn(
                      'border-b border-border transition-colors last:border-b-0',
                      'hover:bg-muted/30',
                    )}
                  >
                    <td className="px-4 py-3">
                      <Link
                        to={`/uploads/${v.id}`}
                        className="font-medium text-foreground underline-offset-4 hover:underline"
                      >
                        {v.title || 'Untitled'}
                      </Link>
                    </td>
                    <td className="px-4 py-3">
                      <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">
                        {shortId(v.id)}
                      </code>
                    </td>
                    <td className="hidden max-w-[160px] truncate px-4 py-3 text-muted-foreground sm:table-cell">
                      {v.uploader || '—'}
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge status={v.status} />
                    </td>
                    <td className="hidden whitespace-nowrap px-4 py-3 text-muted-foreground md:table-cell">
                      {new Date(v.updated_at).toLocaleString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </PageMain>
    </div>
  )
}
