import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { listVideos } from '@/shared/api/video-api'
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
    <div className="page">
      <header className="page-header">
        <h1>Video demo</h1>
        <Link to="/upload" className="btn">
          Upload
        </Link>
      </header>
      {loading ? <p>Loading…</p> : null}
      {err ? <p className="error">{err}</p> : null}
      {!loading && !err && items.length === 0 ? (
        <p>
          No videos yet. <Link to="/upload">Upload one</Link>.
        </p>
      ) : null}
      <ul className="video-list">
        {items.map((v) => (
          <li key={v.id} className="video-list__item">
            <Link to={`/watch/${v.id}`}>{v.title}</Link>
            <StatusBadge status={v.status} />
          </li>
        ))}
      </ul>
    </div>
  )
}
