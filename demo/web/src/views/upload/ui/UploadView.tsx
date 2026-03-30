import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '@/shared/ui/Button'
import { PageMain } from '@/shared/ui/PageChrome'
import { AppHeader } from '@/widgets/app-header'
import { UploadForm } from '@/features/video-upload'
import { Card } from '@/shared/ui/Card'
import type { UploadResponse } from '@/entities/video'

export function UploadView() {
  const nav = useNavigate()
  const [done, setDone] = useState<UploadResponse | null>(null)

  return (
    <div className="min-h-screen bg-background">
      <AppHeader />

      <PageMain>
        <div className="mb-6">
          <h1 className="text-2xl font-semibold tracking-tight">Upload</h1>
          <p className="mt-1 text-sm text-muted-foreground">Add a title and choose a video file.</p>
        </div>

        <div className="mx-auto max-w-2xl">
          <Card>
            <UploadForm
              onUploaded={(r) => {
                setDone(r)
              }}
            />
          </Card>
          {done ? (
            <p className="mt-4 text-muted-foreground">
              Uploaded <code className="rounded bg-muted px-1 py-0.5 text-sm">{done.id}</code> — status{' '}
              {done.status}.{' '}
              <Button type="button" variant="link" className="h-auto p-0" onClick={() => nav(`/watch/${done.id}`)}>
                Open watch page
              </Button>
            </p>
          ) : null}
        </div>
      </PageMain>
    </div>
  )
}
