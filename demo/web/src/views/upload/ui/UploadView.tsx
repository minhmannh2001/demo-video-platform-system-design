import { useNavigate } from 'react-router-dom'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { UploadForm } from '@/features/video-upload'
import { PageMain } from '@/shared/ui/PageChrome'
import { AppHeader } from '@/widgets/app-header'

export function UploadView() {
  const nav = useNavigate()

  return (
    <div className="min-h-screen bg-background">
      <AppHeader />

      <PageMain>
        <div className="mx-auto max-w-2xl">
          <Card className="gap-0 overflow-hidden py-0 shadow-sm ring-1 ring-foreground/5">
            <CardHeader className="border-b border-border bg-muted/30 px-6 pb-4 pt-6">
              <CardTitle className="text-lg tracking-tight">
                Upload a video
              </CardTitle>
              <CardDescription>
                Add metadata and a file — after upload you’ll go to the status
                page. The video appears in{' '}
                <strong className="font-medium text-foreground">
                  Upload queue
                </strong>{' '}
                once the API saves it.
              </CardDescription>
            </CardHeader>
            <CardContent className="bg-card px-6 pb-6 pt-6">
              <UploadForm
                onUploaded={(r) => {
                  nav(`/uploads/${r.id}`)
                }}
              />
            </CardContent>
          </Card>
        </div>
      </PageMain>
    </div>
  )
}
