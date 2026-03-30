import { isFailed, isProcessing, isReady } from '../model/status'
import type { VideoStatus } from '../model/types'
import { cn } from '@/lib/utils'

type Props = { status: VideoStatus }

export function StatusBadge({ status }: Props) {
  return (
    <span
      className={cn(
        'inline-block rounded px-1.5 py-0.5 text-xs font-medium uppercase',
        isReady(status) && 'bg-emerald-950 text-emerald-200',
        isProcessing(status) && 'bg-amber-950 text-amber-200',
        isFailed(status) && 'bg-red-950 text-red-200',
        !isReady(status) && !isProcessing(status) && !isFailed(status) && 'bg-muted text-muted-foreground',
      )}
      data-testid="status-badge"
    >
      {status}
    </span>
  )
}
