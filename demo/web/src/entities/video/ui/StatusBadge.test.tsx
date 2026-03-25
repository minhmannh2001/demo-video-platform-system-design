import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { StatusBadge } from './StatusBadge'

describe('StatusBadge', () => {
  it('renders status text', () => {
    render(<StatusBadge status="ready" />)
    expect(screen.getByTestId('status-badge')).toHaveTextContent('ready')
  })

  it('applies ready class', () => {
    const { container } = render(<StatusBadge status="ready" />)
    expect(container.querySelector('.status-badge--ready')).toBeTruthy()
  })
})
