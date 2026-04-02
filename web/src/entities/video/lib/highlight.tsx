import type { ReactNode } from 'react'

type Segment = { kind: 'text' | 'mark'; text: string }

/** Strip any HTML inside a highlight fragment (defense in depth; ES should only emit &lt;mark&gt;). */
function stripInnerTags(s: string): string {
  return s.replace(/<[^>]+>/g, '')
}

/**
 * Parses Elasticsearch highlight HTML (expected: plain text + &lt;mark&gt;…&lt;/mark&gt;).
 * Unknown tags in outer HTML are dropped; content inside &lt;mark&gt; is text-only after stripping nested tags.
 */
export function parseHighlightSegments(html: string): Segment[] {
  const out: Segment[] = []
  let rest = html
  while (rest.length > 0) {
    const idx = rest.toLowerCase().search(/<mark\b/)
    if (idx === -1) {
      if (rest) out.push({ kind: 'text', text: rest })
      break
    }
    if (idx > 0) out.push({ kind: 'text', text: rest.slice(0, idx) })
    const after = rest.slice(idx)
    const mOpen = after.match(/^<mark\b[^>]*>/i)
    if (!mOpen) {
      out.push({ kind: 'text', text: rest })
      break
    }
    const contentStart = mOpen[0].length
    const closeIdx = after.toLowerCase().indexOf('</mark>', contentStart)
    if (closeIdx === -1) {
      out.push({ kind: 'text', text: rest })
      break
    }
    const inner = stripInnerTags(after.slice(contentStart, closeIdx))
    out.push({ kind: 'mark', text: inner })
    rest = after.slice(closeIdx + '</mark>'.length)
  }
  return out
}

const markClassName =
  'rounded-sm bg-amber-200/90 px-0.5 text-inherit dark:bg-amber-500/35'

/** Renders one ES highlight string as React nodes (no raw HTML injection). */
export function renderSearchHighlight(html: string): ReactNode {
  return parseHighlightSegments(html).map((seg, i) =>
    seg.kind === 'mark' ? (
      <mark key={i} className={markClassName}>
        {seg.text}
      </mark>
    ) : (
      <span key={i}>{seg.text}</span>
    ),
  )
}
