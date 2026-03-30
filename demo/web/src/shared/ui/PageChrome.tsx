import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

/** Same width for header + main on every route so layout lines up (Home ↔ Watch ↔ Upload). */
export const PAGE_CONTAINER_CLASS = 'max-w-6xl'

type Props = {
  children: ReactNode
  /** Override inner max-width (default: {@link PAGE_CONTAINER_CLASS}). */
  containerClass?: string
  className?: string
}

/** Top app bar: distinct from page body (border + muted surface). */
export function PageHeaderBar({ children, containerClass = PAGE_CONTAINER_CLASS, className }: Props) {
  return (
    <header
      className={cn(
        'border-b border-border bg-muted/40',
        className,
      )}
    >
      <div className={cn('mx-auto px-4 py-5 sm:px-6', containerClass)}>{children}</div>
    </header>
  )
}

/** Primary page content below the header. */
export function PageMain({ children, containerClass = PAGE_CONTAINER_CLASS, className }: Props) {
  return (
    <main className={cn('mx-auto px-4 py-8 sm:px-6', containerClass, className)}>{children}</main>
  )
}
