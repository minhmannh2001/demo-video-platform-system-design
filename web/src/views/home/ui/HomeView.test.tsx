import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/shared/api/video-api'
import { makeVideo } from '@/test/fixtures/video'
import { HomeView } from './HomeView'

vi.mock('@/shared/api/video-api', () => ({
  listVideos: vi.fn(),
  searchPublishedVideos: vi.fn(),
  getVideo: vi.fn(),
}))

function renderHome() {
  return render(
    <MemoryRouter>
      <HomeView />
    </MemoryRouter>,
  )
}

describe('HomeView', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('shows loading skeleton then empty state when no videos', async () => {
    vi.mocked(api.listVideos).mockResolvedValue([])
    vi.mocked(api.searchPublishedVideos).mockRejectedValue(
      new Error('should not call search'),
    )
    renderHome()

    expect(screen.getByRole('heading', { name: /videos/i })).toBeInTheDocument()

    await waitFor(() => {
      expect(screen.getByText(/no videos yet/i)).toBeInTheDocument()
    })
    expect(screen.getByRole('link', { name: /upload one/i })).toHaveAttribute(
      'href',
      '/upload',
    )
  })

  it('shows error when list fails', async () => {
    vi.mocked(api.listVideos).mockRejectedValue(new Error('network down'))
    vi.mocked(api.searchPublishedVideos).mockRejectedValue(
      new Error('should not call search'),
    )
    renderHome()

    await waitFor(() => {
      expect(screen.getByText('network down')).toBeInTheDocument()
    })
  })

  it('renders video cards when videos exist', async () => {
    const v = makeVideo({ id: 'abc', title: 'My title' })
    vi.mocked(api.listVideos).mockResolvedValue([v])
    vi.mocked(api.searchPublishedVideos).mockRejectedValue(
      new Error('should not call search'),
    )
    renderHome()

    await waitFor(() => {
      expect(screen.getByText('My title')).toBeInTheDocument()
    })
    expect(screen.getByRole('link', { name: /my title/i })).toHaveAttribute(
      'href',
      '/watch/abc',
    )
  })

  it('debounced search calls API and shows hydrated cards', async () => {
    const user = userEvent.setup()
    vi.mocked(api.listVideos).mockResolvedValue([
      makeVideo({ id: 'c1', title: 'Catalog only' }),
    ])
    vi.mocked(api.searchPublishedVideos).mockResolvedValue({
      total: 1,
      from: 0,
      size: 24,
      hits: [{ video_id: 'hit1' }],
    })
    vi.mocked(api.getVideo).mockResolvedValue(
      makeVideo({ id: 'hit1', title: 'Search hit' }),
    )
    renderHome()

    await waitFor(() => {
      expect(screen.getByText('Catalog only')).toBeInTheDocument()
    })

    await user.type(screen.getByLabelText('Search catalog'), 'doc')

    await waitFor(
      () => {
        expect(api.searchPublishedVideos).toHaveBeenCalledWith(
          'doc',
          expect.objectContaining({ size: 24, highlight: true }),
        )
        expect(screen.getByText('Search hit')).toBeInTheDocument()
      },
      { timeout: 4000 },
    )
  })
})
