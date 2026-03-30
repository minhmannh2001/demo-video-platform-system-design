import { useId, useState, type FormEvent, type DragEvent } from 'react'
import { Label } from '@/components/ui/label'
import { uploadVideo } from '@/shared/api/video-api'
import { Button } from '@/shared/ui/Button'
import { Input } from '@/shared/ui/Input'
import { Textarea } from '@/shared/ui/Textarea'
import { cn } from '@/lib/utils'
import type { UploadResponse } from '@/entities/video'

/** Response from the API plus fields needed for client tracking / navigation. */
export type UploadSuccessPayload = UploadResponse & {
  title: string
  fileName?: string
}

type Props = {
  onUploaded?: (r: UploadSuccessPayload) => void
  apiBase?: string
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function UploadForm({ onUploaded, apiBase }: Props) {
  const formId = useId()
  const titleId = `${formId}-title`
  const descriptionId = `${formId}-description`
  const uploaderId = `${formId}-uploader`
  const fileId = `${formId}-file`

  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [uploader, setUploader] = useState('demo')
  const [file, setFile] = useState<File | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [isDragging, setIsDragging] = useState(false)

  function pickFile(next: File | null) {
    setFile(next)
    setError(null)
  }

  function onDragOver(e: DragEvent) {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(true)
  }

  function onDragLeave(e: DragEvent) {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(false)
  }

  function onDrop(e: DragEvent) {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(false)
    const dropped = e.dataTransfer.files?.[0]
    if (dropped && dropped.type.startsWith('video/')) {
      pickFile(dropped)
      return
    }
    if (dropped?.name.toLowerCase().endsWith('.mp4')) {
      pickFile(dropped)
    }
  }

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
      onUploaded?.({ ...r, title: title.trim(), fileName: file?.name })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-8" data-testid="upload-form">
      <div className="space-y-4">
        <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Details</p>
        <div className="grid gap-6 md:grid-cols-2">
          <div className="flex flex-col gap-3 md:col-span-2">
            <Label htmlFor={titleId}>Video title *</Label>
            <Input
              id={titleId}
              name="title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="e.g. Weekend walkthrough"
              autoComplete="off"
            />
          </div>
          <div className="flex flex-col gap-3 md:col-span-2">
            <Label htmlFor={descriptionId}>Description</Label>
            <Textarea
              id={descriptionId}
              name="description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional — what is this video about?"
              rows={4}
              className="min-h-[100px] resize-y"
            />
          </div>
          <div className="flex flex-col gap-3">
            <Label htmlFor={uploaderId}>Uploader</Label>
            <Input
              id={uploaderId}
              name="uploader"
              value={uploader}
              onChange={(e) => setUploader(e.target.value)}
              placeholder="demo"
            />
          </div>
        </div>
      </div>

      <div className="space-y-3.5">
        <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Video file</p>
        <div
          className={cn(
            'rounded-xl border-2 border-dashed transition-colors',
            isDragging && 'border-primary bg-primary/5 ring-2 ring-primary/20',
            !isDragging && file && 'border-primary/40 bg-muted/40',
            !isDragging && !file && 'border-border bg-muted/20',
          )}
          onDragOver={onDragOver}
          onDragLeave={onDragLeave}
          onDrop={onDrop}
        >
          <input
            id={fileId}
            name="file"
            type="file"
            accept="video/*,.mp4"
            className="sr-only"
            onChange={(e) => pickFile(e.target.files?.[0] ?? null)}
          />
          <label
            htmlFor={fileId}
            className="flex cursor-pointer flex-col items-center gap-2 px-6 py-10 text-center"
          >
            <span className="sr-only">Video file</span>
            <span
              className="flex size-12 items-center justify-center rounded-full bg-background ring-1 ring-border"
              aria-hidden
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                className="size-6 text-muted-foreground"
              >
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                <polyline points="17 8 12 3 7 8" />
                <line x1="12" x2="12" y1="3" y2="15" />
              </svg>
            </span>
            <span className="text-sm font-medium text-foreground">
              {file ? file.name : 'Drop a video here or click to browse'}
            </span>
            <span className="text-xs text-muted-foreground">
              {file
                ? `${formatFileSize(file.size)} · MP4 and common video formats`
                : 'MP4 and common video formats'}
            </span>
          </label>
        </div>
      </div>

      {error ? (
        <div
          className="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-destructive text-sm"
          role="alert"
        >
          {error}
        </div>
      ) : null}

      <div className="flex flex-col gap-3 border-t border-border pt-6 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-xs text-muted-foreground">Fields marked * are required.</p>
        <Button type="submit" size="lg" disabled={submitting} className="w-full sm:w-auto sm:min-w-[140px]">
          {submitting ? 'Uploading…' : 'Upload video'}
        </Button>
      </div>
    </form>
  )
}
