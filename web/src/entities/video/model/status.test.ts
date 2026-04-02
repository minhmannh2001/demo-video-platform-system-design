import { describe, expect, it } from 'vitest'
import {
  effectiveVideoStatus,
  isFailed,
  isProcessing,
  isReady,
} from './status'

describe('video status helpers', () => {
  it('isProcessing', () => {
    expect(isProcessing('processing')).toBe(true)
    expect(isProcessing('ready')).toBe(false)
  })

  it('isReady', () => {
    expect(isReady('ready')).toBe(true)
    expect(isReady('processing')).toBe(false)
  })

  it('isFailed', () => {
    expect(isFailed('failed')).toBe(true)
    expect(isFailed('ready')).toBe(false)
  })

  it('effectiveVideoStatus prefers terminal watch over stale video', () => {
    expect(
      effectiveVideoStatus('processing', { status: 'failed' }),
    ).toBe('failed')
    expect(
      effectiveVideoStatus('processing', { status: 'ready' }),
    ).toBe('ready')
    expect(
      effectiveVideoStatus('processing', { status: 'processing' }),
    ).toBe('processing')
    expect(effectiveVideoStatus('ready', null)).toBe('ready')
  })
})
