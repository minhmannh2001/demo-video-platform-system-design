import { afterEach, describe, expect, it, vi } from 'vitest'
import { getVideo, getWatch, listVideos, uploadVideo } from './video-api'
import type { Video, WatchResponse } from '@/entities/video'

describe('video-api', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('listVideos coerces JSON null to empty array', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(null),
      }),
    )
    const out = await listVideos('http://localhost:9999')
    expect(out).toEqual([])
  })

  it('listVideos parses JSON', async () => {
    const data: Video[] = [
      {
        id: '1',
        title: 'a',
        description: '',
        uploader: 'u',
        raw_s3_key: 'k',
        status: 'ready',
        created_at: '',
        updated_at: '',
      },
    ]
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(data),
      }),
    )
    const out = await listVideos('http://localhost:9999')
    expect(out).toEqual(data)
    expect(fetch).toHaveBeenCalledWith('http://localhost:9999/videos')
  })

  it('getVideo throws on non-ok', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: false,
        statusText: 'Not Found',
        text: () => Promise.resolve('missing'),
      }),
    )
    await expect(getVideo('abc', 'http://localhost:9999')).rejects.toThrow(
      'missing',
    )
  })

  it('getWatch returns manifest', async () => {
    const w: WatchResponse = {
      video_id: 'x',
      status: 'ready',
      manifest_url: 'http://localhost:8080/stream/x/master.m3u8',
    }
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(w),
      }),
    )
    const out = await getWatch('x', 'http://localhost:9999')
    expect(out.manifest_url).toContain('master.m3u8')
  })

  it('uploadVideo posts FormData', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ id: 'vid1', status: 'processing' }),
      }),
    )
    const fd = new FormData()
    fd.set('title', 't')
    const r = await uploadVideo(fd, 'http://localhost:9999')
    expect(r.id).toBe('vid1')
    expect(fetch).toHaveBeenCalledWith(
      'http://localhost:9999/videos/upload',
      expect.objectContaining({ method: 'POST', body: fd }),
    )
  })
})
