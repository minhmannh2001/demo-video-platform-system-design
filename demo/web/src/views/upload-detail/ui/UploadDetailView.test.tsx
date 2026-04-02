import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import * as watchFeature from '@/features/video-watch'
import { makeVideo } from '@/test/fixtures/video'
import { UploadDetailView } from './UploadDetailView'

vi.mock('@/features/video-watch', () => ({
  useVideoWatchFeed: vi.fn(),
}))

function renderDetail(id: string) {
  return render(
    <MemoryRouter initialEntries={[`/uploads/${id}`]}>
      <Routes>
        <Route path="/uploads/:id" element={<UploadDetailView />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('UploadDetailView', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('shows back link to upload queue', () => {
    vi.mocked(watchFeature.useVideoWatchFeed).mockReturnValue({
      video: null,
      watch: null,
      error: null,
      loading: true,
      refresh: vi.fn(),
    })
    renderDetail('vid1')
    expect(
      screen.getByRole('link', { name: /^← upload queue$/i }),
    ).toHaveAttribute('href', '/uploads')
  })

  it('shows loading and id fallback in subtitle', () => {
    vi.mocked(watchFeature.useVideoWatchFeed).mockReturnValue({
      video: null,
      watch: null,
      error: null,
      loading: true,
      refresh: vi.fn(),
    })
    renderDetail('vid-xyz')
    expect(screen.getByText('Loading…')).toBeInTheDocument()
    expect(screen.getByText('vid-xyz')).toBeInTheDocument()
  })

  it('shows processing copy', () => {
    const video = makeVideo({ status: 'processing' })
    vi.mocked(watchFeature.useVideoWatchFeed).mockReturnValue({
      video,
      watch: { video_id: video.id, status: 'processing' },
      error: null,
      loading: false,
      refresh: vi.fn(),
    })
    renderDetail(video.id)
    expect(screen.getByText(/encoding in progress/i)).toBeInTheDocument()
  })

  it('shows failed when watch is failed even if video doc still says processing', () => {
    const video = makeVideo({ status: 'processing' })
    vi.mocked(watchFeature.useVideoWatchFeed).mockReturnValue({
      video,
      watch: { video_id: video.id, status: 'failed', message: 'encoding failed' },
      error: null,
      loading: false,
      refresh: vi.fn(),
    })
    renderDetail(video.id)
    expect(screen.getByText(/encoding failed/i)).toBeInTheDocument()
    expect(screen.queryByText(/encoding in progress/i)).not.toBeInTheDocument()
  })

  it('shows Open player when ready with manifest', () => {
    const video = makeVideo({ status: 'ready' })
    const manifest = 'http://localhost/hls/master.m3u8'
    vi.mocked(watchFeature.useVideoWatchFeed).mockReturnValue({
      video,
      watch: { video_id: video.id, status: 'ready', manifest_url: manifest },
      error: null,
      loading: false,
      refresh: vi.fn(),
    })
    renderDetail(video.id)
    expect(
      screen.getByRole('link', { name: /^open player$/i }),
    ).toHaveAttribute('href', `/watch/${video.id}`)
  })

  it('shows manifest loading hint when ready without manifest', () => {
    const video = makeVideo({ status: 'ready' })
    vi.mocked(watchFeature.useVideoWatchFeed).mockReturnValue({
      video,
      watch: { video_id: video.id, status: 'ready' },
      error: null,
      loading: false,
      refresh: vi.fn(),
    })
    renderDetail(video.id)
    expect(screen.getByText(/manifest is still loading/i)).toBeInTheDocument()
  })
})
