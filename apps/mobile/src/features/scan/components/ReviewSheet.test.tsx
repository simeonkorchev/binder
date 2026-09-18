import { render, screen, userEvent } from '@testing-library/react-native'

import '@/i18n/i18n'

import type { SearchCardsBody } from '@/features/card/types'

import type { RejectedScan, ResolvedScan, ScannedCard } from '../types'
import { useScanReview } from '../useScanReview'

import { ReviewSheet } from './ReviewSheet'

const apiBaseUrl = 'https://api.binder.test'
const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

const card = (id: string, name: string): ScannedCard => ({ id, name, imageObjectKey: null })

const blueEyes = card('blue-eyes', 'Blue-Eyes White Dragon')
const darkMagician = card('dark-magician', 'Dark Magician')

/** The name rung's clear winner: a card, and no set. The case US3 names. */
const byName = (code: string): ResolvedScan => ({
  code,
  resolution: 'by_name',
  outcome: 'card_only',
  card: blueEyes,
  printing: null,
  candidates: [],
})

/** A code rung that matched two printings of one card. */
const ambiguous = (code: string): ResolvedScan => ({
  code,
  resolution: 'unresolved',
  outcome: 'ambiguous',
  card: blueEyes,
  printing: null,
  candidates: [
    {
      card: blueEyes,
      printing: { id: 'lob-001', cardId: 'blue-eyes', setCode: 'LOB-001', rarity: 'Ultra Rare' },
    },
    {
      card: blueEyes,
      printing: { id: 'sdk-001', cardId: 'blue-eyes', setCode: 'SDK-001', rarity: 'Common' },
    },
  ],
})

/** The bottom of the ladder: nothing matched at all. */
const nothing = (code: string): ResolvedScan => ({
  code,
  resolution: 'unresolved',
  outcome: 'no_match',
  card: null,
  printing: null,
  candidates: [],
})

const refused = (code: string): RejectedScan => ({ code, status: 422 })

/**
 * The sheet driven by the real review hook, because what is worth asserting is
 * what a user sees after pressing: a stubbed `ScanReview` would prove the
 * button calls a function and nothing about the answer coming back.
 */
const Harness = ({
  resolved = [],
  rejected = [],
  onClose = (): void => undefined,
}: {
  resolved?: ResolvedScan[]
  rejected?: RejectedScan[]
  onClose?: () => void
}): React.JSX.Element => {
  const review = useScanReview(resolved, rejected)
  return <ReviewSheet visible review={review} onClose={onClose} />
}

