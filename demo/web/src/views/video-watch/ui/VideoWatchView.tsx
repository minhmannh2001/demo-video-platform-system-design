import { Link, useParams } from 'react-router-dom'
import { isFailed, isProcessing, isReady } from '@/entities/video'
import { useVideoPolling } from '@/features/video-watch'
import { VideoPlayer } from '@/widgets/video-player'

export function VideoWatchView() {
  const { id } = useParams<{ id: string }>()
  const { video, watch, error, loading } = useVideoPolling(id)

  return (
    <div className="page">
      <header className="page-header">
        <h1>{video?.title ?? 'Watch'}</h1>
        <Link to="/">Home</Link>
      </header>
      {loading ? <p>Loading…</p> : null}
      {error ? <p className="error">{error}</p> : null}
      {video && isProcessing(video.status) ? <p>Encoding in progress…</p> : null}
      {video && isFailed(video.status) ? <p className="error">Encoding failed.</p> : null}
      {video && isReady(video.status) && watch?.manifest_url ? (
        <VideoPlayer manifestUrl={watch.manifest_url} />
      ) : null}
    </div>
  )
}
