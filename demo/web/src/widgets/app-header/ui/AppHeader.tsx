import { Link, useLocation } from 'react-router-dom'

import { buttonVariants } from '@/components/ui/button'
import { PageHeaderBar } from '@/shared/ui/PageChrome'
import { cn } from '@/lib/utils'

/** Shared outline style for header actions (same look on Home / Upload / Watch). */
const headerActionClass = cn(buttonVariants({ variant: 'outline', size: 'default' }), 'shrink-0')

/**
 * Global top bar: brand (home) on the left, one primary action on the right.
 * FSD: lives in `widgets` because it composes routing + layout for every page.
 */
export function AppHeader() {
  const { pathname } = useLocation()
  const onUploadPage = pathname === '/upload'

  return (
    <PageHeaderBar>
      <div className="flex min-h-12 items-center justify-between gap-4">
        <Link
          to="/"
          className="text-base font-semibold tracking-tight text-foreground transition-colors hover:text-primary"
        >
          Video demo
        </Link>
        <nav className="flex items-center gap-2" aria-label="Main">
          {onUploadPage ? (
            <Link to="/" className={headerActionClass}>
              Home
            </Link>
          ) : (
            <Link to="/upload" className={headerActionClass}>
              Upload
            </Link>
          )}
        </nav>
      </div>
    </PageHeaderBar>
  )
}