describe('ReviewSheet', () => {
  beforeEach(() => {
    mockFetch.mockReset()
    globalThis.fetch = mockFetch
    process.env.EXPO_PUBLIC_API_URL = apiBaseUrl
  })

  it('says the sweep is clean rather than showing a blank sheet', async () => {
    await render(<Harness />)

    expect(
      screen.getByText('Nothing to check. Every card this sweep captured was matched to a set.'),
    ).toBeOnTheScreen()
  })

  describe('a card with no set', () => {
    it('says which code it was and what is missing', async () => {
      await render(<Harness resolved={[byName('LOB-EN002')]} />)

      expect(screen.getByText('Scanned code LOB-EN002')).toBeOnTheScreen()
      expect(screen.getByText('Found the card, but not which set it is from.')).toBeOnTheScreen()
    })

    it('confirms the card the ladder named, with its set left open', async () => {
      const user = userEvent.setup()
      await render(<Harness resolved={[byName('LOB-EN002')]} />)

      await user.press(
        screen.getByRole('button', { name: 'Keep Blue-Eyes White Dragon, set unknown' }),
      )

      expect(
        await screen.findByText('Keeping Blue-Eyes White Dragon, set unknown'),
      ).toBeOnTheScreen()
    })

    it('settles every copy of the card with one decision', async () => {
      await render(<Harness resolved={[byName('LOB-EN002'), byName('LOB-EN002')]} />)

      expect(screen.getByText('2 copies captured')).toBeOnTheScreen()
      expect(
        screen.getAllByRole('button', { name: 'Keep Blue-Eyes White Dragon, set unknown' }),
      ).toHaveLength(1)
    })
  })

  describe('a scan that matched several printings', () => {
    it('offers each candidate by name and set', async () => {
      await render(<Harness resolved={[ambiguous('001')]} />)

      expect(screen.getByText('More than one card matches. Pick the one you are holding.')).toBeOnTheScreen()
      expect(
        screen.getByRole('button', { name: 'Blue-Eyes White Dragon — LOB-001' }),
      ).toBeOnTheScreen()
      expect(
        screen.getByRole('button', { name: 'Blue-Eyes White Dragon — SDK-001' }),
      ).toBeOnTheScreen()
    })

    it('records the printing the user picked', async () => {
      const user = userEvent.setup()
      await render(<Harness resolved={[ambiguous('001')]} />)

      await user.press(screen.getByRole('button', { name: 'Blue-Eyes White Dragon — SDK-001' }))

      expect(
        await screen.findByText('Keeping Blue-Eyes White Dragon from SDK-001'),
      ).toBeOnTheScreen()
    })
  })

  describe('a scan that named no card', () => {
    it('offers no card to keep, because there is none', async () => {
      await render(<Harness resolved={[nothing('SMUDGE')]} />)

      expect(screen.getByText('No card matched this code.')).toBeOnTheScreen()
      expect(screen.queryByRole('button', { name: /^Keep / })).not.toBeOnTheScreen()
    })

    it('lets the card be left out of the binder', async () => {
      const user = userEvent.setup()
      await render(<Harness resolved={[nothing('SMUDGE')]} />)

      await user.press(screen.getByRole('button', { name: 'Leave this card out' }))

      expect(await screen.findByText('Left out of the binder')).toBeOnTheScreen()
    })
  })

  it('surfaces a scan the resolver refused rather than dropping it', async () => {
    await render(<Harness rejected={[refused('!!!')]} />)

    expect(screen.getByText('Scanned code !!!')).toBeOnTheScreen()
    expect(
      screen.getByText('The server would not accept this read, so no card was looked up.'),
    ).toBeOnTheScreen()
  })

  it('takes a decision back, so a wrong tap is not permanent', async () => {
    const user = userEvent.setup()
    await render(<Harness resolved={[byName('LOB-EN002')]} />)

    await user.press(screen.getByRole('button', { name: 'Leave this card out' }))
    await user.press(
      await screen.findByRole('button', { name: 'Change what happens to LOB-EN002' }),
    )

    expect(
      await screen.findByRole('button', { name: 'Keep Blue-Eyes White Dragon, set unknown' }),
    ).toBeOnTheScreen()
  })

  describe('correcting a card by name', () => {
    it('files the card the user found, with its set still unknown', async () => {
      mockFetch.mockImplementation(() =>
        Promise.resolve(
          new Response(JSON.stringify({ cards: [darkMagician] } satisfies SearchCardsBody), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          }),
        ),
      )
      const user = userEvent.setup()
      await render(<Harness resolved={[nothing('SMUDGE')]} />)

      await user.press(screen.getByRole('button', { name: 'Search by name' }))
      await user.type(await screen.findByLabelText('Card name'), 'Dark Mag')
      await user.press(screen.getByRole('button', { name: 'Search' }))

      await user.press(await screen.findByRole('button', { name: 'Dark Magician — set unknown' }))

      expect(await screen.findByText('Keeping Dark Magician, set unknown')).toBeOnTheScreen()
    })

    it('says the search is unreachable instead of showing an empty shelf', async () => {
      mockFetch.mockImplementation(() => Promise.reject(new Error('network down')))
      const user = userEvent.setup()
      await render(<Harness resolved={[nothing('SMUDGE')]} />)

      await user.press(screen.getByRole('button', { name: 'Search by name' }))
      await user.type(await screen.findByLabelText('Card name'), 'Dark Mag')
      await user.press(screen.getByRole('button', { name: 'Search' }))

      expect(
        await screen.findByText('The card search could not be reached. Try again.'),
      ).toBeOnTheScreen()
    })

    it('leaves the flagged card untouched when the search is abandoned', async () => {
      const user = userEvent.setup()
      await render(<Harness resolved={[nothing('SMUDGE')]} />)

      await user.press(screen.getByRole('button', { name: 'Search by name' }))
      await user.press(await screen.findByRole('button', { name: 'Back to the flagged cards' }))

      expect(await screen.findByText('No card matched this code.')).toBeOnTheScreen()
      expect(mockFetch).not.toHaveBeenCalled()
    })
  })

  it('closes when the user is done', async () => {
    const onClose = jest.fn()
    const user = userEvent.setup()
    await render(<Harness resolved={[byName('LOB-EN002')]} onClose={onClose} />)

    await user.press(screen.getByRole('button', { name: 'Done' }))

    expect(onClose).toHaveBeenCalled()
  })
})
