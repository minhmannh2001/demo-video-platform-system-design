import type { Video } from '@/entities/video'

/** Minimal valid `Video` for tests. */
export function makeVideo(overrides: Partial<Video> = {}): Video {
  return {
    id: '507f1f77bcf86cd799439011',
    title: 'Test clip',
    description: '',
    uploader: 'demo',
    raw_s3_key: 'videos/507f1f77bcf86cd799439011/original.mp4',
    status: 'ready',
    created_at: '2024-01-01T00:00:00.000Z',
    updated_at: '2024-01-02T12:00:00.000Z',
    ...overrides,
  }
}
