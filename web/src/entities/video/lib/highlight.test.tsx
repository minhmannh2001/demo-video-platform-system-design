import { describe, expect, it } from 'vitest'
import { parseHighlightSegments, renderSearchHighlight } from './highlight'

describe('parseHighlightSegments', () => {
  it('splits plain text and mark regions', () => {
    expect(parseHighlightSegments('foo <mark>bar</mark> baz')).toEqual([
      { kind: 'text', text: 'foo ' },
      { kind: 'mark', text: 'bar' },
      { kind: 'text', text: ' baz' },
    ])
  })

  it('supports mark with attributes', () => {
    expect(
      parseHighlightSegments('x<mark class="x">y</mark>z'),
    ).toEqual([
      { kind: 'text', text: 'x' },
      { kind: 'mark', text: 'y' },
      { kind: 'text', text: 'z' },
    ])
  })

  it('strips nested tags inside mark', () => {
    expect(parseHighlightSegments('<mark>a<b>x</b></mark>')).toEqual([
      { kind: 'mark', text: 'ax' },
    ])
  })
})

describe('renderSearchHighlight', () => {
  it('returns renderable nodes', () => {
    const nodes = renderSearchHighlight('a<mark>b</mark>c')
    expect(Array.isArray(nodes)).toBe(true)
    expect(nodes).toHaveLength(3)
  })
})
