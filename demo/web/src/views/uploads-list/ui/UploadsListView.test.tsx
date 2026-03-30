import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/shared/api/video-api'
import { makeVideo } from '@/test/fixtures/video'
import { UploadsListView } from './UploadsListView'

vi.mock('@/shared/api/video-api', () => ({
  listVideos: vi.fn(),
}))

function renderList() {
  return render(
    <MemoryRouter>
      <UploadsListView />
    </MemoryRouter>,
  )
}

describe('UploadsListView', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('shows loading then empty state with CTA', async () => {
    vi.mocked(api.listVideos).mockResolvedValue([])
    renderList()

    expect(screen.getByText('Loading…')).toBeInTheDocument()

    await waitFor(() => {
      expect(screen.queryByText('Loading…')).not.toBeInTheDocument()
    })
    expect(
      screen.getByText(/no videos in the catalog yet/i),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: /upload a video/i }),
    ).toHaveAttribute('href', '/upload')
  })

  it('shows error message', async () => {
    vi.mocked(api.listVideos).mockRejectedValue(new Error('API unavailable'))
    renderList()

    await waitFor(() => {
      expect(screen.getByText('API unavailable')).toBeInTheDocument()
    })
  })

  it('renders table row with link to upload detail', async () => {
    const v = makeVideo({
      id: 'deadbeef1234567890abcdef',
      title: 'Row title',
      uploader: 'alice',
    })
    vi.mocked(api.listVideos).mockResolvedValue([v])
    renderList()

    await waitFor(() => {
      expect(
        screen.getByRole('columnheader', { name: /^title$/i }),
      ).toBeInTheDocument()
    })
    expect(screen.getByRole('link', { name: /row title/i })).toHaveAttribute(
      'href',
      '/uploads/deadbeef1234567890abcdef',
    )
    expect(screen.getByText('alice')).toBeInTheDocument()
  })
})
