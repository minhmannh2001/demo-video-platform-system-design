import { describe, expect, it } from 'vitest'

import { formatClientHttpSpanName } from './browser-tracing'

describe('formatClientHttpSpanName', () => {
  it('normalizes method and strips origin', () => {
    expect(
      formatClientHttpSpanName('get', 'http://localhost:8080/videos', 'http://localhost'),
    ).toBe('GET /videos')
  })

  it('keeps query string', () => {
    expect(
      formatClientHttpSpanName('GET', 'http://localhost:8080/videos?q=1', 'http://localhost'),
    ).toBe('GET /videos?q=1')
  })
})
