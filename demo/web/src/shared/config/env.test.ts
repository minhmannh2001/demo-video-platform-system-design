import { afterEach, describe, expect, it, vi } from 'vitest'
import { getApiBase } from './env'

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
