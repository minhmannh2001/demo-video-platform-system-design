import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const toastError = vi.fn()

vi.mock('@/shared/ui/sonner', () => ({
  toast: {
    error: (...args: unknown[]) => toastError(...args),
  },
}))

import { useToastOnError } from './useToastOnError'

function TestHarness() {
  const [err, setErr] = useState<string | null>(null)
  useToastOnError(err)
  return (
    <div>
      <button type="button" onClick={() => setErr('e1')}>
        set1
      </button>
      <button type="button" onClick={() => setErr('e2')}>
        set2
      </button>
      <button type="button" onClick={() => setErr(null)}>
        clear
      </button>
    </div>
  )
}

describe('useToastOnError', () => {
  beforeEach(() => {
    toastError.mockClear()
  })

  it('calls toast.error when error becomes non-null', async () => {
    const user = userEvent.setup()
    render(<TestHarness />)
    await user.click(screen.getByRole('button', { name: 'set1' }))
    expect(toastError).toHaveBeenCalledTimes(1)
    expect(toastError).toHaveBeenCalledWith('e1')
  })

  it('does not toast again while the same error string stays set', async () => {
    const user = userEvent.setup()
    render(<TestHarness />)
    await user.click(screen.getByRole('button', { name: 'set1' }))
    expect(toastError).toHaveBeenCalledTimes(1)
    await user.click(screen.getByRole('button', { name: 'set1' }))
    expect(toastError).toHaveBeenCalledTimes(1)
  })

  it('toasts again after clear then the same message', async () => {
    const user = userEvent.setup()
    render(<TestHarness />)
    await user.click(screen.getByRole('button', { name: 'set1' }))
    await user.click(screen.getByRole('button', { name: 'clear' }))
    await user.click(screen.getByRole('button', { name: 'set1' }))
    expect(toastError).toHaveBeenCalledTimes(2)
    expect(toastError).toHaveBeenNthCalledWith(1, 'e1')
    expect(toastError).toHaveBeenNthCalledWith(2, 'e1')
  })

  it('toasts when the error message changes', async () => {
    const user = userEvent.setup()
    render(<TestHarness />)
    await user.click(screen.getByRole('button', { name: 'set1' }))
    await user.click(screen.getByRole('button', { name: 'set2' }))
    expect(toastError).toHaveBeenCalledTimes(2)
    expect(toastError).toHaveBeenNthCalledWith(2, 'e2')
  })

  it('clears dedupe state when error becomes null', async () => {
    const user = userEvent.setup()
    render(<TestHarness />)
    await user.click(screen.getByRole('button', { name: 'set2' }))
    await user.click(screen.getByRole('button', { name: 'clear' }))
    await user.click(screen.getByRole('button', { name: 'set2' }))
    expect(toastError).toHaveBeenCalledTimes(2)
  })
})
