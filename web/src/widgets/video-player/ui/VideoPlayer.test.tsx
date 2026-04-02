import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { VideoPlayer } from './VideoPlayer'

const { MockHls, mockDestroy, getLastInstance } = vi.hoisted(() => {
  const mockDestroy = vi.fn()
  let lastInstance: MockHls | null = null
  class MockHls {
    static Events = { ERROR: 'hls-error', MANIFEST_PARSED: 'manifest-parsed' }
    handlers: Record<string, Array<() => void>> = {}
    levels: Array<{ height: number }> = []
    currentLevel = -1
    nextLevel = -1
    on = vi.fn((event: string, cb: () => void) => {
      if (!this.handlers[event]) this.handlers[event] = []
      this.handlers[event].push(cb)
    })
    loadSource = vi.fn()
    attachMedia = vi.fn()
    destroy = mockDestroy
    constructor() {
      lastInstance = this
    }
    emit(event: string) {
      for (const cb of this.handlers[event] ?? []) cb()
    }
    static isSupported() {
      return true
    }
  }
  return { MockHls, mockDestroy, getLastInstance: () => lastInstance }
})

vi.mock('hls.js', () => ({
  default: MockHls,
}))

const sampleRenditions = [
  { quality: '360p', playlist_url: 'http://example/hls/360p/prog.m3u8' },
  { quality: '720p', playlist_url: 'http://example/hls/720p/prog.m3u8' },
]

describe('VideoPlayer', () => {
  beforeEach(() => {
    vi.spyOn(HTMLVideoElement.prototype, 'load').mockImplementation(() => {})
  })
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
    render(
      <VideoPlayer
        manifestUrl="http://example/hls/master.m3u8"
        renditions={sampleRenditions}
      />,
    )
    expect(screen.getByTestId('video-player')).toBeInTheDocument()
  })

  it('destroys Hls on unmount', () => {
    vi.spyOn(HTMLVideoElement.prototype, 'canPlayType').mockReturnValue('')
    const { unmount } = render(
      <VideoPlayer
        manifestUrl="http://example/hls/master.m3u8"
        renditions={sampleRenditions}
      />,
    )
    unmount()
    expect(mockDestroy).toHaveBeenCalled()
  })

  it('shows quality buttons and switches hls level', () => {
    vi.spyOn(HTMLVideoElement.prototype, 'canPlayType').mockReturnValue('')
    render(
      <VideoPlayer
        manifestUrl="http://example/hls/master.m3u8"
        renditions={sampleRenditions}
      />,
    )
    const hls = getLastInstance()
    expect(hls).not.toBeNull()
    if (!hls) return
    hls.levels = [{ height: 360 }, { height: 720 }]
    act(() => {
      hls.emit(MockHls.Events.MANIFEST_PARSED)
    })

    expect(screen.getByTestId('quality-toolbar')).toBeInTheDocument()
    fireEvent.click(screen.getByTestId('quality-720p'))
    expect(hls.currentLevel).toBe(1)
    fireEvent.click(screen.getByTestId('quality-auto'))
    expect(hls.currentLevel).toBe(-1)
  })

  it('native HLS: quality buttons switch video src', () => {
    vi.spyOn(HTMLVideoElement.prototype, 'canPlayType').mockImplementation(
      (t) => (t === 'application/vnd.apple.mpegurl' ? 'maybe' : ''),
    )
    render(
      <VideoPlayer
        manifestUrl="http://example/hls/master.m3u8"
        renditions={sampleRenditions}
      />,
    )
    const video = screen.getByTestId('video-player') as HTMLVideoElement
    expect(video.src).toContain('master.m3u8')
    fireEvent.click(screen.getByTestId('quality-360p'))
    expect(video.src).toContain('360p')
    fireEvent.click(screen.getByTestId('quality-auto'))
    expect(video.src).toContain('master.m3u8')
  })
})
