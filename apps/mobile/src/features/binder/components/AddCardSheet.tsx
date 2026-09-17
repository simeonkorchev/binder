import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { FlatList, Modal, Pressable, StyleSheet, Text, TextInput, View } from 'react-native'

// The card database is neither feature's property — both the scan review and
// this sheet search it, and one search hook is better than two copies of it.
// It lives under `features/scan/` because that is where it was first needed;
// its home is a `features/card/` of its own, which is a move for the session
// after W9 stops writing to that tree.
import { useCardSearch } from '@/features/scan/api/useCardSearch'
import { useTheme } from '@/theme/useTheme'

import type { CardSummary } from '../types'

interface AddCardSheetProps {
  onPick: (card: CardSummary) => void
  onClose: () => void
}

/**
 * Putting a card into the binder by naming it.
 *
 * A card found this way is filed with **no set**: `GET /cards` answers with
 * cards and not printings, so the only truthful resolution for it is `by_name`,
 * and the pocket it lands in says so. The note in the sheet tells the user that
 * before they pick, because it is the difference between this route and
 * scanning the card.
 */
export const AddCardSheet = ({ onPick, onClose }: AddCardSheetProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const search = useCardSearch()
  const [name, setName] = useState('')

  return (
    <Modal visible transparent animationType="slide" onRequestClose={onClose}>
      <View style={[styles.sheet, { backgroundColor: colors.background }]}>
        <Text style={[styles.title, { color: colors.textPrimary }]}>{t('binder.add.title')}</Text>
        <Text style={[styles.note, { color: colors.textSecondary }]}>{t('binder.add.note')}</Text>

        <TextInput
          value={name}
          onChangeText={setName}
          onSubmitEditing={() => search.search(name)}
          placeholder={t('binder.add.searchPlaceholder')}
          placeholderTextColor={colors.textSecondary}
          accessibilityLabel={t('binder.add.searchLabel')}
          autoCorrect={false}
          returnKeyType="search"
          style={[
            styles.input,
            { backgroundColor: colors.surfaceMuted, borderColor: colors.border, color: colors.textPrimary },
          ]}
        />

        <SearchResults
          cards={search.results}
          isSearching={search.isSearching}
          hasFailed={search.hasFailed}
          hasSearched={search.hasSearched}
          onPick={onPick}
        />

        <Pressable
          onPress={onClose}
          accessibilityRole="button"
          accessibilityLabel={t('binder.add.close')}
          style={[styles.close, { borderColor: colors.border }]}
        >
          <Text style={[styles.closeLabel, { color: colors.textPrimary }]}>
            {t('binder.add.close')}
          </Text>
        </Pressable>
      </View>
    </Modal>
  )
}

interface SearchResultsProps {
  cards: CardSummary[]
  isSearching: boolean
  hasFailed: boolean
  hasSearched: boolean
  onPick: (card: CardSummary) => void
}

/**
 * The results, and the four things their absence can mean: the search is still
 * running, it broke, nobody has searched yet, or nothing matched. Collapsing
 * the last two would tell a user who has typed nothing that their card does not
 * exist.
 */
const SearchResults = ({
  cards,
  isSearching,
  hasFailed,
  hasSearched,
  onPick,
}: SearchResultsProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()

  if (isSearching) {
    return (
      <Text style={[styles.status, { color: colors.textSecondary }]}>
        {t('binder.add.searching')}
      </Text>
    )
  }
  if (hasFailed) {
    return <Text style={[styles.status, { color: colors.error }]}>{t('binder.add.error')}</Text>
  }
  if (cards.length === 0) {
    return (
      <Text style={[styles.status, { color: colors.textSecondary }]}>
        {t(hasSearched ? 'binder.add.empty' : 'binder.add.prompt')}
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
          accessibilityLabel={t('binder.add.addLabel', { name: item.name })}
          style={[styles.result, { borderBottomColor: colors.border }]}
        >
          <Text style={[styles.resultName, { color: colors.textPrimary }]}>{item.name}</Text>
        </Pressable>
      )}
    />
  )
}

const styles = StyleSheet.create({
  close: { alignSelf: 'flex-start', borderRadius: 8, borderWidth: 1, marginTop: 16, paddingHorizontal: 14, paddingVertical: 10 },
  closeLabel: { fontSize: 15, fontWeight: '600' },
  input: { borderRadius: 8, borderWidth: 1, fontSize: 16, marginTop: 16, paddingHorizontal: 12, paddingVertical: 10 },
  note: { fontSize: 13, lineHeight: 18, marginTop: 6 },
  result: { borderBottomWidth: StyleSheet.hairlineWidth, paddingVertical: 12 },
  resultName: { fontSize: 16, fontWeight: '600' },
  results: { marginTop: 12 },
  sheet: { flex: 1, padding: 20 },
  status: { fontSize: 14, lineHeight: 20, marginTop: 20 },
  title: { fontSize: 20, fontWeight: '700' },
})
