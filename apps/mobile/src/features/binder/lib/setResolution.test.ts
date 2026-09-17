import type { SetResolution } from '../types'

import { hasKnownSet, resolutionLabelKeys } from './setResolution'

const everyRung: SetResolution[] = [
  'exact',
  'by_prefix_and_number',
  'by_number',
  'by_name',
  'unresolved',
]

describe('resolutionLabelKeys', () => {
  it('names every rung of the ladder', () => {
    expect(Object.keys(resolutionLabelKeys).sort()).toEqual([...everyRung].sort())
  })

  it.each(everyRung)('gives %s a key under the binder catalogue', (rung) => {
    expect(resolutionLabelKeys[rung]).toMatch(/^binder\.resolution\./)
  })
})

describe('hasKnownSet', () => {
  it.each<[SetResolution, boolean]>([
    ['exact', true],
    ['by_prefix_and_number', true],
    ['by_number', true],
    ['by_name', false],
    ['unresolved', false],
  ])('%s records a set: %s', (rung, known) => {
    expect(hasKnownSet(rung)).toBe(known)
  })
})
