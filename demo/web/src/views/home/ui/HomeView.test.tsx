import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/shared/api/video-api'
import { makeVideo } from '@/test/fixtures/video'
import { HomeView } from './HomeView'

vi.mock('@/shared/api/video-api', () => ({
  listVideos: vi.fn(),
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
    renderHome()

    expect(screen.getByRole('heading', { name: /videos/i })).toBeInTheDocument()

    await waitFor(() => {
      expect(screen.getByText(/no videos yet/i)).toBeInTheDocument()
    })
    expect(screen.getByRole('link', { name: /upload one/i })).toHaveAttribute('href', '/upload')
  })

  it('shows error when list fails', async () => {
    vi.mocked(api.listVideos).mockRejectedValue(new Error('network down'))
    renderHome()

    await waitFor(() => {
      expect(screen.getByText('network down')).toBeInTheDocument()
    })
  })

  it('renders video cards when videos exist', async () => {
    const v = makeVideo({ id: 'abc', title: 'My title' })
    vi.mocked(api.listVideos).mockResolvedValue([v])
    renderHome()

    await waitFor(() => {
      expect(screen.getByText('My title')).toBeInTheDocument()
    })
    expect(screen.getByRole('link', { name: /my title/i })).toHaveAttribute('href', '/watch/abc')
  })
})
