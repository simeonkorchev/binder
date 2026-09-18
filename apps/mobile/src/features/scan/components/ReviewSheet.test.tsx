import { render, screen, userEvent } from '@testing-library/react-native'

import '@/i18n/i18n'

import type { Binder, SlotCardBody } from '@/features/binder/types'
import type { SearchCardsBody } from '@/features/card/types'

import { useCommitSweep } from '../api/useCommitSweep'
import type { RejectedScan, ResolvedScan, ScannedCard } from '../types'
import { useScanReview } from '../useScanReview'

import { ReviewSheet } from './ReviewSheet'

const apiBaseUrl = 'https://api.binder.test'
const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

/** Where a filed sweep sends the user — the screen's job, recorded here. */
const onFiled = jest.fn<void, [Binder]>()
/** What the scan session does with a sweep the binder now has: lets it go. */
const letGo = jest.fn<void, []>()

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

/** A code rung that matched one printing outright — the review never asks about it. */
const settled = (code: string): ResolvedScan => ({
  code,
  resolution: 'exact',
  outcome: 'resolved',
  card: blueEyes,
  printing: { id: 'lob-001', cardId: 'blue-eyes', setCode: 'LOB-001', rarity: 'Ultra Rare' },
  candidates: [],
})

const theBinder: Binder = {
  id: 'binder-1',
  name: 'Duplicates',
  createdAt: '2026-09-18T10:00:00Z',
  updatedAt: '2026-09-18T10:00:00Z',
}

const searchAnswer = (): Response =>
  new Response(JSON.stringify({ cards: [darkMagician] } satisfies SearchCardsBody), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })

/**
 * The three requests this sheet can make: the name search, the collector's
 * binders, and the one batch that files the sweep.
 */
const serverWhere =
  (batch: () => Promise<Response>) =>
  (input: RequestInfo | URL): Promise<Response> => {
    const url = String(input)
    if (url.endsWith('/slots/batch')) return batch()
    if (url.endsWith('/binders')) {
      return Promise.resolve(new Response(JSON.stringify({ binders: [theBinder] }), { status: 201 }))
    }
    return Promise.resolve(searchAnswer())
  }

const filed = (): Promise<Response> =>
  Promise.resolve(new Response(JSON.stringify({ slots: [] }), { status: 201 }))

const batches = (): RequestInit[] =>
  mockFetch.mock.calls
    .filter(([input]) => String(input).endsWith('/slots/batch'))
    .map(([, init]) => init ?? {})

/** The cards one batch carried, by the order the batches were sent. */
const batched = (attempt: number): SlotCardBody[] => {
  const body = batches()[attempt]?.body
  if (typeof body !== 'string') throw new Error(`no batch number ${attempt} was sent`)

  const parsed: { cards: SlotCardBody[] } = JSON.parse(body)
  return parsed.cards
}

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
  // `letGo` is what the scan session does with a filed sweep — this harness has
  // no session, and `useScanSession`'s own suite pins the clearing.
  const commit = useCommitSweep({ resolved, rejected, reviewed: review.rows }, letGo)
  return (
    <ReviewSheet visible review={review} commit={commit} onFiled={onFiled} onClose={onClose} />
  )
}

