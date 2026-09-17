import { renderHook } from '@testing-library/react-native'

import {
  absentReadsToLeaveFrame,
  stableReadsRequired,
  useStableRead,
  type StableReader,
} from './useStableRead'

const blueEyes = 'LOB-EN001'
const darkMagician = 'LOB-EN005'

/** One read per entry; returns only the codes the reader accepted. */
const sweep = (reader: StableReader, reads: readonly (string | null)[]): string[] => {
  const accepted: string[] = []
  for (const read of reads) {
    const code = reader.observe(read)
    if (code !== null) accepted.push(code)
  }
  return accepted
}

const repeated = (read: string | null, times: number): (string | null)[] =>
  Array.from({ length: times }, () => read)

/** The card sits in the frame long enough to be accepted. */
const inFrame = (code: string): (string | null)[] => repeated(code, stableReadsRequired)

/** Long enough with no sight of a code for it to count as having left the frame. */
const outOfFrame = (): (string | null)[] => repeated(null, absentReadsToLeaveFrame)

const renderReader = async (): Promise<StableReader> => {
  const { result } = await renderHook(() => useStableRead())
  return result.current
}

describe('useStableRead', () => {
  describe('the stable-read threshold', () => {
    it('accepts a code only once it has been read the required number of times', async () => {
      const reader = await renderReader()

      const beforeThreshold = sweep(reader, repeated(blueEyes, stableReadsRequired - 1))
      expect(beforeThreshold).toEqual([])

      expect(reader.observe(blueEyes)).toBe(blueEyes)
    })

    it('ignores a one-frame misread between two reads of the real code', async () => {
      const reader = await renderReader()

      expect(sweep(reader, [blueEyes, 'L0B-EN00I', blueEyes, blueEyes])).toEqual([])
    })

    it('starts the count again after a frame that carried no code', async () => {
      const reader = await renderReader()

      expect(sweep(reader, [blueEyes, blueEyes, null, blueEyes, blueEyes])).toEqual([])
      expect(reader.observe(blueEyes)).toBe(blueEyes)
    })

    it('accepts nothing while two cards alternate under an unsteady hand', async () => {
      const reader = await renderReader()

      const alternating = [blueEyes, darkMagician, blueEyes, darkMagician, blueEyes]
      expect(sweep(reader, alternating)).toEqual([])
    })
  })

  describe('the same card, still in the frame', () => {
    it('accepts a card held under the lens exactly once', async () => {
      const reader = await renderReader()

      expect(sweep(reader, repeated(blueEyes, stableReadsRequired * 5))).toEqual([blueEyes])
    })

    it('does not re-accept it after a blink that never reached the leave threshold', async () => {
      const reader = await renderReader()
      const blink = repeated(null, absentReadsToLeaveFrame - 1)

      expect(sweep(reader, [...inFrame(blueEyes), ...blink, ...inFrame(blueEyes)])).toEqual([
        blueEyes,
      ])
    })

    it('does not re-accept it when the glimpse of another card was too short', async () => {
      const reader = await renderReader()
      const glimpse = repeated(darkMagician, absentReadsToLeaveFrame - 1)

      expect(sweep(reader, [...inFrame(blueEyes), ...glimpse, ...inFrame(blueEyes)])).toEqual([
        blueEyes,
      ])
    })
  })

  describe('a second copy, met after the first left the frame', () => {
    it('accepts the same code again once the frame went empty for long enough', async () => {
      const reader = await renderReader()

      const twoCopies = [...inFrame(blueEyes), ...outOfFrame(), ...inFrame(blueEyes)]
      expect(sweep(reader, twoCopies)).toEqual([blueEyes, blueEyes])
    })

    it('accepts the same code again when another card held the frame in between', async () => {
      const reader = await renderReader()

      const pastAnotherCard = [
        ...inFrame(blueEyes),
        ...inFrame(darkMagician),
        ...inFrame(blueEyes),
      ]
      expect(sweep(reader, pastAnotherCard)).toEqual([blueEyes, darkMagician, blueEyes])
    })

    it('accepts a third copy the same way', async () => {
      const reader = await renderReader()

      const threeCopies = [
        ...inFrame(blueEyes),
        ...outOfFrame(),
        ...inFrame(blueEyes),
        ...outOfFrame(),
        ...inFrame(blueEyes),
      ]
      expect(sweep(reader, threeCopies)).toEqual([blueEyes, blueEyes, blueEyes])
    })
  })

  it('keeps its state across renders', async () => {
    const { result, rerender } = await renderHook(() => useStableRead())

    sweep(result.current, repeated(blueEyes, stableReadsRequired - 1))
    await rerender(undefined)

    expect(result.current.observe(blueEyes)).toBe(blueEyes)
  })
})
