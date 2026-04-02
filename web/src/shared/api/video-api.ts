import { getApiBase } from '@/shared/config/env'
import type {
  UploadResponse,
  Video,
  VideoSearchResponse,
  WatchResponse,
} from '@/entities/video'

async function parseJson<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || res.statusText)
  }
  return res.json() as Promise<T>
}

export async function listVideos(
  baseUrl: string = getApiBase(),
): Promise<Video[]> {
  const res = await fetch(`${baseUrl}/videos`)
  const data = await parseJson<Video[] | null>(res)
  return Array.isArray(data) ? data : []
}

export type SearchPublishedOptions = {
  from?: number
  size?: number
  /** Ask API for <mark> snippets in highlights (optional). */
  highlight?: boolean
}

export async function searchPublishedVideos(
  q: string,
  opts: SearchPublishedOptions = {},
  baseUrl: string = getApiBase(),
): Promise<VideoSearchResponse> {
  const params = new URLSearchParams()
  params.set('q', q)
  if (opts.from != null) params.set('from', String(opts.from))
  if (opts.size != null) params.set('size', String(opts.size))
  if (opts.highlight) params.set('highlight', 'true')
  const res = await fetch(
    `${baseUrl}/videos/search?${params.toString()}`,
  )
  return parseJson<VideoSearchResponse>(res)
}

export async function getVideo(
  id: string,
  baseUrl: string = getApiBase(),
): Promise<Video> {
  const res = await fetch(`${baseUrl}/videos/${encodeURIComponent(id)}`)
  return parseJson<Video>(res)
}

export async function getWatch(
  id: string,
  baseUrl: string = getApiBase(),
): Promise<WatchResponse> {
  const res = await fetch(`${baseUrl}/videos/${encodeURIComponent(id)}/watch`)
  return parseJson<WatchResponse>(res)
}

export async function uploadVideo(
  formData: FormData,
  baseUrl: string = getApiBase(),
): Promise<UploadResponse> {
  const res = await fetch(`${baseUrl}/videos/upload`, {
    method: 'POST',
    body: formData,
  })
  return parseJson<UploadResponse>(res)
}

export async function deleteVideo(
  id: string,
  baseUrl: string = getApiBase(),
): Promise<void> {
  const res = await fetch(`${baseUrl}/videos/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || res.statusText)
  }
}
