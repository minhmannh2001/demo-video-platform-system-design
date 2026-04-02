/** Formats ISO date for list/detail UI. */
export function formatPublishedAt(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return new Intl.DateTimeFormat('en-US', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(d)
}

/** Formats duration in seconds as `m:ss` or `h:mm:ss` for detail UI. */
export function formatDurationSec(totalSec: number): string {
  if (!Number.isFinite(totalSec) || totalSec < 0) return '—'
  const s = Math.floor(totalSec % 60)
  const m = Math.floor((totalSec / 60) % 60)
  const h = Math.floor(totalSec / 3600)
  if (h > 0) {
    return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
  }
  return `${m}:${s.toString().padStart(2, '0')}`
}

/** Truncates plain text for card previews (ellipsis via CSS preferred in UI). */
export function truncateDescription(text: string, maxChars = 120): string {
  const t = text.trim()
  if (t.length <= maxChars) return t
  return `${t.slice(0, maxChars).trim()}…`
}
