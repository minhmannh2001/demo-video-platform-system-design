import { memo } from 'react'
import { Link, useLocation } from 'react-router-dom'

import { buttonVariants } from '@/components/ui/button'
import { PageHeaderBar } from '@/shared/ui/PageChrome'
import { cn } from '@/lib/utils'

/** Shared outline style for header actions (same look on Home / Upload / Watch). */
const headerActionClass = cn(
  buttonVariants({ variant: 'outline', size: 'default' }),
  'shrink-0',
)

/**
 * Global top bar: brand (home) on the left, one primary action on the right.
 * FSD: lives in `widgets` because it composes routing + layout for every page.
 *
 * `memo`: skipping re-renders when a parent view updates unrelated state (same props — none).
 * `useLocation` still re-renders this component when the URL changes.
 */
export const AppHeader = memo(function AppHeader() {
  const { pathname } = useLocation()
  /** List page only: hide the queue link (you’re already there). Still show on `/uploads/:id`. */
  const hideQueueNav = pathname === '/uploads'
  const onAddVideo = pathname === '/upload'

  return (
    <PageHeaderBar>
      <div className="flex min-h-12 items-center justify-between gap-4">
        <Link
          to="/"
          className="text-base font-semibold tracking-tight text-foreground transition-colors hover:text-primary"
        >
          Video demo
        </Link>
        <nav
          className="flex flex-wrap items-center justify-end gap-2"
          aria-label="Main"
        >
          {pathname !== '/' ? (
            <Link to="/" className={headerActionClass}>
              Home
            </Link>
          ) : null}
          {!hideQueueNav ? (
            <Link to="/uploads" className={headerActionClass}>
              Upload queue
            </Link>
          ) : null}
          {!onAddVideo ? (
            <Link to="/upload" className={headerActionClass}>
              Add video
            </Link>
          ) : null}
        </nav>
      </div>
    </PageHeaderBar>
  )
})
