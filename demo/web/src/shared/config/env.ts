/** Public API base URL (no trailing slash). Overridable via VITE_API_URL. */
export function getApiBase(): string {
  const raw = import.meta.env.VITE_API_URL
  const base =
    typeof raw === 'string' && raw.trim() !== ''
      ? raw.trim()
      : 'http://localhost:8080'
  return base.replace(/\/$/, '')
}
