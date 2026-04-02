import { afterEach, describe, expect, it, vi } from 'vitest'
import { getApiBase, getWebSocketUrl } from './env'

describe('getApiBase', () => {
  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('trims trailing slash from VITE_API_URL', () => {
    vi.stubEnv('VITE_API_URL', 'http://api.example.com/')
    expect(getApiBase()).toBe('http://api.example.com')
  })

  it('uses default when unset', () => {
    vi.stubEnv('VITE_API_URL', '')
    expect(getApiBase()).toBe('http://localhost:8080')
  })
})

describe('getWebSocketUrl', () => {
  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('maps http to ws and appends /ws', () => {
    vi.stubEnv('VITE_API_URL', 'http://localhost:8080')
    vi.stubEnv('VITE_WEBSOCKET_TOKEN', '')
    expect(getWebSocketUrl()).toBe('ws://localhost:8080/ws')
  })

  it('maps https to wss', () => {
    vi.stubEnv('VITE_API_URL', 'https://api.example.com')
    expect(getWebSocketUrl()).toBe('wss://api.example.com/ws')
  })

  it('adds token query when set', () => {
    vi.stubEnv('VITE_API_URL', 'http://localhost:8080')
    vi.stubEnv('VITE_WEBSOCKET_TOKEN', 'secret')
    expect(getWebSocketUrl()).toBe('ws://localhost:8080/ws?token=secret')
  })
})
