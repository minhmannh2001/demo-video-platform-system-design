import { describe, expect, it } from 'vitest'
import { Toaster, toast } from './sonner'

describe('sonner wrapper', () => {
  it('exports Toaster and toast helpers from sonner', () => {
    expect(typeof Toaster).toBe('function')
    expect(typeof toast).toBe('function')
    expect(typeof toast.success).toBe('function')
    expect(typeof toast.error).toBe('function')
    expect(typeof toast.promise).toBe('function')
  })
})
