import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { AppHeader } from './AppHeader'

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppHeader />
    </MemoryRouter>,
  )
}

describe('AppHeader', () => {
  it('renders brand link to home', () => {
    renderAt('/')
    const brand = screen.getByRole('link', { name: /video demo/i })
    expect(brand).toHaveAttribute('href', '/')
  })

  it('shows Upload on home route', () => {
    renderAt('/')
    expect(screen.getByRole('link', { name: /^upload$/i })).toHaveAttribute('href', '/upload')
  })

  it('shows Upload on watch route', () => {
    renderAt('/watch/abc123def4567890abcdef12')
    expect(screen.getByRole('link', { name: /^upload$/i })).toHaveAttribute('href', '/upload')
  })

  it('shows Home on upload route', () => {
    renderAt('/upload')
    expect(screen.getByRole('link', { name: /^home$/i })).toHaveAttribute('href', '/')
  })

  it('exposes main navigation landmark', () => {
    renderAt('/')
    expect(screen.getByRole('navigation', { name: /main/i })).toBeInTheDocument()
  })
})
