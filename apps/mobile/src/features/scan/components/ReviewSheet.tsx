import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { FlatList, Modal, Pressable, StyleSheet, Text, View } from 'react-native'
import { SafeAreaView } from 'react-native-safe-area-context'

import { useTheme } from '@/theme/useTheme'

import type { ScanReview } from '../useScanReview'

import { CardSearchPicker } from './CardSearchPicker'
import { FlaggedCardRow } from './FlaggedCardRow'

interface ReviewSheetProps {
  visible: boolean
  review: ScanReview
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
 * Which row is being searched is the one thing held here: it is where the sheet
 * is, not what the session knows, and only one search runs at a time so only
 * one `useCardSearch` is ever mounted.
 */
export const ReviewSheet = ({ visible, review, onClose }: ReviewSheetProps): React.JSX.Element => {
  const { colors } = useTheme()
  const [searchingFor, setSearchingFor] = useState<string | null>(null)

  return (
    <Modal visible={visible} animationType="slide" onRequestClose={onClose}>
      <SafeAreaView style={[styles.sheet, { backgroundColor: colors.background }]}>
        {searchingFor === null ? (
          <ReviewList review={review} onClose={onClose} onSearch={setSearchingFor} />
        ) : (
          <CardSearchPicker
            code={searchingFor}
            onPick={(card) => {
              review.decide(searchingFor, { kind: 'kept', card, printing: null })
              setSearchingFor(null)
            }}
            onCancel={() => setSearchingFor(null)}
          />
        )}
      </SafeAreaView>
    </Modal>
  )
}

interface ReviewListProps {
  review: ScanReview
  onClose: () => void
  onSearch: (code: string) => void
}

/**
 * The flagged rows, or the line that says there are none.
 *
 * A sweep where every card resolved is the good outcome, and the sheet has to
 * say so: an empty list behind a button the user just pressed reads as a screen
 * that failed to load.
 */
const ReviewList = ({ review, onClose, onSearch }: ReviewListProps): React.JSX.Element => {
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
    </View>
  )
}

const styles = StyleSheet.create({
  done: { borderRadius: 8, paddingHorizontal: 16, paddingVertical: 10 },
  doneLabel: { fontSize: 15, fontWeight: '600' },
  empty: { fontSize: 15, lineHeight: 22, paddingHorizontal: 16, paddingVertical: 24 },
  header: { flexDirection: 'row', gap: 12, paddingHorizontal: 16, paddingTop: 12 },
  headings: { flex: 1 },
  list: { flex: 1 },
  rows: { gap: 12, padding: 16 },
  sheet: { flex: 1 },
  subtitle: { fontSize: 14, lineHeight: 20, marginTop: 4 },
  title: { fontSize: 20, fontWeight: '700' },
})
