import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import * as watchFeature from '@/features/video-watch'
import * as videoApi from '@/shared/api/video-api'
import { makeVideo } from '@/test/fixtures/video'
import { VideoWatchView } from './VideoWatchView'

vi.mock('@/features/video-watch', () => ({
  useVideoPolling: vi.fn(),
}))

vi.mock('@/shared/api/video-api', async () => {
  const actual = await vi.importActual<typeof videoApi>(
    '@/shared/api/video-api',
  )
  return { ...actual, deleteVideo: vi.fn() }
})

vi.mock('@/widgets/video-player', () => ({
  VideoPlayer: ({ manifestUrl }: { manifestUrl: string }) => (
    <div data-testid="video-player-mock">{manifestUrl}</div>
  ),
}))

function renderWatch(id: string) {
  return render(
    <MemoryRouter initialEntries={[`/watch/${id}`]}>
      <Routes>
        <Route path="/" element={<div>Home page</div>} />
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
    expect(screen.getByRole('heading', { name: /about/i })).toBeInTheDocument()
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
    const video = makeVideo({
      status: 'ready',
      title: 'Ready vid',
      description: 'Full description text',
      duration_sec: 90,
    })
    const manifest = 'http://localhost:8080/stream/x/master.m3u8'
    vi.mocked(watchFeature.useVideoPolling).mockReturnValue({
      video,
      watch: { video_id: video.id, status: 'ready', manifest_url: manifest },
      error: null,
      loading: false,
      refresh: vi.fn(),
    })
    renderWatch(video.id)
    expect(
      screen.getByRole('heading', { name: /ready vid/i }),
    ).toBeInTheDocument()
    expect(screen.getByText('Full description text')).toBeInTheDocument()
    expect(screen.getByText('Uploaded by')).toBeInTheDocument()
    expect(screen.getByText('Published')).toBeInTheDocument()
    expect(screen.getByText('Length')).toBeInTheDocument()
    expect(screen.getByText('1:30')).toBeInTheDocument()
    expect(screen.getByText('Ready to play')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /all videos/i })).toHaveAttribute(
      'href',
      '/',
    )
    expect(screen.getByTestId('video-player-mock')).toHaveTextContent(manifest)
  })

  it('delete video confirms then calls API and navigates home', async () => {
    const user = userEvent.setup()
    const video = makeVideo({ status: 'ready', title: 'To delete' })
    vi.mocked(videoApi.deleteVideo).mockResolvedValue(undefined)
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)

    vi.mocked(watchFeature.useVideoPolling).mockReturnValue({
      video,
      watch: {
        video_id: video.id,
        status: 'ready',
        manifest_url: 'http://x/m.m3u8',
      },
      error: null,
      loading: false,
      refresh: vi.fn(),
    })
    renderWatch(video.id)

    await user.click(screen.getByRole('button', { name: /delete video/i }))
    expect(videoApi.deleteVideo).toHaveBeenCalledWith(video.id)
    expect(await screen.findByText('Home page')).toBeInTheDocument()

    confirmSpy.mockRestore()
  })
})
