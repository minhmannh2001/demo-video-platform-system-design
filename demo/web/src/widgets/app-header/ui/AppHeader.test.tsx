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

  it('shows Add video and Upload queue on home route', () => {
    renderAt('/')
    expect(screen.getByRole('link', { name: /^add video$/i })).toHaveAttribute('href', '/upload')
    expect(screen.getByRole('link', { name: /^upload queue$/i })).toHaveAttribute('href', '/uploads')
  })

  it('shows Home and Upload queue on add-video route', () => {
    renderAt('/upload')
    expect(screen.getByRole('link', { name: /^home$/i })).toHaveAttribute('href', '/')
    expect(screen.getByRole('link', { name: /^upload queue$/i })).toHaveAttribute('href', '/uploads')
  })

  it('shows Home, Upload queue, and Add video on watch route', () => {
    renderAt('/watch/abc123def4567890abcdef12')
    expect(screen.getByRole('link', { name: /^home$/i })).toHaveAttribute('href', '/')
    expect(screen.getByRole('link', { name: /^upload queue$/i })).toHaveAttribute('href', '/uploads')
    expect(screen.getByRole('link', { name: /^add video$/i })).toHaveAttribute('href', '/upload')
  })

  it('hides Upload queue nav on uploads list only', () => {
    renderAt('/uploads')
    expect(screen.queryByRole('link', { name: /^upload queue$/i })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: /^add video$/i })).toHaveAttribute('href', '/upload')
  })

  it('shows Upload queue nav on upload detail route', () => {
    renderAt('/uploads/abc123')
    expect(screen.getByRole('link', { name: /^upload queue$/i })).toHaveAttribute('href', '/uploads')
  })

  it('exposes main navigation landmark', () => {
    renderAt('/')
    expect(screen.getByRole('navigation', { name: /main/i })).toBeInTheDocument()
  })
})
