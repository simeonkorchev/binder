import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { FlatList, Modal, Pressable, StyleSheet, Text, View } from 'react-native'
import { SafeAreaView } from 'react-native-safe-area-context'

import type { Binder } from '@/features/binder/types'
import { useTheme } from '@/theme/useTheme'

import type { SweepCommit } from '../api/useCommitSweep'
import type { ScanReview } from '../useScanReview'

import { CardSearchPicker } from './CardSearchPicker'
import { FileSweep } from './FileSweep'
import { FlaggedCardRow } from './FlaggedCardRow'

/**
 * Where in the sheet the user is. One at a time, and a union rather than three
 * booleans: searching for a card *while* choosing a binder is not a state this
 * sheet has.
 */
type SheetView =
  | { kind: 'rows' }
  /** The name search for one flagged code. */
  | { kind: 'search'; code: string }
  /** Choosing the binder the whole sweep goes into. */
  | { kind: 'file' }

interface ReviewSheetProps {
  visible: boolean
  review: ScanReview
  commit: SweepCommit
  /** The binder the sweep was filed in. The screen closes the sheet and opens it. */
  onFiled: (binder: Binder) => void
  onClose: () => void
}

/**
 * Everything the sweep could not settle, in one place, while the cards are
 * still in front of the user.
 *
 * It is a modal over the scan tab rather than a pushed route because the whole
 * session — the queue's answers and the user's decisions — lives in the hooks
 * the scan screen holds. A route would have to carry that state through
 * navigation params, which would make a live sweep a snapshot taken when the
 * route opened.
 *
 * Which of its three views is up is the one thing held here: it is where the
 * sheet is, not what the session knows, and only one search runs at a time so
 * only one `useCardSearch` is ever mounted. Leaving the sheet returns it to the
 * rows, so it never reopens halfway through something the user has finished.
 */
export const ReviewSheet = ({
  visible,
  review,
  commit,
  onFiled,
  onClose,
}: ReviewSheetProps): React.JSX.Element => {
  const { colors } = useTheme()
  const [view, setView] = useState<SheetView>({ kind: 'rows' })

  const leave = (): void => {
    setView({ kind: 'rows' })
    onClose()
  }

  return (
    <Modal visible={visible} animationType="slide" onRequestClose={leave}>
      <SafeAreaView style={[styles.sheet, { backgroundColor: colors.background }]}>
        {view.kind === 'rows' ? (
          <ReviewList
            review={review}
            cardCount={commit.cardCount}
            onClose={leave}
            onSearch={(code) => setView({ kind: 'search', code })}
            onFile={() => setView({ kind: 'file' })}
          />
        ) : null}

        {view.kind === 'search' ? (
          <CardSearchPicker
            code={view.code}
            onPick={(card) => {
              review.decide(view.code, { kind: 'kept', card, printing: null })
              setView({ kind: 'rows' })
            }}
            onCancel={() => setView({ kind: 'rows' })}
          />
        ) : null}

        {view.kind === 'file' ? (
          <FileSweep
            commit={commit}
            undecidedCount={review.undecidedCount}
            onFiled={(binder) => {
              setView({ kind: 'rows' })
              onFiled(binder)
            }}
            onCancel={() => setView({ kind: 'rows' })}
          />
        ) : null}
      </SafeAreaView>
    </Modal>
  )
}

interface ReviewListProps {
  review: ScanReview
  /** How many cards the commit would file, for the footer that offers it. */
  cardCount: number
  onClose: () => void
  onSearch: (code: string) => void
  onFile: () => void
}

/**
 * The flagged rows, or the line that says there are none — and the way on to a
 * binder either way.
 *
 * A sweep where every card resolved is the good outcome, and the sheet has to
 * say so: an empty list behind a button the user just pressed reads as a screen
 * that failed to load. The footer is offered in both cases, because a clean
 * sweep is the one that most needs filing.
 */
const ReviewList = ({
  review,
  cardCount,
  onClose,
  onSearch,
  onFile,
}: ReviewListProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()

  return (
    <View style={styles.list}>
      <View style={styles.header}>
        <View style={styles.headings}>
          <Text style={[styles.title, { color: colors.textPrimary }]}>{t('scan.review.title')}</Text>
          <Text style={[styles.subtitle, { color: colors.textSecondary }]}>
            {t('scan.review.subtitle')}
          </Text>
        </View>
        <Pressable
          onPress={onClose}
          accessibilityRole="button"
          accessibilityLabel={t('scan.review.done')}
          style={[styles.done, { backgroundColor: colors.accent }]}
        >
          <Text style={[styles.doneLabel, { color: colors.onAccent }]}>{t('scan.review.done')}</Text>
        </Pressable>
      </View>

      {review.rows.length === 0 ? (
        <Text style={[styles.empty, { color: colors.textSecondary }]}>{t('scan.review.empty')}</Text>
      ) : (
        <FlatList
          data={review.rows}
          keyExtractor={(row) => row.code}
          contentContainerStyle={styles.rows}
          renderItem={({ item }) => (
            <FlaggedCardRow
              row={item}
              onDecide={(decision) => review.decide(item.code, decision)}
              onReopen={() => review.reopen(item.code)}
              onSearch={() => onSearch(item.code)}
            />
          )}
        />
      )}

      <View style={[styles.footer, { borderTopColor: colors.border }]}>
        <Pressable
          onPress={onFile}
          accessibilityRole="button"
          accessibilityLabel={t('scan.commit.openAccessible', { cards: cardCount })}
          style={[styles.file, { backgroundColor: colors.accent }]}
        >
          <Text style={[styles.fileLabel, { color: colors.onAccent }]}>
            {t('scan.commit.open')}
          </Text>
        </Pressable>
      </View>
    </View>
  )
}

const styles = StyleSheet.create({
  done: { borderRadius: 8, paddingHorizontal: 16, paddingVertical: 10 },
  doneLabel: { fontSize: 15, fontWeight: '600' },
  empty: { fontSize: 15, lineHeight: 22, paddingHorizontal: 16, paddingVertical: 24 },
  file: { alignItems: 'center', borderRadius: 8, paddingHorizontal: 16, paddingVertical: 12 },
  fileLabel: { fontSize: 15, fontWeight: '700' },
  footer: { borderTopWidth: StyleSheet.hairlineWidth, padding: 16 },
  header: { flexDirection: 'row', gap: 12, paddingHorizontal: 16, paddingTop: 12 },
  headings: { flex: 1 },
  list: { flex: 1 },
  rows: { gap: 12, padding: 16 },
  sheet: { flex: 1 },
  subtitle: { fontSize: 14, lineHeight: 20, marginTop: 4 },
  title: { fontSize: 20, fontWeight: '700' },
})
