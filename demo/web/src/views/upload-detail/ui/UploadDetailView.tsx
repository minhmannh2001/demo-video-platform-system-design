import { Link, useParams } from 'react-router-dom'
import { isFailed, isProcessing, isReady, StatusBadge } from '@/entities/video'
import { useVideoPolling } from '@/features/video-watch'
import { useToastOnError } from '@/shared/lib/useToastOnError'
import { PageMain } from '@/shared/ui/PageChrome'
import { AppHeader } from '@/widgets/app-header'
import { buttonVariants } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export function UploadDetailView() {
  const { id } = useParams<{ id: string }>()
  const { video, watch, error, loading } = useVideoPolling(id)
  useToastOnError(error)

  return (
    <div className="min-h-screen bg-background">
      <AppHeader />

      <PageMain>
        <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <p className="text-sm">
              <Link
                to="/uploads"
                className="text-muted-foreground underline-offset-4 transition-colors hover:text-foreground hover:underline"
              >
                ← Upload queue
              </Link>
            </p>
            <h1 className="mt-1 text-2xl font-semibold tracking-tight">
              Upload status
            </h1>
            <p className="mt-1 text-sm text-muted-foreground">
              {video?.title ?? id ?? 'Loading…'}
            </p>
          </div>
          {video ? (
            <div className="flex flex-wrap items-center gap-2">
              <StatusBadge status={video.status} />
              {isReady(video.status) && watch?.manifest_url ? (
                <Link
                  to={`/watch/${video.id}`}
                  className={cn(
                    buttonVariants({ variant: 'default', size: 'default' }),
                    'inline-flex',
                  )}
                >
                  Open player
                </Link>
              ) : null}
            </div>
          ) : null}
        </div>

        {loading ? <p className="text-muted-foreground">Loading…</p> : null}
        {error ? <p className="text-destructive">{error}</p> : null}

        {video && !loading ? (
          <div className="rounded-xl border border-border bg-card p-6 ring-1 ring-foreground/5">
            <dl className="grid gap-4 text-sm sm:grid-cols-2">
              <div>
                <dt className="text-muted-foreground">Video ID</dt>
                <dd className="mt-1 font-mono text-xs break-all">{video.id}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Last updated</dt>
                <dd className="mt-1">
                  {new Date(video.updated_at).toLocaleString()}
                </dd>
              </div>
            </dl>

            <div className="mt-6 border-t border-border pt-6">
              {isProcessing(video.status) ? (
                <p className="text-muted-foreground">
                  Encoding in progress… This page refreshes automatically.
                </p>
              ) : null}
              {isFailed(video.status) ? (
                <p className="text-destructive">
                  Encoding failed. Check the worker or source file.
                </p>
              ) : null}
              {isReady(video.status) && watch?.manifest_url ? (
                <p className="text-muted-foreground">
                  Ready to play. Use <strong>Open player</strong> above or{' '}
                  <Link
                    to={`/watch/${video.id}`}
                    className="font-medium text-foreground underline-offset-4 hover:underline"
                  >
                    watch page
                  </Link>
                  .
                </p>
              ) : null}
              {isReady(video.status) && !watch?.manifest_url ? (
                <p className="text-muted-foreground">
                  Video is ready; manifest is still loading…
                </p>
              ) : null}
            </div>
          </div>
        ) : null}
      </PageMain>
    </div>
  )
}
