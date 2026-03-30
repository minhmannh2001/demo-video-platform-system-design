import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import type { Video } from '../model/types'
import { VideoCard } from './VideoCard'

function renderCard(video: Video) {
  return render(
    <MemoryRouter>
      <VideoCard video={video} />
    </MemoryRouter>,
  )
}

const base = (over: Partial<Video> = {}): Video => ({
  id: '507f1f77bcf86cd799439011',
  title: 'My clip',
  description: 'A short description for the card.',
  uploader: 'demo',
  raw_s3_key: 'videos/x/original.mp4',
  status: 'ready',
  created_at: '2026-03-15T14:30:00.000Z',
  updated_at: '2026-03-15T14:30:00.000Z',
  ...over,
})

describe('VideoCard', () => {
  it('links to watch URL for video id', () => {
    const v = base({ id: 'abc123' })
    renderCard(v)
    const link = screen.getByRole('link', { name: /my clip/i })
    expect(link).toHaveAttribute('href', '/watch/abc123')
  })

  it('renders title, uploader, status badge, and description', () => {
    renderCard(base())
    expect(
      screen.getByRole('heading', { level: 2, name: /my clip/i }),
    ).toBeInTheDocument()
    expect(screen.getByText('demo')).toBeInTheDocument()
    expect(screen.getByTestId('status-badge')).toHaveTextContent('ready')
    expect(
      screen.getByText(/a short description for the card/i),
    ).toBeInTheDocument()
  })

  it('shows placeholder when no thumbnail_url', () => {
    renderCard(base())
    expect(screen.getByText('No preview')).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
  })

  it('renders thumbnail image when thumbnail_url is set', () => {
    const { container } = renderCard(
      base({ thumbnail_url: 'https://example.com/poster.jpg' }),
    )
    const img = container.querySelector(
      'img[src="https://example.com/poster.jpg"]',
    )
    expect(img).toBeTruthy()
  })

  it('shows No description when description is empty', () => {
    renderCard(base({ description: '   ' }))
    expect(screen.getByText('No description')).toBeInTheDocument()
  })

  it('uses Unknown when uploader is empty', () => {
    renderCard(base({ uploader: '' }))
    expect(screen.getByText('Unknown')).toBeInTheDocument()
  })
})
