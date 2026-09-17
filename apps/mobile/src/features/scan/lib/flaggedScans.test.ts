import type { RejectedScan, ResolvedScan, ScannedCard, ScannedPrinting } from '../types'

import { flaggedScans } from './flaggedScans'

const card = (id: string): ScannedCard => ({ id, name: `Card ${id}`, imageObjectKey: null })

const printing = (id: string, cardId: string): ScannedPrinting => ({
  id,
  cardId,
  setCode: 'LOB-EN001',
  rarity: 'Ultra Rare',
})

/**
 * The five shapes `internal/card/service/match.go` can actually produce, each
 * built the way that file builds it. They are fixtures rather than inline
 * objects because *which* of them is flagged is the whole subject here.
 */
const settled = (code: string): ResolvedScan => ({
  code,
  resolution: 'exact',
  card: card('blue-eyes'),
  printing: printing('lob-001', 'blue-eyes'),
  candidates: [],
})

/** The name rung's clear winner: a card, and never a set. */
const byName = (code: string): ResolvedScan => ({
  code,
  resolution: 'by_name',
  card: card('blue-eyes'),
  printing: null,
  candidates: [],
})

/** A code rung that matched several printings of one card. */
const ambiguousPrintings = (code: string): ResolvedScan => ({
  code,
  resolution: 'unresolved',
  card: card('blue-eyes'),
  printing: null,
  candidates: [
    { card: card('blue-eyes'), printing: printing('lob-001', 'blue-eyes') },
    { card: card('blue-eyes'), printing: printing('sdk-001', 'blue-eyes') },
  ],
})

/** The name rung's tie: two different cards scored the same. */
const ambiguousCards = (code: string): ResolvedScan => ({
  code,
  resolution: 'unresolved',
  card: null,
  printing: null,
  candidates: [
    { card: card('mirror-force'), printing: null },
    { card: card('mirror-wall'), printing: null },
  ],
})

/** The bottom of the ladder: nothing matched at all. */
const nothing = (code: string): ResolvedScan => ({
  code,
  resolution: 'unresolved',
  card: null,
  printing: null,
  candidates: [],
})

const refused = (code: string): RejectedScan => ({ code, status: 422 })

describe('flaggedScans', () => {
  describe('what it flags', () => {
    it('leaves a scan the ladder settled out of the sheet', () => {
      expect(flaggedScans([settled('LOB-EN001')], [])).toEqual([])
    })

    it('flags a card matched by name as missing its set', () => {
      expect(flaggedScans([byName('LOB-EN002')], [])).toEqual([
        {
          code: 'LOB-EN002',
          reason: 'no_set',
          copies: 1,
          card: card('blue-eyes'),
          candidates: [],
        },
      ])
    })

    it('flags a code that matched nothing as unresolved', () => {
      expect(flaggedScans([nothing('SMUDGE-1')], [])).toEqual([
        { code: 'SMUDGE-1', reason: 'unresolved', copies: 1, card: null, candidates: [] },
      ])
    })

    it('flags several printings of one card as a choice, and carries them', () => {
      const [row] = flaggedScans([ambiguousPrintings('001')], [])

      expect(row?.reason).toBe('ambiguous')
      expect(row?.card).toEqual(card('blue-eyes'))
      expect(row?.candidates).toHaveLength(2)
    })

    it('flags a tie between different cards as a choice too', () => {
      const [row] = flaggedScans([ambiguousCards('MIRROR')], [])

      expect(row?.reason).toBe('ambiguous')
      expect(row?.card).toBeNull()
      expect(row?.candidates).toHaveLength(2)
    })

    // The ladder answers `unresolved` for *both* "several matched" and "nothing
    // matched", so reading the reason off `resolution` would collapse a choice
    // the user can make in one tap into a search they have to type.
    it('tells an ambiguous scan from an empty one although both resolve to unresolved', () => {
      const rows = flaggedScans([ambiguousPrintings('001'), nothing('002')], [])

      expect(rows.map((row) => row.reason)).toEqual(['ambiguous', 'unresolved'])
    })

    it('flags a scan the resolver refused, so the card is never silently dropped', () => {
      expect(flaggedScans([], [refused('!!!')])).toEqual([
        { code: '!!!', reason: 'refused', copies: 1, card: null, candidates: [] },
      ])
    })
  })

  describe('one question per code', () => {
    it('counts two captures of one flagged code as two copies of one question', () => {
      const rows = flaggedScans([byName('LOB-EN002'), byName('LOB-EN002')], [])

      expect(rows).toHaveLength(1)
      expect(rows[0]?.copies).toBe(2)
    })

    it('counts two refusals of one code as two copies', () => {
      const rows = flaggedScans([], [refused('!!!'), refused('!!!')])

      expect(rows).toHaveLength(1)
      expect(rows[0]?.copies).toBe(2)
    })

    it('lets an answer outrank a refusal of the same code', () => {
      const rows = flaggedScans([byName('LOB-EN002')], [refused('LOB-EN002')])

      expect(rows).toEqual([
        {
          code: 'LOB-EN002',
          reason: 'no_set',
          copies: 1,
          card: card('blue-eyes'),
          candidates: [],
        },
      ])
    })

    it('keeps a settled code out of the sheet even when a copy of it was refused', () => {
      expect(flaggedScans([settled('LOB-EN001')], [refused('LOB-EN001')])).toEqual([])
    })

    it('keeps the sweep order, oldest first, answers before refusals', () => {
      const rows = flaggedScans(
        [nothing('first'), byName('second')],
        [refused('third'), refused('fourth')],
      )

      expect(rows.map((row) => row.code)).toEqual(['first', 'second', 'third', 'fourth'])
    })
  })

  it('has nothing to show for a sweep that resolved cleanly', () => {
    expect(flaggedScans([], [])).toEqual([])
  })
})
