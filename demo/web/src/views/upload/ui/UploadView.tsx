import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { buttonVariants } from '@/components/ui/button'
import { Button } from '@/shared/ui/Button'
import { UploadForm } from '@/features/video-upload'
import { Card } from '@/shared/ui/Card'
import { cn } from '@/lib/utils'
import type { UploadResponse } from '@/entities/video'

export function UploadView() {
  const nav = useNavigate()
  const [done, setDone] = useState<UploadResponse | null>(null)

  return (
    <div className="mx-auto max-w-2xl px-6 py-6">
      <header className="mb-6 flex items-center justify-between gap-4">
        <h1 className="text-xl font-semibold">Upload</h1>
        <Link to="/" className={cn(buttonVariants({ variant: 'ghost' }))}>
          Home
        </Link>
      </header>
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
  )
}
