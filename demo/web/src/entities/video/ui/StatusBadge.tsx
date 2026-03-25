import { isFailed, isProcessing, isReady } from '../model/status'
import type { VideoStatus } from '../model/types'

type Props = { status: VideoStatus }

export function StatusBadge({ status }: Props) {
  let className = 'status-badge'
  if (isReady(status)) className += ' status-badge--ready'
  else if (isProcessing(status)) className += ' status-badge--processing'
  else if (isFailed(status)) className += ' status-badge--failed'

  return (
    <span className={className} data-testid="status-badge">
      {status}
    </span>
  )
}
