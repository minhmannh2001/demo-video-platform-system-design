import type { ReactNode } from 'react'

import { Card as ShadCard, CardContent } from '@/components/ui/card'

export function Card({ children }: { children: ReactNode }) {
  return (
    <ShadCard>
      <CardContent className="pt-6">{children}</CardContent>
    </ShadCard>
  )
}
