import { useNavigation } from '@react-navigation/native'
import { useTranslation } from 'react-i18next'
import { FlatList, Pressable, StyleSheet, Text, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

import { useListings } from './api/useListings'
import { ListingFilters } from './components/ListingFilters'
import { ListingRow } from './components/ListingRow'
import { isNarrowed, type ListingFilter } from './lib/listingsPath'

/**
 * What other collectors are selling.
 *
 * The screen keeps the three states apart, and keeps the two empty ones apart
 * too: a market nobody has listed in yet and a filter that matched nothing are
 * the same response and different sentences. Telling a buyer who searched for
 * "Kuriboh" that nothing is for sale would be a lie about the whole market.
 *
 * Contact details are deliberately not here. The feed carries a `sellerId` and
 * nothing else about a seller, so scrolling cannot collect anybody's email
 * address; revealing one is a separate request the buyer makes on purpose.
 */
const MarketScreen = (): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const navigation = useNavigation()
  const listings = useListings()

  return (
    <View style={[styles.screen, { backgroundColor: colors.background }]}>
      <ListingFilters onBrowse={listings.browse} />

      {listings.state.status === 'loading' ? (
        <Text style={[styles.status, { color: colors.textSecondary }]}>
          {t('market.browse.loading')}
        </Text>
      ) : null}

      {listings.state.status === 'error' ? (
        <View style={styles.failure}>
          <Text style={[styles.error, { color: colors.error }]}>
            {t('market.browse.loadError')}
          </Text>
          <Pressable
            onPress={listings.reload}
            accessibilityRole="button"
            accessibilityLabel={t('market.browse.retry')}
            style={[styles.retry, { borderColor: colors.accent }]}
          >
            <Text style={[styles.retryLabel, { color: colors.accent }]}>
              {t('market.browse.retry')}
            </Text>
          </Pressable>
        </View>
      ) : null}

      {listings.state.status === 'ready' ? (
        <FlatList
          data={listings.state.listings}
          keyExtractor={(listing) => listing.listingId}
          contentContainerStyle={styles.list}
          keyboardShouldPersistTaps="handled"
          ListEmptyComponent={<NothingForSale filter={listings.state.filter} />}
          renderItem={({ item }) => (
            <ListingRow
              listing={item}
              onContact={() => navigation.navigate('SellerContact', { sellerId: item.sellerId })}
            />
          )}
        />
      ) : null}
    </View>
  )
}

export default MarketScreen

/** The two things an empty feed can mean, said as the two different things they are. */
const NothingForSale = ({ filter }: { filter: ListingFilter }): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()

  return (
    <Text style={[styles.status, { color: colors.textSecondary }]}>
      {t(isNarrowed(filter) ? 'market.browse.emptyFiltered' : 'market.browse.empty')}
    </Text>
  )
}

const styles = StyleSheet.create({
  error: { fontSize: 14, lineHeight: 20 },
  failure: { padding: 16 },
  list: { padding: 16 },
  retry: { alignSelf: 'flex-start', borderRadius: 8, borderWidth: 1, marginTop: 12, paddingHorizontal: 14, paddingVertical: 10 },
  retryLabel: { fontSize: 14, fontWeight: '600' },
  screen: { flex: 1 },
  status: { fontSize: 14, lineHeight: 20, padding: 16 },
})
