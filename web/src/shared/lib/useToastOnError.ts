import { useEffect, useRef } from 'react'
import { toast } from '@/shared/ui/sonner'

/**
 * Toast when `error` becomes non-null or changes. Clears internal seen state when `error` is null.
 * Call once per independent source (e.g. `useToastOnError(pollError)` and `useToastOnError(formError)`).
 * Avoids spam when the same string is re-set on interval refetch.
 */
export function useToastOnError(error: string | null) {
  const prev = useRef<string | null>(null)
  useEffect(() => {
    if (error) {
      if (error !== prev.current) toast.error(error)
      prev.current = error
    } else {
      prev.current = null
    }
  }, [error])
}
