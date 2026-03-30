import { Link, useParams } from 'react-router-dom'
import {
  formatDurationSec,
  formatPublishedAt,
  isFailed,
  isProcessing,
  isReady,
  StatusBadge,
} from '@/entities/video'
import { useVideoPolling } from '@/features/video-watch'
import { VideoPlayer } from '@/widgets/video-player'
import { PageMain } from '@/shared/ui/PageChrome'
import { AppHeader } from '@/widgets/app-header'
import { cn } from '@/lib/utils'

const panelClass =
  'rounded-xl border border-border bg-card p-6 ring-1 ring-foreground/5'

/** User-facing line for `watch.status` from the playback endpoint (not raw API jargon). */
function playbackStatusLabel(status: string): string {
  const s = status.toLowerCase()
  if (s === 'ready') return 'Ready to play'
  if (s === 'processing') return 'Preparing playback'
  if (s === 'failed') return 'Playback unavailable'
  return status
}

export function VideoWatchView() {
  const { id } = useParams<{ id: string }>()
  const { video, watch, error, loading } = useVideoPolling(id)

  const title = video?.title ?? (loading ? 'Watch' : (id ?? 'Watch'))
  const desc = video?.description?.trim()

  return (
    <div className="min-h-screen bg-background">
      <AppHeader />

      <PageMain>
        <div className="mb-8 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <p className="text-sm">
              <Link
                to="/"
                className="text-muted-foreground underline-offset-4 transition-colors hover:text-foreground hover:underline"
              >
                ← All videos
              </Link>
            </p>
            <h1 className="mt-2 text-2xl font-semibold tracking-tight sm:text-3xl">
              {title}
            </h1>
            {video ? (
              <div className="mt-4 flex flex-col gap-4 sm:flex-row sm:flex-wrap sm:items-start sm:gap-x-10 sm:gap-y-3">
                <div>
                  <p className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                    Uploaded by
                  </p>
                  <p className="mt-1 text-sm font-medium text-foreground">
                    {video.uploader || 'Unknown'}
                  </p>
                </div>
                <div>
                  <p className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                    Published
                  </p>
                  <p className="mt-1 text-sm text-foreground">
                    <time dateTime={video.created_at}>
                      {formatPublishedAt(video.created_at)}
                    </time>
                  </p>
                </div>
                {video.duration_sec != null && video.duration_sec >= 0 ? (
                  <div>
                    <p className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                      Length
                    </p>
                    <p className="mt-1 text-sm tabular-nums text-foreground">
                      {formatDurationSec(video.duration_sec)}
                    </p>
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>
          {video ? (
            <div className="flex flex-wrap items-center gap-2">
              <StatusBadge status={video.status} />
            </div>
          ) : null}
        </div>

        {loading ? <p className="text-muted-foreground">Loading…</p> : null}
        {error ? <p className="text-destructive">{error}</p> : null}

        {video && !loading ? (
          <div className="grid gap-6 lg:grid-cols-5 lg:gap-8">
            <div className="space-y-6 lg:col-span-3">
              {video && isReady(video.status) && watch?.manifest_url ? (
                <div
                  className={cn(
                    panelClass,
                    'overflow-hidden p-0',
                    'bg-black ring-foreground/20',
                  )}
                >
                  <VideoPlayer manifestUrl={watch.manifest_url} />
                </div>
              ) : null}

              {video && isProcessing(video.status) ? (
                <div className={panelClass}>
                  <p className="text-muted-foreground">
                    Encoding in progress… This page refreshes automatically when
                    the video is ready.
                  </p>
                </div>
              ) : null}

              {video && isFailed(video.status) ? (
                <div className={panelClass}>
                  <p className="text-destructive">
                    Encoding failed. The source file or worker may need
                    attention.
                  </p>
                </div>
              ) : null}

              {video && isReady(video.status) && !watch?.manifest_url ? (
                <div className={panelClass}>
                  <p className="text-muted-foreground">
                    Video is ready, but the playback manifest is still loading…
                  </p>
                </div>
              ) : null}
            </div>

            <aside className="lg:col-span-2">
              <div className={panelClass}>
                <h2 className="text-sm font-medium text-foreground">About</h2>
                {desc ? (
                  <p className="mt-3 whitespace-pre-wrap text-sm leading-relaxed text-muted-foreground">
                    {desc}
                  </p>
                ) : (
                  <p className="mt-3 text-sm italic text-muted-foreground">
                    No description.
                  </p>
                )}

                <dl className="mt-6 grid gap-4 border-t border-border pt-6 text-sm">
                  <div>
                    <dt className="text-muted-foreground">Last updated</dt>
                    <dd className="mt-1 text-foreground">
                      {formatPublishedAt(video.updated_at)}
                    </dd>
                  </div>
                  {watch?.status ? (
                    <div>
                      <dt className="text-muted-foreground">Playback</dt>
                      <dd className="mt-1 text-foreground">
                        {playbackStatusLabel(watch.status)}
                      </dd>
                    </div>
                  ) : null}
                </dl>

                {watch?.message ? (
                  <p className="mt-4 rounded-lg border border-border bg-muted/40 px-3 py-2 text-muted-foreground text-xs leading-relaxed">
                    {watch.message}
                  </p>
                ) : null}
              </div>
            </aside>
          </div>
        ) : null}
      </PageMain>
    </div>
  )
}
