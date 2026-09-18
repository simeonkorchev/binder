import { useTranslation } from 'react-i18next'
import { Pressable, StyleSheet, Text, View } from 'react-native'

import { BinderPicker } from '@/features/binder/components/BinderPicker'
import type { Binder } from '@/features/binder/types'
import { useTheme } from '@/theme/useTheme'

import type { SweepCommit } from '../api/useCommitSweep'

/**
 * What takes the picker's place once a binder has been pressed: while the batch
 * is in flight, because a second binder pressed now would file the sweep twice,
 * and after it lands, because the cards are no longer here to send.
 */
const inFlightKey = (
  status: SweepCommit['status'],
): 'scan.commit.sending' | 'scan.commit.filed' | null => {
  switch (status) {
    case 'sending':
      return 'scan.commit.sending'
    case 'filed':
      return 'scan.commit.filed'
    case 'idle':
    case 'failed':
      return null
  }
}

interface FileSweepProps {
  commit: SweepCommit
  /** How many flagged cards are still unanswered, and so will be left out. */
  undecidedCount: number
  /** The binder that now holds the sweep. The screen takes the user there. */
  onFiled: (binder: Binder) => void
  /** Back to the flagged cards, with every decision still as it was. */
  onCancel: () => void
}

/**
 * Where the sweep goes: one binder, one request, all of it or none of it.
 *
 * The list is the binder feature's own picker, so a collector with no binder can
 * make one here rather than leaving a reviewed sweep to go and do it on another
 * tab.
 *
 * A failed commit says so and leaves everything exactly where it was — the
 * decisions, the captures, and this view. Pressing a binder again sends the same
 * batch, which is the whole reason nothing is thrown away when it fails: losing a
 * reviewed sweep to a dropped connection would mean scanning the page again.
 */
export const FileSweep = ({
  commit,
  undecidedCount,
  onFiled,
  onCancel,
}: FileSweepProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const inFlight = inFlightKey(commit.status)

  const file = async (binder: Binder): Promise<void> => {
    const filed = await commit.send(binder.id)
    // Only a binder that really has the cards is somewhere to send the user.
    if (filed) onFiled(binder)
  }

  return (
    <View style={styles.view}>
      <View style={styles.header}>
        <View style={styles.headings}>
          <Text style={[styles.title, { color: colors.textPrimary }]}>
            {t('scan.commit.title')}
          </Text>
          <Text style={[styles.subtitle, { color: colors.textSecondary }]}>
            {t('scan.commit.subtitle', { cards: commit.cardCount })}
          </Text>
        </View>
        <Pressable
          onPress={onCancel}
          accessibilityRole="button"
          accessibilityLabel={t('scan.commit.back')}
          style={[styles.back, { borderColor: colors.border }]}
        >
          <Text style={[styles.backLabel, { color: colors.textPrimary }]}>
            {t('scan.commit.back')}
          </Text>
        </Pressable>
      </View>

      {commit.cardCount === 0 ? (
        <Text style={[styles.note, { color: colors.textSecondary }]}>
          {t('scan.commit.nothing')}
        </Text>
      ) : null}

      {undecidedCount > 0 ? (
        <Text style={[styles.note, { color: colors.textSecondary }]}>
          {t('scan.commit.leftOut', { undecided: undecidedCount })}
        </Text>
      ) : null}

      {commit.status === 'failed' ? (
        <Text style={[styles.note, { color: colors.error }]}>{t('scan.commit.failed')}</Text>
      ) : null}

      {inFlight === null ? (
        <BinderPicker
          rowLabel={(binder) => t('scan.commit.intoLabel', { name: binder.name })}
          onPick={(binder) => void file(binder)}
        />
      ) : (
        <Text style={[styles.note, { color: colors.textSecondary }]}>{t(inFlight)}</Text>
      )}
    </View>
  )
}

const styles = StyleSheet.create({
  back: { borderRadius: 8, borderWidth: 1, paddingHorizontal: 14, paddingVertical: 10 },
  backLabel: { fontSize: 14, fontWeight: '600' },
  header: { flexDirection: 'row', gap: 12, paddingHorizontal: 16, paddingTop: 12 },
  headings: { flex: 1 },
  note: { fontSize: 14, lineHeight: 20, paddingHorizontal: 16, paddingTop: 12 },
  subtitle: { fontSize: 14, lineHeight: 20, marginTop: 4 },
  title: { fontSize: 20, fontWeight: '700' },
  view: { flex: 1 },
})
