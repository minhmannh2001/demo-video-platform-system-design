import { describe, expect, it } from 'vitest'
import { cn } from './utils'

describe('cn', () => {
  it('joins class names', () => {
    expect(cn('foo', 'bar')).toContain('foo')
    expect(cn('foo', 'bar')).toContain('bar')
  })

  it('merges tailwind conflicts (later wins)', () => {
    expect(cn('p-2', 'p-4')).toBe('p-4')
  })

  it('handles conditional falsy inputs', () => {
    expect(cn('base', false && 'no', undefined, null, 'yes')).toContain('base')
    expect(cn('base', false && 'no', undefined, null, 'yes')).toContain('yes')
  })
})