describe('ReviewSheet', () => {
  beforeEach(() => {
    mockFetch.mockReset()
    mockFetch.mockImplementation(serverWhere(filed))
    onFiled.mockReset()
    letGo.mockReset()
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
  describe('filing the sweep', () => {
    const THE_BINDER = 'File this sweep in Duplicates'

    /**
     * The footer that leads to a binder. Its accessible name carries the count,
     * so asking for it by that name is also an assertion about how many cards the
     * sheet is about to file.
     */
    const fileButton = (cards: number): string =>
      `File this sweep in a binder. Cards to file: ${cards}`

    /** The review, then the binder: the loop from a swept card to a filed one. */
    const fileInTheBinder = async (
      user: ReturnType<typeof userEvent.setup>,
      cards: number,
    ): Promise<void> => {
      await user.press(await screen.findByRole('button', { name: fileButton(cards) }))
      await user.press(await screen.findByRole('button', { name: THE_BINDER }))
    }

    it('sends a printing the user picked as manual, never as the rung it came in on', async () => {
      const user = userEvent.setup()
      await render(<Harness resolved={[ambiguous('001')]} />)

      await user.press(screen.getByRole('button', { name: 'Blue-Eyes White Dragon — SDK-001' }))
      await fileInTheBinder(user, 1)

      expect(batched(0)).toEqual([
        { cardId: 'blue-eyes', cardPrintingId: 'sdk-001', setResolution: 'manual' },
      ] satisfies SlotCardBody[])
    })

    it('sends a card the ladder settled on the rung the ladder answered with', async () => {
      const user = userEvent.setup()
      await render(<Harness resolved={[settled('LOB-001')]} />)

      await fileInTheBinder(user, 1)

      expect(batched(0)).toEqual([
        { cardId: 'blue-eyes', cardPrintingId: 'lob-001', setResolution: 'exact' },
      ] satisfies SlotCardBody[])
    })

    it('sends a card kept with its set left open as by_name, with no printing', async () => {
      const user = userEvent.setup()
      await render(<Harness resolved={[byName('LOB-EN002')]} />)

      await user.press(
        screen.getByRole('button', { name: 'Keep Blue-Eyes White Dragon, set unknown' }),
      )
      await fileInTheBinder(user, 1)

      expect(batched(0)).toEqual([
        { cardId: 'blue-eyes', cardPrintingId: null, setResolution: 'by_name' },
      ] satisfies SlotCardBody[])
    })

    it('sends the whole sweep as one request, not one per card', async () => {
      const user = userEvent.setup()
      await render(<Harness resolved={[settled('LOB-001'), settled('LOB-001'), byName('X')]} />)

      await user.press(
        screen.getByRole('button', { name: 'Keep Blue-Eyes White Dragon, set unknown' }),
      )
      await fileInTheBinder(user, 3)

      expect(batches()).toHaveLength(1)
      expect(batched(0)).toHaveLength(3)
    })

    it('takes the user to the binder that was filled, and lets the sweep go', async () => {
      const user = userEvent.setup()
      await render(<Harness resolved={[settled('LOB-001')]} />)

      await fileInTheBinder(user, 1)

      expect(onFiled).toHaveBeenCalledWith(theBinder)
      expect(letGo).toHaveBeenCalledTimes(1)
    })

    // Without this the scanner files once and then dead-ends: the second sweep's
    // file view would still be showing the first one's outcome.
    it('offers a binder again once a sweep has been filed', async () => {
      const user = userEvent.setup()
      await render(<Harness resolved={[settled('LOB-001')]} />)

      await fileInTheBinder(user, 1)
      await user.press(await screen.findByRole('button', { name: fileButton(1) }))

      expect(await screen.findByRole('button', { name: THE_BINDER })).toBeOnTheScreen()
    })

    it('files a sweep in which every card was left out, as an empty batch', async () => {
      const user = userEvent.setup()
      await render(<Harness resolved={[nothing('SMUDGE')]} />)

      await user.press(screen.getByRole('button', { name: 'Leave this card out' }))
      await user.press(await screen.findByRole('button', { name: fileButton(0) }))
      expect(
        screen.getByText('Every card in this sweep was left out, so nothing will be filed.'),
      ).toBeOnTheScreen()

      await user.press(await screen.findByRole('button', { name: THE_BINDER }))

      expect(batched(0)).toEqual([])
      expect(onFiled).toHaveBeenCalledWith(theBinder)
    })

    it('says how many unchecked cards the commit would leave out', async () => {
      const user = userEvent.setup()
      await render(<Harness resolved={[byName('LOB-EN002'), nothing('SMUDGE')]} />)

      await user.press(await screen.findByRole('button', { name: fileButton(0) }))

      expect(
        screen.getByText('Cards still unchecked, and so left out: 2'),
      ).toBeOnTheScreen()
    })

    describe('a commit that did not land', () => {
      const refusedBatch = (): Promise<Response> =>
        Promise.resolve(new Response(JSON.stringify({}), { status: 500 }))

      it('says nothing was added, and does not send the user anywhere', async () => {
        mockFetch.mockImplementation(serverWhere(refusedBatch))
        const user = userEvent.setup()
        await render(<Harness resolved={[settled('LOB-001')]} />)

        await fileInTheBinder(user, 1)

        expect(
          await screen.findByText(
            'The sweep could not be filed, so nothing was added to the binder. Every decision is still here — pick a binder to send it again.',
          ),
        ).toBeOnTheScreen()
        expect(onFiled).not.toHaveBeenCalled()
        expect(letGo).not.toHaveBeenCalled()
      })

      // The harm the server's own transaction exists to prevent, reintroduced on the
      // client: a reviewed sweep lost to a bad signal is a page to scan again.
      it('keeps every decision, so the same sweep is sent again without a re-scan', async () => {
        mockFetch.mockImplementation(serverWhere(refusedBatch))
        const user = userEvent.setup()
        await render(<Harness resolved={[ambiguous('001')]} />)

        await user.press(screen.getByRole('button', { name: 'Blue-Eyes White Dragon — SDK-001' }))
        await fileInTheBinder(user, 1)

        await user.press(await screen.findByRole('button', { name: 'Back to the cards' }))
        expect(
          await screen.findByText('Keeping Blue-Eyes White Dragon from SDK-001'),
        ).toBeOnTheScreen()

        mockFetch.mockImplementation(serverWhere(filed))
        await fileInTheBinder(user, 1)

        expect(batches()).toHaveLength(2)
        expect(batched(1)).toEqual(batched(0))
        expect(onFiled).toHaveBeenCalledWith(theBinder)
      })
    })
  })
})
