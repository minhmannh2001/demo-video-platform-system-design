import { useParams } from 'react-router-dom'
import { isFailed, isProcessing, isReady } from '@/entities/video'
import { useVideoPolling } from '@/features/video-watch'
import { VideoPlayer } from '@/widgets/video-player'
import { PageMain } from '@/shared/ui/PageChrome'
import { AppHeader } from '@/widgets/app-header'

export function VideoWatchView() {
  const { id } = useParams<{ id: string }>()
  const { video, watch, error, loading } = useVideoPolling(id)

  return (
    <div className="min-h-screen bg-background">
      <AppHeader />

      <PageMain>
        <div className="mb-6">
          <h1 className="text-2xl font-semibold tracking-tight">{video?.title ?? 'Watch'}</h1>
        </div>

        {loading ? <p className="text-muted-foreground">Loading…</p> : null}
        {error ? <p className="text-destructive">{error}</p> : null}
        {video && isProcessing(video.status) ? (
          <p className="text-muted-foreground">Encoding in progress…</p>
        ) : null}
        {video && isFailed(video.status) ? <p className="text-destructive">Encoding failed.</p> : null}
        {video && isReady(video.status) && watch?.manifest_url ? (
          <VideoPlayer manifestUrl={watch.manifest_url} />
        ) : null}
      </PageMain>
    </div>
  )
}
