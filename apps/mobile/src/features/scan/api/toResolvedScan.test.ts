import type { ScanMatchBody } from '../types'

import { toResolvedScan } from './toResolvedScan'

const card = {
  id: '6d9f2f2c-0f3e-4a1f-9d4e-1f7c2b8a3d55',
  name: 'Blue-Eyes White Dragon',
  imageObjectKey: 'cards/89631139.jpg',
}

const printing = {
  id: 'b1c2d3e4-f5a6-4b7c-8d9e-0f1a2b3c4d5e',
  cardId: card.id,
  setCode: 'LOB-EN001',
  rarity: 'Ultra Rare',
}

const candidateCard = {
  id: '9c8b7a65-4321-4fed-9876-543210fedcba',
  name: 'Blue-Eyes Alternative White Dragon',
  imageObjectKey: null,
}

const candidatePrinting = {
  id: '11112222-3333-4444-5555-666677778888',
  cardId: candidateCard.id,
  setCode: 'MVP1-ENS55',
  rarity: 'Secret Rare',
}

/** Every field of the response body set to a distinct, non-zero value. */
const fullMatch: ScanMatchBody = {
  $schema: 'https://api.binder.test/schemas/ScanMatch.json',
  resolution: 'by_prefix_and_number',
  card,
  printing,
  candidates: [{ card: candidateCard, printing: candidatePrinting }],
}

describe('toResolvedScan', () => {
  it('carries every field of the answer, plus the code that was scanned', () => {
    expect(toResolvedScan('LOB-EN001', fullMatch)).toEqual({
      code: 'LOB-EN001',
      resolution: 'by_prefix_and_number',
      card,
      printing,
      candidates: [{ card: candidateCard, printing: candidatePrinting }],
    })
  })

  it('keeps the code the camera read even when the ladder resolved nothing', () => {
    const unresolved: ScanMatchBody = {
      resolution: 'unresolved',
      card: null,
      printing: null,
      candidates: [],
    }

    expect(toResolvedScan('SDK-001', unresolved)).toEqual({
      code: 'SDK-001',
      resolution: 'unresolved',
      card: null,
      printing: null,
      candidates: [],
    })
  })

  it('carries a candidate the name rung produced, which names a card and no set', () => {
    const byName: ScanMatchBody = {
      resolution: 'by_name',
      card,
      printing: null,
      candidates: [{ card: candidateCard, printing: null }],
    }

    const resolved = toResolvedScan('LOB-EN001', byName)

    expect(resolved.printing).toBeNull()
    expect(resolved.candidates).toEqual([{ card: candidateCard, printing: null }])
  })
})
