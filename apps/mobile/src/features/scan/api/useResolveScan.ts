import { useRef, useState } from 'react'

import { apiUrl } from '@/lib/apiUrl'

import type { RejectedScan, ResolvedScan, ScanMatchBody } from '../types'

import { toResolvedScan } from './toResolvedScan'

type ScanOutcome =
  | { kind: 'resolved'; scan: ResolvedScan }
  | { kind: 'rejected'; scan: RejectedScan }

/**
 * Asks the resolver about one code.
 *
 * Throws when the trip failed — no connection, or a server that broke — and
 * returns an outcome when the resolver answered about *this scan*. The queue
 * needs that distinction: a trip can be made again, an answer cannot be argued
 * with. A 4xx means this code will be refused every time it is sent, so
 * retrying it forever would wedge the queue behind one bad read.
 */
const requestScan = async (code: string): Promise<ScanOutcome> => {
  const response = await fetch(apiUrl('/scans/resolve'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code }),
  })

  if (response.status >= 500) {
    throw new Error(`the resolver answered ${response.status}`)
  }
  if (!response.ok) {
    return { kind: 'rejected', scan: { code, status: response.status } }
  }

  const body: ScanMatchBody = await response.json()
  return { kind: 'resolved', scan: toResolvedScan(code, body) }
}

export interface ResolveScanQueue {
  /**
   * Queues one accepted read. Returns immediately — the scan loop never waits
   * on the network, which is the whole premise of sweeping a page.
   */
  resolve: (code: string) => void
  /** The scans the ladder has answered, in the order they were swept. */
  resolved: ResolvedScan[]
  /** Scans the resolver refused. Kept, never dropped: the card was really there. */
  rejected: RejectedScan[]
  /** How many scans are still waiting for their answer. */
  queuedCount: number
  /**
   * True when the last attempt never reached the resolver. The queue is intact
   * and paused; it is not a count of lost cards.
   */
  isOffline: boolean
  /**
   * Drains what is queued again after a dropped connection. The next accepted
   * card restarts the queue on its own, so this is for the sweep that ended on
   * a bad signal — without it the last cards would wait for a scan that never
   * comes.
   */
  retryQueued: () => void
  /**
   * Forgets the whole sweep: its answers, its refusals and anything still
   * queued. Called once the cards are in a binder — a sweep left behind after it
   * was filed is a sweep that can be filed twice.
   */
  clear: () => void
}

/**
 * Resolves scanned codes in the background, one at a time, without ever making
 * the camera wait.
 *
 * The queue is a ref, not state: it is written from inside the drain loop
 * between awaits, and a re-render per step would be a re-render per card for a
 * value only the loop reads. What the screen draws — the answers, the
 * refusals, the backlog, the connection — is state.
 *
 * Nothing here runs from an effect, so React's double-mount in Strict Mode
 * cannot fire a duplicate request: work starts only when a scan is queued or a
 * retry is asked for.
 */
export const useResolveScan = (): ResolveScanQueue => {
  const queue = useRef<string[]>([])
  const isDraining = useRef(false)
  const [resolved, setResolved] = useState<ResolvedScan[]>([])
  const [rejected, setRejected] = useState<RejectedScan[]>([])
  const [queuedCount, setQueuedCount] = useState(0)
  const [isOffline, setIsOffline] = useState(false)

  const setQueue = (codes: string[]): void => {
    queue.current = codes
    setQueuedCount(codes.length)
  }

  const drain = async (): Promise<void> => {
    if (isDraining.current) return
    isDraining.current = true

    try {
      for (;;) {
        const code = queue.current[0]
        if (code === undefined) return

        let outcome: ScanOutcome
        try {
          outcome = await requestScan(code)
        } catch {
          // The trip failed, not the scan. Leave the code at the head of the
          // queue — the next accepted card, or `retryQueued`, carries it — and
          // say so, because a silent backlog reads as a scanner that stopped
          // finding cards.
          setIsOffline(true)
          return
        }

        setQueue(queue.current.slice(1))
        setIsOffline(false)
        if (outcome.kind === 'resolved') {
          setResolved((scans) => [...scans, outcome.scan])
        } else {
          setRejected((scans) => [...scans, outcome.scan])
        }
      }
    } finally {
      isDraining.current = false
    }
  }

  const startDraining = (): void => {
    void drain()
  }

  return {
    resolve: (code: string): void => {
      setQueue([...queue.current, code])
      startDraining()
    },
    retryQueued: startDraining,
    clear: (): void => {
      setQueue([])
      setResolved([])
      setRejected([])
      setIsOffline(false)
    },
    resolved,
    rejected,
    queuedCount,
    isOffline,
  }
}
