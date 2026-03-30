import { useId, useState, type FormEvent } from 'react'
import { Label } from '@/components/ui/label'
import { uploadVideo } from '@/shared/api/video-api'
import { Button } from '@/shared/ui/Button'
import { Input } from '@/shared/ui/Input'
import { Textarea } from '@/shared/ui/Textarea'
import type { UploadResponse } from '@/entities/video'

type Props = {
  onUploaded?: (r: UploadResponse) => void
  apiBase?: string
}

export function UploadForm({ onUploaded, apiBase }: Props) {
  const formId = useId()
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [uploader, setUploader] = useState('demo')
  const [file, setFile] = useState<File | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    if (!title.trim()) {
      setError('Title is required')
      return
    }
    if (!file) {
      setError('Choose a video file')
      return
    }
    const fd = new FormData()
    fd.set('title', title.trim())
    fd.set('description', description)
    fd.set('uploader', uploader.trim() || 'demo')
    fd.set('file', file)
    setSubmitting(true)
    try {
      const r = await uploadVideo(fd, apiBase)
      onUploaded?.(r)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4" data-testid="upload-form">
      <div className="space-y-1.5">
        <Label htmlFor={`${formId}-title`} className="w-fit">
          Title *
        </Label>
        <Input
          id={`${formId}-title`}
          name="title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          autoComplete="off"
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor={`${formId}-description`} className="w-fit">
          Description
        </Label>
        <Textarea
          id={`${formId}-description`}
          name="description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor={`${formId}-uploader`} className="w-fit">
          Uploader
        </Label>
        <Input
          id={`${formId}-uploader`}
          name="uploader"
          value={uploader}
          onChange={(e) => setUploader(e.target.value)}
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor={`${formId}-file`} className="w-fit">
          File *
        </Label>
        <Input
          id={`${formId}-file`}
          name="file"
          type="file"
          accept="video/*,.mp4"
          onChange={(e) => setFile(e.target.files?.[0] ?? null)}
        />
      </div>
      {error ? (
        <p className="text-destructive text-sm" role="alert">
          {error}
        </p>
      ) : null}
      <Button type="submit" disabled={submitting}>
        {submitting ? 'Uploading…' : 'Upload'}
      </Button>
    </form>
  )
}
