import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { FlatList, Pressable, StyleSheet, Text, TextInput, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

import { useCardSearch } from '@/features/card/api/useCardSearch'
import type { ScannedCard } from '../types'

interface CardSearchPickerProps {
  /** The scanned code being corrected, so the user can see which card they are naming. */
  code: string
  onPick: (card: ScannedCard) => void
  onCancel: () => void
}

/**
 * Naming the card the scanner could not.
 *
 * Only the text in the field is held here — the search itself is
 * `useCardSearch`. The result list is a card and never a printing, because
 * `GET /cards` answers with cards alone: a card picked by name is filed with
 * its set still unknown, which is the truthful outcome and the one the binder's
 * own null-printing slot records.
 */
export const CardSearchPicker = ({
  code,
  onPick,
  onCancel,
}: CardSearchPickerProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const search = useCardSearch()
  const [name, setName] = useState('')

  return (
    <View style={styles.container}>
      <Text style={[styles.title, { color: colors.textPrimary }]}>{t('scan.search.title')}</Text>
      <Text style={[styles.code, { color: colors.textSecondary }]}>
        {t('scan.review.code', { code })}
      </Text>

      <View style={styles.field}>
        <TextInput
          value={name}
          onChangeText={setName}
          onSubmitEditing={() => search.search(name)}
          placeholder={t('scan.search.placeholder')}
          placeholderTextColor={colors.textSecondary}
          accessibilityLabel={t('scan.search.label')}
          autoCorrect={false}
          returnKeyType="search"
          style={[
            styles.input,
            { backgroundColor: colors.surfaceMuted, borderColor: colors.border, color: colors.textPrimary },
          ]}
        />
        <Pressable
          onPress={() => search.search(name)}
          accessibilityRole="button"
          accessibilityLabel={t('scan.search.submit')}
          style={[styles.submit, { backgroundColor: colors.accent }]}
        >
          <Text style={[styles.submitLabel, { color: colors.onAccent }]}>
            {t('scan.search.submit')}
          </Text>
        </Pressable>
      </View>

      <CardSearchResults
        cards={search.results}
        isSearching={search.isSearching}
        hasFailed={search.hasFailed}
        hasSearched={search.hasSearched}
        onPick={onPick}
      />

      <Pressable
        onPress={onCancel}
        accessibilityRole="button"
        accessibilityLabel={t('scan.search.back')}
        style={[styles.back, { borderColor: colors.border }]}
      >
        <Text style={[styles.backLabel, { color: colors.textPrimary }]}>{t('scan.search.back')}</Text>
      </Pressable>
    </View>
  )
}

interface CardSearchResultsProps {
  cards: ScannedCard[]
  isSearching: boolean
  hasFailed: boolean
  hasSearched: boolean
  onPick: (card: ScannedCard) => void
}

/**
 * The four things the result area can be: working, broken, empty-because-nobody
 * -asked, and empty-because-nothing-matched. The last two are separate lines on
 * purpose — "no card by that name" about a search the user never ran would be a
 * lie, and one that tells them to try a shorter name is the help they need.
 */
const CardSearchResults = ({
  cards,
  isSearching,
  hasFailed,
  hasSearched,
  onPick,
}: CardSearchResultsProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()

  if (isSearching) {
    return <Text style={[styles.status, { color: colors.textSecondary }]}>{t('scan.search.searching')}</Text>
  }
  if (hasFailed) {
    return <Text style={[styles.status, { color: colors.error }]}>{t('scan.search.failed')}</Text>
  }
  if (cards.length === 0) {
    return (
      <Text style={[styles.status, { color: colors.textSecondary }]}>
        {t(hasSearched ? 'scan.search.empty' : 'scan.search.prompt')}
      </Text>
    )
  }

  return (
    <FlatList
      data={cards}
      keyExtractor={(card) => card.id}
      style={styles.results}
      keyboardShouldPersistTaps="handled"
      renderItem={({ item }) => (
        <Pressable
          onPress={() => onPick(item)}
          accessibilityRole="button"
          accessibilityLabel={t('scan.review.chooseNoSet', { name: item.name })}
          style={[styles.result, { borderColor: colors.border }]}
        >
          <Text style={[styles.resultName, { color: colors.textPrimary }]}>{item.name}</Text>
          <Text style={[styles.resultSet, { color: colors.textSecondary }]}>
            {t('scan.review.noSet')}
          </Text>
        </Pressable>
      )}
    />
  )
}

const styles = StyleSheet.create({
  back: { alignSelf: 'flex-start', borderRadius: 8, borderWidth: 1, marginTop: 16, paddingHorizontal: 14, paddingVertical: 10 },
  backLabel: { fontSize: 15, fontWeight: '600' },
  code: { fontSize: 13, marginTop: 4 },
  container: { flex: 1, padding: 16 },
  field: { alignItems: 'stretch', flexDirection: 'row', gap: 8, marginTop: 16 },
  input: { borderRadius: 8, borderWidth: 1, flex: 1, fontSize: 16, paddingHorizontal: 12, paddingVertical: 10 },
  result: { borderBottomWidth: StyleSheet.hairlineWidth, paddingVertical: 12 },
  resultName: { fontSize: 16, fontWeight: '600' },
  resultSet: { fontSize: 13, marginTop: 2 },
  results: { marginTop: 12 },
  status: { fontSize: 14, lineHeight: 20, marginTop: 20 },
  submit: { borderRadius: 8, justifyContent: 'center', paddingHorizontal: 16 },
  submitLabel: { fontSize: 15, fontWeight: '600' },
  title: { fontSize: 18, fontWeight: '700' },
})
