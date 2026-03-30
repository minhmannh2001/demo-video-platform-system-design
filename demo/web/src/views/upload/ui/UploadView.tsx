import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '@/shared/ui/Button'
import { PageMain } from '@/shared/ui/PageChrome'
import { AppHeader } from '@/widgets/app-header'
import { UploadForm } from '@/features/video-upload'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import type { UploadResponse } from '@/entities/video'

export function UploadView() {
  const nav = useNavigate()
  const [done, setDone] = useState<UploadResponse | null>(null)

  return (
    <div className="min-h-screen bg-background">
      <AppHeader />

      <PageMain>
        <div className="mx-auto max-w-2xl">
          <Card className="gap-0 overflow-hidden py-0 shadow-sm ring-1 ring-foreground/5">
            <CardHeader className="border-b border-border bg-muted/30 px-6 pb-4 pt-6">
              <CardTitle className="text-lg tracking-tight">Upload a video</CardTitle>
              <CardDescription>
                Add metadata and a file — we’ll process it for playback on the watch page.
              </CardDescription>
            </CardHeader>
            <CardContent className="bg-card px-6 pb-6 pt-6">
              <UploadForm
                onUploaded={(r) => {
                  setDone(r)
                }}
              />
            </CardContent>
          </Card>
          {done ? (
            <div className="mt-6 rounded-xl border border-border bg-muted/30 px-4 py-3 text-sm text-muted-foreground">
              <p>
                Uploaded{' '}
                <code className="rounded-md bg-background px-1.5 py-0.5 font-mono text-foreground text-xs ring-1 ring-border">
                  {done.id}
                </code>{' '}
                — status <span className="font-medium text-foreground">{done.status}</span>.
              </p>
              <Button
                type="button"
                variant="link"
                className="mt-1 h-auto p-0 text-foreground"
                onClick={() => nav(`/watch/${done.id}`)}
              >
                Open watch page →
              </Button>
            </div>
          ) : null}
        </div>
      </PageMain>
    </div>
  )
}
