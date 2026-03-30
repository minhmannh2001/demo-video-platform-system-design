import { describe, expect, it } from 'vitest'
import {
  formatDurationSec,
  formatPublishedAt,
  truncateDescription,
} from './format'

describe('formatPublishedAt', () => {
  it('returns em dash for invalid iso', () => {
    expect(formatPublishedAt('not-a-date')).toBe('—')
  })

  it('returns em dash for empty string', () => {
    expect(formatPublishedAt('')).toBe('—')
  })

  it('formats valid iso for en-US locale', () => {
    const s = formatPublishedAt('2026-03-15T14:30:00.000Z')
    expect(s).toMatch(/2026/)
    expect(s).toMatch(/Mar/)
  })
})

describe('formatDurationSec', () => {
  it('formats under one hour', () => {
    expect(formatDurationSec(0)).toBe('0:00')
    expect(formatDurationSec(65)).toBe('1:05')
  })

  it('formats one hour or more', () => {
    expect(formatDurationSec(3661)).toBe('1:01:01')
  })

  it('returns em dash for invalid', () => {
    expect(formatDurationSec(NaN)).toBe('—')
    expect(formatDurationSec(-1)).toBe('—')
  })
})

describe('truncateDescription', () => {
  it('returns trimmed text when under limit', () => {
    expect(truncateDescription('  hello  ')).toBe('hello')
  })

  it('short text unchanged', () => {
    expect(truncateDescription('short')).toBe('short')
  })

  it('adds ellipsis when over maxChars', () => {
    const long = 'a'.repeat(150)
    const out = truncateDescription(long, 120)
    expect(out.length).toBeLessThanOrEqual(122)
    expect(out.endsWith('…')).toBe(true)
  })
})
