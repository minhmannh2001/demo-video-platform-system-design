import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Video as VideoIcon } from 'lucide-react'

import { Card, CardContent } from '@/components/ui/card'
import { cn } from '@/lib/utils'
import type { Video as VideoModel } from '../model/types'
import { formatPublishedAt, truncateDescription } from '../lib/format'
import { renderSearchHighlight } from '../lib/highlight'
import { StatusBadge } from './StatusBadge'

type Props = {
  video: VideoModel
  className?: string
  /** First Elasticsearch highlight fragment for title (HTML string from API). */
  titleHighlight?: string
  /** First Elasticsearch highlight fragment for description. */
  descriptionHighlight?: string
}

export function VideoCard({
  video,
  className,
  titleHighlight,
  descriptionHighlight,
}: Props) {
  const [thumbFailed, setThumbFailed] = useState(false)
  useEffect(() => {
    setThumbFailed(false)
  }, [video.id, video.thumbnail_url])

  const desc = video.description?.trim()
  const preview = desc ? truncateDescription(desc) : null
  const descFromHighlight =
    descriptionHighlight?.trim() &&
    renderSearchHighlight(descriptionHighlight.trim())

  const showThumbnail = Boolean(video.thumbnail_url) && !thumbFailed

  return (
    <Link
      to={`/watch/${video.id}`}
      aria-label={`Watch ${video.title}`}
      className={cn(
        'block outline-none transition focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background',
        className,
      )}
    >
      <Card className="h-full gap-0 overflow-hidden py-0 transition-shadow hover:shadow-md hover:ring-1 hover:ring-primary/25">
        <div
          className={cn(
            'relative aspect-video w-full overflow-hidden',
            showThumbnail ? 'bg-muted/30' : 'bg-card p-2.5 sm:p-3',
          )}
        >
          {showThumbnail && video.thumbnail_url ? (
            <img
              src={video.thumbnail_url}
              alt=""
              className="h-full w-full object-cover"
              loading="lazy"
              onError={() => setThumbFailed(true)}
            />
          ) : (
            <div
              className="flex h-full min-h-[8rem] w-full flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-border/90 bg-muted/15 text-muted-foreground"
              aria-hidden
            >
              <VideoIcon
                className="size-11 text-muted-foreground/60"
                strokeWidth={1.25}
              />
              <span className="text-xs font-medium tracking-wide text-muted-foreground/90">
                No preview
              </span>
            </div>
          )}
        </div>
        <CardContent className="flex flex-col gap-3 px-5 pb-5 pt-4">
          <h2 className="line-clamp-2 text-base font-semibold leading-snug text-foreground [&_mark]:text-foreground">
            {titleHighlight?.trim()
              ? renderSearchHighlight(titleHighlight.trim())
              : video.title}
          </h2>
          <p className="text-xs text-muted-foreground">
            <span className="font-medium text-foreground/80">
              {video.uploader || 'Unknown'}
            </span>
            <span className="mx-1.5 text-border">·</span>
            <time dateTime={video.created_at}>
              {formatPublishedAt(video.created_at)}
            </time>
          </p>
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge status={video.status} />
          </div>
          <div className="border-t border-border/60 pt-3">
            {descFromHighlight ? (
              <p className="line-clamp-2 text-sm leading-relaxed text-muted-foreground [&_mark]:text-foreground/90">
                {descFromHighlight}
              </p>
            ) : preview ? (
              <p className="line-clamp-2 text-sm leading-relaxed text-muted-foreground">
                {preview}
              </p>
            ) : (
              <p className="text-sm italic text-muted-foreground/80">
                No description
              </p>
            )}
          </div>
        </CardContent>
      </Card>
    </Link>
  )
}
