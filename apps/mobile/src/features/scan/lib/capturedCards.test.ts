import type { ResolvedScan } from '../types'

import { capturedCards, type CapturedCard } from './capturedCards'

/** A resolved scan with every field set to something distinct, for the completeness test. */
const resolvedScan = (code: string, name: string): ResolvedScan => ({
  code,
  resolution: 'exact',
  card: { id: `card-${code}`, name, imageObjectKey: `art/${code}.jpg` },
  printing: { id: `printing-${code}`, cardId: `card-${code}`, setCode: 'LOB', rarity: 'Ultra Rare' },
  candidates: [],
})

const unmatchedScan = (code: string): ResolvedScan => ({
  code,
  resolution: 'unresolved',
  card: null,
  printing: null,
  candidates: [],
})

describe('capturedCards', () => {
  it('carries every field of a resolved scan the strip renders', () => {
    const captured = capturedCards(
      ['LOB-EN001'],
      [resolvedScan('LOB-EN001', 'Blue-Eyes White Dragon')],
      [],
    )

    const expected: CapturedCard[] = [
      { key: '0:LOB-EN001', code: 'LOB-EN001', status: 'matched', name: 'Blue-Eyes White Dragon' },
    ]
    expect(captured).toEqual(expected)
  })

  it('shows a capture the resolver has not answered about as pending', () => {
    expect(capturedCards(['LOB-EN001'], [], [])).toEqual([
      { key: '0:LOB-EN001', code: 'LOB-EN001', status: 'pending', name: null },
    ])
  })

  it('shows a capture nothing matched as unmatched, not as a name', () => {
    expect(capturedCards(['ZZZ-EN999'], [unmatchedScan('ZZZ-EN999')], [])).toEqual([
      { key: '0:ZZZ-EN999', code: 'ZZZ-EN999', status: 'unmatched', name: null },
    ])
  })

  it('shows a capture the resolver refused as refused', () => {
    expect(capturedCards(['LOB-EN001'], [], [{ code: 'LOB-EN001', status: 422 }])).toEqual([
      { key: '0:LOB-EN001', code: 'LOB-EN001', status: 'refused', name: null },
    ])
  })

  it('puts the newest capture first, because that is the one being confirmed', () => {
    const captured = capturedCards(['LOB-EN001', 'SDK-002', 'MP21-EN003'], [], [])

    expect(captured.map((card) => card.code)).toEqual(['MP21-EN003', 'SDK-002', 'LOB-EN001'])
  })

  it('keeps two copies of one card as two rows with different keys', () => {
    const captured = capturedCards(
      ['LOB-EN001', 'LOB-EN001'],
      [resolvedScan('LOB-EN001', 'Blue-Eyes White Dragon')],
      [],
    )

    expect(captured).toEqual([
      { key: '1:LOB-EN001', code: 'LOB-EN001', status: 'matched', name: 'Blue-Eyes White Dragon' },
      { key: '0:LOB-EN001', code: 'LOB-EN001', status: 'matched', name: 'Blue-Eyes White Dragon' },
    ])
  })

  it('leaves a capture pending while an earlier code is still in flight', () => {
    const captured = capturedCards(
      ['LOB-EN001', 'SDK-002'],
      [resolvedScan('SDK-002', 'Dark Magician')],
      [],
    )

    expect(captured.map((card) => card.status)).toEqual(['matched', 'pending'])
  })
})
