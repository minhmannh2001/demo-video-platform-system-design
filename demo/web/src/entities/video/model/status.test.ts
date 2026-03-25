import { describe, expect, it } from 'vitest'
import { isFailed, isProcessing, isReady } from './status'

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
})
