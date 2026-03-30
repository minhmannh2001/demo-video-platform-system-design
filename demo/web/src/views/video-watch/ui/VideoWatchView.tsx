import { Link, useParams } from 'react-router-dom'
import { buttonVariants } from '@/components/ui/button'
import { isFailed, isProcessing, isReady } from '@/entities/video'
import { useVideoPolling } from '@/features/video-watch'
import { VideoPlayer } from '@/widgets/video-player'
import { cn } from '@/lib/utils'

export function VideoWatchView() {
  const { id } = useParams<{ id: string }>()
  const { video, watch, error, loading } = useVideoPolling(id)

  return (
    <div className="mx-auto max-w-2xl px-6 py-6">
      <header className="mb-6 flex items-center justify-between gap-4">
        <h1 className="text-xl font-semibold">{video?.title ?? 'Watch'}</h1>
        <Link to="/" className={cn(buttonVariants({ variant: 'ghost' }))}>
          Home
        </Link>
      </header>
      {loading ? <p className="text-muted-foreground">Loading…</p> : null}
      {error ? <p className="text-destructive">{error}</p> : null}
      {video && isProcessing(video.status) ? (
        <p className="text-muted-foreground">Encoding in progress…</p>
      ) : null}
      {video && isFailed(video.status) ? <p className="text-destructive">Encoding failed.</p> : null}
      {video && isReady(video.status) && watch?.manifest_url ? (
        <VideoPlayer manifestUrl={watch.manifest_url} />
      ) : null}
    </div>
  )
}
