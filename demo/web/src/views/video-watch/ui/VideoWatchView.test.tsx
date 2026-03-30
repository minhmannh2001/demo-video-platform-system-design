import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import * as watchFeature from '@/features/video-watch'
import { makeVideo } from '@/test/fixtures/video'
import { VideoWatchView } from './VideoWatchView'

vi.mock('@/features/video-watch', () => ({
  useVideoPolling: vi.fn(),
}))

vi.mock('@/widgets/video-player', () => ({
  VideoPlayer: ({ manifestUrl }: { manifestUrl: string }) => (
    <div data-testid="video-player-mock">{manifestUrl}</div>
  ),
}))

function renderWatch(id: string) {
  return render(
    <MemoryRouter initialEntries={[`/watch/${id}`]}>
      <Routes>
        <Route path="/watch/:id" element={<VideoWatchView />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('VideoWatchView', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('shows loading', () => {
    vi.mocked(watchFeature.useVideoPolling).mockReturnValue({
      video: null,
      watch: null,
      error: null,
      loading: true,
      refresh: vi.fn(),
    })
    renderWatch('v1')
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows error', () => {
    vi.mocked(watchFeature.useVideoPolling).mockReturnValue({
      video: null,
      watch: null,
      error: 'boom',
      loading: false,
      refresh: vi.fn(),
    })
    renderWatch('v1')
    expect(screen.getByText('boom')).toBeInTheDocument()
  })

  it('shows encoding message when processing', () => {
    const video = makeVideo({ status: 'processing' })
    vi.mocked(watchFeature.useVideoPolling).mockReturnValue({
      video,
      watch: { video_id: video.id, status: 'processing' },
      error: null,
      loading: false,
      refresh: vi.fn(),
    })
    renderWatch(video.id)
    expect(screen.getByText(/encoding in progress/i)).toBeInTheDocument()
  })

  it('shows failed message when failed', () => {
    const video = makeVideo({ status: 'failed' })
    vi.mocked(watchFeature.useVideoPolling).mockReturnValue({
      video,
      watch: { video_id: video.id, status: 'failed' },
      error: null,
      loading: false,
      refresh: vi.fn(),
    })
    renderWatch(video.id)
    expect(screen.getByText(/encoding failed/i)).toBeInTheDocument()
  })

  it('renders VideoPlayer when ready with manifest', () => {
    const video = makeVideo({ status: 'ready', title: 'Ready vid' })
    const manifest = 'http://localhost:8080/stream/x/master.m3u8'
    vi.mocked(watchFeature.useVideoPolling).mockReturnValue({
      video,
      watch: { video_id: video.id, status: 'ready', manifest_url: manifest },
      error: null,
      loading: false,
      refresh: vi.fn(),
    })
    renderWatch(video.id)
    expect(screen.getByRole('heading', { name: /ready vid/i })).toBeInTheDocument()
    expect(screen.getByTestId('video-player-mock')).toHaveTextContent(manifest)
  })
})
