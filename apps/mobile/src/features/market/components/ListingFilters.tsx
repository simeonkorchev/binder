import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

import { wholeMarket, type ListingFilter } from '../lib/listingsPath'

interface ListingFiltersProps {
  /** Reads the feed again. Called with an empty filter when the buyer clears them. */
  onBrowse: (filter: ListingFilter) => void
}

/**
 * The two ways a buyer narrows the market: part of a card's name, and a set.
 *
 * What is typed is this component's own state and nothing else's — the feed is
 * only asked when the buyer says so. Searching on every keystroke would be a
 * request per letter on a phone connection, and a timer deciding when state
 * changes; a submit is one intent and one request.
 */
export const ListingFilters = ({ onBrowse }: ListingFiltersProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const [query, setQuery] = useState('')
  const [setCode, setSetCode] = useState('')

  const clear = (): void => {
    setQuery('')
    setSetCode('')
    onBrowse(wholeMarket)
  }

  return (
    <View style={styles.filters}>
      <TextInput
        value={query}
        onChangeText={setQuery}
        onSubmitEditing={() => onBrowse({ query, setCode })}
        placeholder={t('market.browse.searchPlaceholder')}
        placeholderTextColor={colors.textSecondary}
        accessibilityLabel={t('market.browse.searchLabel')}
        autoCorrect={false}
        returnKeyType="search"
        style={[
          styles.input,
          { backgroundColor: colors.surfaceMuted, borderColor: colors.border, color: colors.textPrimary },
        ]}
      />

      <TextInput
        value={setCode}
        onChangeText={setSetCode}
        onSubmitEditing={() => onBrowse({ query, setCode })}
        placeholder={t('market.browse.setPlaceholder')}
        placeholderTextColor={colors.textSecondary}
        accessibilityLabel={t('market.browse.setLabel')}
        autoCapitalize="characters"
        autoCorrect={false}
        returnKeyType="search"
        style={[
          styles.input,
          { backgroundColor: colors.surfaceMuted, borderColor: colors.border, color: colors.textPrimary },
        ]}
      />

      <View style={styles.actions}>
        <Pressable
          onPress={() => onBrowse({ query, setCode })}
          accessibilityRole="button"
          accessibilityLabel={t('market.browse.search')}
          style={[styles.search, { backgroundColor: colors.accent }]}
        >
          <Text style={[styles.searchLabel, { color: colors.onAccent }]}>
            {t('market.browse.search')}
          </Text>
        </Pressable>

        <Pressable
          onPress={clear}
          accessibilityRole="button"
          accessibilityLabel={t('market.browse.clear')}
          style={[styles.clear, { borderColor: colors.border }]}
        >
          <Text style={[styles.clearLabel, { color: colors.textPrimary }]}>
            {t('market.browse.clear')}
          </Text>
        </Pressable>
      </View>
    </View>
  )
}

const styles = StyleSheet.create({
  actions: { flexDirection: 'row', gap: 10, marginTop: 10 },
  clear: { borderRadius: 8, borderWidth: 1, paddingHorizontal: 14, paddingVertical: 10 },
  clearLabel: { fontSize: 14, fontWeight: '600' },
  filters: { paddingHorizontal: 16, paddingTop: 16 },
  input: { borderRadius: 8, borderWidth: 1, fontSize: 16, marginTop: 8, paddingHorizontal: 12, paddingVertical: 10 },
  search: { borderRadius: 8, paddingHorizontal: 16, paddingVertical: 10 },
  searchLabel: { fontSize: 14, fontWeight: '700' },
})
