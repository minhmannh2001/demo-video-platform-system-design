import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  browserSamplingSummary,
  defaultBrowserSampleRatio,
  formatClientHttpSpanName,
  parseBrowserTraceSampleRatio,
} from './browser-tracing'

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

describe('parseBrowserTraceSampleRatio', () => {
  it('defaults when empty or invalid', () => {
    expect(parseBrowserTraceSampleRatio(undefined, 0.2)).toBe(0.2)
    expect(parseBrowserTraceSampleRatio('  ', 0.2)).toBe(0.2)
    expect(parseBrowserTraceSampleRatio('x', 0.2)).toBe(0.2)
    expect(parseBrowserTraceSampleRatio('2', 0.2)).toBe(0.2)
  })

  it('parses ratio', () => {
    expect(parseBrowserTraceSampleRatio('0.25', defaultBrowserSampleRatio)).toBe(0.25)
    expect(parseBrowserTraceSampleRatio('1', defaultBrowserSampleRatio)).toBe(1)
    expect(parseBrowserTraceSampleRatio('0', defaultBrowserSampleRatio)).toBe(0)
  })
})

describe('browserSamplingSummary', () => {
  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('always_on when flag not true', () => {
    vi.stubEnv('VITE_OTEL_TRACE_SAMPLING_ENABLED', '')
    expect(browserSamplingSummary()).toContain('always_on')
  })

  it('shows ratio when enabled', () => {
    vi.stubEnv('VITE_OTEL_TRACE_SAMPLING_ENABLED', 'true')
    vi.stubEnv('VITE_OTEL_TRACE_SAMPLE_RATIO', '0.2')
    expect(browserSamplingSummary()).toContain('0.2')
    expect(browserSamplingSummary()).toContain('parent_based')
  })
})
