import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { VideoPlayer } from './VideoPlayer'

const { MockHls, mockDestroy } = vi.hoisted(() => {
  const mockDestroy = vi.fn()
  class MockHls {
    loadSource = vi.fn()
    attachMedia = vi.fn()
    destroy = mockDestroy
    static isSupported() {
      return true
    }
  }
  return { MockHls, mockDestroy }
})

vi.mock('hls.js', () => ({
  default: MockHls,
}))

describe('VideoPlayer', () => {
  afterEach(() => {
    mockDestroy.mockClear()
    vi.restoreAllMocks()
  })

  it('renders nothing without manifestUrl', () => {
    const { container } = render(<VideoPlayer manifestUrl={null} />)
    expect(container.querySelector('video')).toBeNull()
  })

  it('uses Hls when native HLS not available', () => {
    vi.spyOn(HTMLVideoElement.prototype, 'canPlayType').mockReturnValue('')
    render(<VideoPlayer manifestUrl="http://example/hls/master.m3u8" />)
    expect(screen.getByTestId('video-player')).toBeInTheDocument()
  })

  it('destroys Hls on unmount', () => {
    vi.spyOn(HTMLVideoElement.prototype, 'canPlayType').mockReturnValue('')
    const { unmount } = render(<VideoPlayer manifestUrl="http://example/hls/master.m3u8" />)
    unmount()
    expect(mockDestroy).toHaveBeenCalled()
  })
})
