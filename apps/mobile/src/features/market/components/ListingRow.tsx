import { useTranslation } from 'react-i18next'
import { Pressable, StyleSheet, Text, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

import type { ListedCard } from '../types'

interface ListingRowProps {
  listing: ListedCard
  /** Opens the seller's contact details. The feed carries none of them itself. */
  onContact: () => void
}

/**
 * One card for sale.
 *
 * The set is shown as words rather than left blank when the seller's scan never
 * determined it: a row with an empty space where a set belongs reads as a bug,
 * and "set unknown" is a fact a buyer wants before they ask about the card.
 */
export const ListingRow = ({ listing, onContact }: ListingRowProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()

  return (
    <Pressable
      onPress={onContact}
      accessibilityRole="button"
      accessibilityLabel={t('market.browse.openSeller', { name: listing.cardName })}
      style={[styles.row, { backgroundColor: colors.surface, borderColor: colors.border }]}
    >
      <Text style={[styles.name, { color: colors.textPrimary }]} numberOfLines={2}>
        {listing.cardName}
      </Text>
      <View style={styles.meta}>
        <Text style={[styles.set, { color: colors.textSecondary }]}>
          {listing.setCode ?? t('market.browse.unknownSet')}
        </Text>
      </View>
    </Pressable>
  )
}

const styles = StyleSheet.create({
  meta: { flexDirection: 'row', marginTop: 4 },
  name: { fontSize: 16, fontWeight: '600' },
  row: { borderRadius: 10, borderWidth: 1, marginBottom: 10, padding: 16 },
  set: { fontSize: 13 },
})
