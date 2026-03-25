import { getApiBase } from '@/shared/config/env'
import type { UploadResponse, Video, WatchResponse } from '@/entities/video'

async function parseJson<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || res.statusText)
  }
  return res.json() as Promise<T>
}

export async function listVideos(baseUrl: string = getApiBase()): Promise<Video[]> {
  const res = await fetch(`${baseUrl}/videos`)
  const data = await parseJson<Video[] | null>(res)
  return Array.isArray(data) ? data : []
}

export async function getVideo(id: string, baseUrl: string = getApiBase()): Promise<Video> {
  const res = await fetch(`${baseUrl}/videos/${encodeURIComponent(id)}`)
  return parseJson<Video>(res)
}

export async function getWatch(id: string, baseUrl: string = getApiBase()): Promise<WatchResponse> {
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
