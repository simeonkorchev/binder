import { useTranslation } from 'react-i18next'
import { Pressable, StyleSheet, Text, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

interface PageIndicatorProps {
  /** Zero-based, as the API numbers pages; shown to the user from 1. */
  page: number
  /** How many pages the pager offers, never zero. */
  pageCount: number
  onTurnPage: (step: -1 | 1) => void
}

/**
 * Which page of the binder is open, and the way to the ones either side that
 * does not need a swipe.
 */
export const PageIndicator = ({
  page,
  pageCount,
  onTurnPage,
}: PageIndicatorProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()

  return (
    <View style={styles.bar}>
      <TurnPageButton
        label={t('binder.page.previous')}
        glyph="‹"
        isEnabled={page > 0}
        onPress={() => onTurnPage(-1)}
      />
      <Text style={[styles.label, { color: colors.textPrimary }]}>
        {t('binder.page.indicator', { page: page + 1, pageCount })}
      </Text>
      <TurnPageButton
        label={t('binder.page.next')}
        glyph="›"
        isEnabled={page < pageCount - 1}
        onPress={() => onTurnPage(1)}
      />
    </View>
  )
}

interface TurnPageButtonProps {
  label: string
  glyph: string
  isEnabled: boolean
  onPress: () => void
}

/** A glyph carries the direction; the translated label carries the meaning. */
const TurnPageButton = ({
  label,
  glyph,
  isEnabled,
  onPress,
}: TurnPageButtonProps): React.JSX.Element => {
  const { colors } = useTheme()

  return (
    <Pressable
      onPress={onPress}
      disabled={!isEnabled}
      accessibilityRole="button"
      accessibilityLabel={label}
      accessibilityState={{ disabled: !isEnabled }}
      style={[styles.turn, { borderColor: colors.border }, isEnabled ? null : styles.unavailable]}
    >
      <Text style={[styles.glyph, { color: colors.textPrimary }]}>{glyph}</Text>
    </Pressable>
  )
}

const styles = StyleSheet.create({
  bar: { alignItems: 'center', flexDirection: 'row', gap: 16, justifyContent: 'center', paddingVertical: 12 },
  glyph: { fontSize: 20, fontWeight: '700', lineHeight: 24 },
  label: { fontSize: 15, fontWeight: '600', minWidth: 140, textAlign: 'center' },
  turn: { alignItems: 'center', borderRadius: 8, borderWidth: 1, justifyContent: 'center', minHeight: 44, minWidth: 44 },
  unavailable: { opacity: 0.4 },
})
