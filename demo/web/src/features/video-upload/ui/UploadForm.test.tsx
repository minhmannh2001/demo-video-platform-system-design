import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { UploadForm } from './UploadForm'
import * as api from '@/shared/api/video-api'

vi.mock('@/shared/api/video-api', () => ({
  uploadVideo: vi.fn(),
}))

describe('UploadForm', () => {
  it('shows validation when title empty', async () => {
    const user = userEvent.setup()
    render(<UploadForm apiBase="http://test" />)
    await user.click(screen.getByRole('button', { name: /upload/i }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Title is required')
  })

  it('shows validation when no file', async () => {
    const user = userEvent.setup()
    render(<UploadForm apiBase="http://test" />)
    await user.type(screen.getByLabelText(/title/i), 'My video')
    await user.click(screen.getByRole('button', { name: /upload/i }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Choose a video file')
  })

  it('calls uploadVideo and onUploaded', async () => {
    const user = userEvent.setup()
    vi.mocked(api.uploadVideo).mockResolvedValue({ id: 'new-id', status: 'processing' })
    const onUploaded = vi.fn()
    render(<UploadForm apiBase="http://test" onUploaded={onUploaded} />)
    await user.type(screen.getByLabelText(/title/i), 'My video')
    const file = new File(['x'], 'clip.mp4', { type: 'video/mp4' })
    await user.upload(screen.getByLabelText(/file/i), file)
    await user.click(screen.getByRole('button', { name: /upload video/i }))
    await vi.waitFor(() => {
      expect(onUploaded).toHaveBeenCalledWith({ id: 'new-id', status: 'processing' })
    })
    expect(api.uploadVideo).toHaveBeenCalled()
  })
})
