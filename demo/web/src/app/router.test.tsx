import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AppRouter } from './router'

vi.mock('@/shared/api/video-api', () => ({
  listVideos: vi.fn().mockResolvedValue([]),
}))

describe('AppRouter', () => {
  beforeEach(() => {
    window.history.pushState({}, '', '/')
  })

  it('renders home route', async () => {
    render(<AppRouter />)
    expect(await screen.findByRole('heading', { name: /videos/i })).toBeInTheDocument()
  })
})
