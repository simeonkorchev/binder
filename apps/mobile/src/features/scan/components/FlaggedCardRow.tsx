import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { Pressable, StyleSheet, Text, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

import type { FlagReason } from '../lib/flaggedScans'
import type { ScanCandidate } from '../types'
import type { ReviewDecision, ReviewRow } from '../useScanReview'

/**
 * Why this card is in the sheet, as a sentence.
 *
 * Every reason is words. Marking a flagged card with a colour would say nothing
 * to a colour-blind user and nothing at all to a screen reader — and unlike the
 * capture strip, this screen exists *because* the difference between the states
 * is what the user has to act on.
 */
const reasonKeys = {
  refused: 'scan.review.reason.refused',
  unresolved: 'scan.review.reason.unresolved',
  ambiguous: 'scan.review.reason.ambiguous',
  no_set: 'scan.review.reason.no_set',
} as const satisfies Record<FlagReason, string>

interface FlaggedCardRowProps {
  row: ReviewRow
  onDecide: (decision: ReviewDecision) => void
  onReopen: () => void
  /** Opens the name search for this row — the sheet owns which row is being searched. */
  onSearch: () => void
}

/**
 * One card the sweep could not settle, and every way out of it.
 *
 * A row always ends in something the user can press. That is the whole point of
 * the sheet: a flagged card with no action is a card that silently does not
 * make it into the binder, which is the outcome US3 exists to prevent.
 */
export const FlaggedCardRow = ({
  row,
  onDecide,
  onReopen,
  onSearch,
}: FlaggedCardRowProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()

  return (
    <View style={[styles.row, { backgroundColor: colors.surface, borderColor: colors.border }]}>
      <Text style={[styles.code, { color: colors.textPrimary }]}>
        {t('scan.review.code', { code: row.code })}
      </Text>
      <Text style={[styles.reason, { color: colors.textSecondary }]}>{t(reasonKeys[row.reason])}</Text>
      {row.copies > 1 ? (
        <Text style={[styles.copies, { color: colors.textSecondary }]}>
          {t('scan.review.copies', { copies: row.copies })}
        </Text>
      ) : null}

      {row.decision === null ? (
        <RowChoices row={row} onDecide={onDecide} onSearch={onSearch} />
      ) : (
        <RowDecision row={row} decision={row.decision} onReopen={onReopen} />
      )}
    </View>
  )
}

/** The label a candidate is offered under: its name, and its set when it has one. */
const candidateLabel = (candidate: ScanCandidate, t: TFunction): string => {
  if (candidate.printing === null) {
    return t('scan.review.chooseNoSet', { name: candidate.card.name })
  }
  return t('scan.review.choose', { name: candidate.card.name, set: candidate.printing.setCode })
}

/** What a settled row reports back: the card and its set, or that it was left out. */
const settledLabel = (decision: ReviewDecision, t: TFunction): string => {
  if (decision.kind === 'discarded') return t('scan.review.discarded')
  if (decision.printing === null) return t('scan.review.kept', { name: decision.card.name })
  return t('scan.review.keptWithSet', {
    name: decision.card.name,
    set: decision.printing.setCode,
  })
}

interface RowChoicesProps {
  row: ReviewRow
  onDecide: (decision: ReviewDecision) => void
  onSearch: () => void
}

/**
 * What an undecided row offers: the candidates the ladder could not choose
 * between, keeping the card it did name with its set still open, a name search
 * for when it named nothing usable, and leaving the card out.
 *
 * "Keep" appears whenever a card was named — including for an ambiguous scan,
 * where "I cannot tell which printing this is" is a real answer and a slot with
 * no printing is exactly how the binder records it.
 */
const RowChoices = ({ row, onDecide, onSearch }: RowChoicesProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()
  // Bound out of the props so the press handler below keeps the narrowing the
  // guard proves: a closure over `row.card` would be back to `ScannedCard | null`.
  const named = row.card

  return (
    <View style={styles.choices}>
      {row.candidates.map((candidate) => {
        const label = candidateLabel(candidate, t)
        return (
          <Pressable
            key={`${candidate.card.id}:${candidate.printing?.id ?? 'no-set'}`}
            onPress={() =>
              onDecide({ kind: 'kept', card: candidate.card, printing: candidate.printing })
            }
            accessibilityRole="button"
            accessibilityLabel={label}
            style={[styles.choice, { borderColor: colors.accent }]}
          >
            <Text style={[styles.choiceLabel, { color: colors.accent }]}>{label}</Text>
          </Pressable>
        )
      })}

      {named === null ? null : (
        <Pressable
          onPress={() => onDecide({ kind: 'kept', card: named, printing: null })}
          accessibilityRole="button"
          accessibilityLabel={t('scan.review.keep', { name: named.name })}
          style={[styles.choice, { borderColor: colors.accent }]}
        >
          <Text style={[styles.choiceLabel, { color: colors.accent }]}>
            {t('scan.review.keep', { name: named.name })}
          </Text>
        </Pressable>
      )}

      <Pressable
        onPress={onSearch}
        accessibilityRole="button"
        accessibilityLabel={t('scan.review.search')}
        style={[styles.choice, { borderColor: colors.border }]}
      >
        <Text style={[styles.choiceLabel, { color: colors.textPrimary }]}>
          {t('scan.review.search')}
        </Text>
      </Pressable>

      <Pressable
        onPress={() => onDecide({ kind: 'discarded' })}
        accessibilityRole="button"
        accessibilityLabel={t('scan.review.discard')}
        style={[styles.choice, { borderColor: colors.border }]}
      >
        <Text style={[styles.choiceLabel, { color: colors.textSecondary }]}>
          {t('scan.review.discard')}
        </Text>
      </Pressable>
    </View>
  )
}

interface RowDecisionProps {
  row: ReviewRow
  decision: ReviewDecision
  onReopen: () => void
}

/** What a settled row says, and the way back out of it. */
const RowDecision = ({ row, decision, onReopen }: RowDecisionProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()

  return (
    <View style={styles.settled}>
      <Text style={[styles.settledLabel, { color: colors.textPrimary }]}>
        {settledLabel(decision, t)}
      </Text>
      <Pressable
        onPress={onReopen}
        accessibilityRole="button"
        accessibilityLabel={t('scan.review.changeAccessible', { code: row.code })}
        style={[styles.choice, { borderColor: colors.border }]}
      >
        <Text style={[styles.choiceLabel, { color: colors.textPrimary }]}>
          {t('scan.review.change')}
        </Text>
      </Pressable>
    </View>
  )
}

const styles = StyleSheet.create({
  choice: { borderRadius: 8, borderWidth: 1, paddingHorizontal: 12, paddingVertical: 10 },
  choiceLabel: { fontSize: 14, fontWeight: '600' },
  choices: { gap: 8, marginTop: 12 },
  code: { fontSize: 15, fontWeight: '700' },
  copies: { fontSize: 13, marginTop: 2 },
  reason: { fontSize: 14, lineHeight: 20, marginTop: 4 },
  row: { borderRadius: 12, borderWidth: 1, padding: 14 },
  settled: { alignItems: 'flex-start', gap: 8, marginTop: 12 },
  settledLabel: { fontSize: 14, fontWeight: '600' },
})
