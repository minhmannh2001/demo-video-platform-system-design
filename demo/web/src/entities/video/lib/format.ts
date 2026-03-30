/** Formats ISO date for list/detail UI. */
export function formatPublishedAt(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return new Intl.DateTimeFormat('en-US', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(d)
}

/** Truncates plain text for card previews (ellipsis via CSS preferred in UI). */
export function truncateDescription(text: string, maxChars = 120): string {
  const t = text.trim()
  if (t.length <= maxChars) return t
  return `${t.slice(0, maxChars).trim()}…`
}
