import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { PageHeaderBar, PageMain, PAGE_CONTAINER_CLASS } from './PageChrome'

describe('PageChrome', () => {
  it('PageHeaderBar renders children inside constrained container', () => {
    render(
      <PageHeaderBar>
        <span>header content</span>
      </PageHeaderBar>,
    )
    const header = screen.getByRole('banner')
    expect(header.className).toMatch(/sticky/)
    expect(header.className).toMatch(/top-0/)
    expect(screen.getByText('header content')).toBeInTheDocument()
    const inner = screen.getByText('header content').parentElement
    expect(inner?.className).toContain(PAGE_CONTAINER_CLASS)
  })

  it('PageMain applies default container class', () => {
    render(
      <PageMain>
        <span>main content</span>
      </PageMain>,
    )
    const main = screen.getByRole('main')
    expect(main.className).toContain(PAGE_CONTAINER_CLASS)
    expect(screen.getByText('main content')).toBeInTheDocument()
  })

  it('PageMain respects containerClass override', () => {
    render(
      <PageMain containerClass="max-w-none">
        <span>x</span>
      </PageMain>,
    )
    const main = screen.getByRole('main')
    expect(main.className).toContain('max-w-none')
    expect(main.className).not.toContain(PAGE_CONTAINER_CLASS)
  })
})
