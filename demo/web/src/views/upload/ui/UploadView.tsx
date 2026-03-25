import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { UploadForm } from '@/features/video-upload'
import { Card } from '@/shared/ui/Card'
import type { UploadResponse } from '@/entities/video'

export function UploadView() {
  const nav = useNavigate()
  const [done, setDone] = useState<UploadResponse | null>(null)

  return (
    <div className="page">
      <header className="page-header">
        <h1>Upload</h1>
        <Link to="/">Home</Link>
      </header>
      <Card>
        <UploadForm
          onUploaded={(r) => {
            setDone(r)
          }}
        />
      </Card>
      {done ? (
        <p className="upload-done">
          Uploaded <code>{done.id}</code> — status {done.status}.{' '}
          <button type="button" className="btn-link" onClick={() => nav(`/watch/${done.id}`)}>
            Open watch page
          </button>
        </p>
      ) : null}
    </div>
  )
}
