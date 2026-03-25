import { renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useVideoPolling } from './useVideoPolling'
import * as api from '@/shared/api/video-api'
import type { Video, WatchResponse } from '@/entities/video'

vi.mock('@/shared/config/env', () => ({
  getApiBase: () => 'http://test',
}))

vi.mock('@/shared/api/video-api', () => ({
  getVideo: vi.fn(),
  getWatch: vi.fn(),
}))

describe('useVideoPolling', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('clears state when videoId undefined', () => {
    const { result } = renderHook(() => useVideoPolling(undefined))
    expect(result.current.video).toBeNull()
    expect(result.current.loading).toBe(false)
  })

  it('fetches video and watch', async () => {
    const video: Video = {
      id: 'v1',
      title: 't',
      description: '',
      uploader: 'u',
      raw_s3_key: 'k',
      status: 'ready',
      created_at: '',
      updated_at: '',
    }
    const watch: WatchResponse = {
      video_id: 'v1',
      status: 'ready',
      manifest_url: 'http://test/stream/v1/master.m3u8',
    }
    vi.mocked(api.getVideo).mockResolvedValue(video)
    vi.mocked(api.getWatch).mockResolvedValue(watch)

    const { result } = renderHook(() => useVideoPolling('v1', 60_000))

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.video).toEqual(video)
    expect(result.current.watch).toEqual(watch)
    expect(result.current.error).toBeNull()
    expect(api.getVideo).toHaveBeenCalledWith('v1', 'http://test')
    expect(api.getWatch).toHaveBeenCalledWith('v1', 'http://test')
  })

  it('sets error on failure', async () => {
    vi.mocked(api.getVideo).mockRejectedValue(new Error('network'))
    vi.mocked(api.getWatch).mockRejectedValue(new Error('network'))

    const { result } = renderHook(() => useVideoPolling('v1', 60_000))

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error).toBe('network')
  })
})
