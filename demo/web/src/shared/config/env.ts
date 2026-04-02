/** Public API base URL (no trailing slash). Overridable via VITE_API_URL. */
export function getApiBase(): string {
  const raw = import.meta.env.VITE_API_URL
  const base =
    typeof raw === 'string' && raw.trim() !== ''
      ? raw.trim()
      : 'http://localhost:8080'
  return base.replace(/\/$/, '')
}

/**
 * WebSocket URL for GET /ws (same host as API). Optional VITE_WEBSOCKET_TOKEN → ?token=
 */
export function getWebSocketUrl(): string {
  const httpBase = getApiBase()
  const wsOrigin = httpBase.replace(/^http/i, (m) =>
    m.toLowerCase() === 'https' ? 'wss' : 'ws',
  )
  const u = new URL(`${wsOrigin}/ws`)
  const tok = import.meta.env.VITE_WEBSOCKET_TOKEN
  if (typeof tok === 'string' && tok.trim() !== '') {
    u.searchParams.set('token', tok.trim())
  }
  return u.toString()
}
