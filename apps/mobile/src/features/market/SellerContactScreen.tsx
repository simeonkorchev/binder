import type { NativeStackScreenProps } from '@react-navigation/native-stack'
import { useTranslation } from 'react-i18next'
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native'

import type { RootStackParamList } from '@/navigation/AppNavigator'
import { useTheme } from '@/theme/useTheme'

import { useSellerContact } from './api/useSellerContact'
import { ContactMethodRow } from './components/ContactMethodRow'
import { useContactActions } from './lib/useContactActions'

type SellerContactScreenProps = NativeStackScreenProps<RootStackParamList, 'SellerContact'>

/**
 * How to reach one seller — and nothing else about them.
 *
 * A seller who published neither an address nor a number is the ordinary case,
 * not an edge one: the server answers 200 with two nulls, and this screen says
 * so in a sentence. Rendering that as a failure, or as a blank space where
 * buttons belong, would tell a buyer the app is broken when it is working
 * exactly as the seller asked it to.
 *
 * There is no chat here on purpose (D4): the app hands the buyer the seller's
 * own details and gets out of the way.
 */
const SellerContactScreen = ({ route }: SellerContactScreenProps): React.JSX.Element => {
  const { sellerId } = route.params
  const { t } = useTranslation()
  const { colors } = useTheme()
  const contact = useSellerContact(sellerId)
  const actions = useContactActions()

  if (contact.state.status === 'loading') {
    return (
      <View style={[styles.screen, { backgroundColor: colors.background }]}>
        <Text style={[styles.status, { color: colors.textSecondary }]}>
          {t('market.contact.loading')}
        </Text>
      </View>
    )
  }

  if (contact.state.status === 'error') {
    return (
      <View style={[styles.screen, { backgroundColor: colors.background }]}>
        <Text style={[styles.error, { color: colors.error }]}>
          {t('market.contact.loadError')}
        </Text>
        <Pressable
          onPress={contact.reload}
          accessibilityRole="button"
          accessibilityLabel={t('market.contact.retry')}
          style={[styles.retry, { borderColor: colors.accent }]}
        >
          <Text style={[styles.retryLabel, { color: colors.accent }]}>
            {t('market.contact.retry')}
          </Text>
        </Pressable>
      </View>
    )
  }

  const { methods } = contact.state

  return (
    <ScrollView
      style={[styles.screen, { backgroundColor: colors.background }]}
      contentContainerStyle={styles.content}
    >
      {methods.length === 0 ? (
        <Text style={[styles.status, { color: colors.textSecondary }]}>
          {t('market.contact.none')}
        </Text>
      ) : null}

      {methods.map((method) => (
        <ContactMethodRow
          key={method.channel}
          method={method}
          note={actions.outcome?.channel === method.channel ? actions.outcome.kind : null}
          onOpen={() => actions.open(method)}
          onCopy={() => actions.copy(method)}
        />
      ))}
    </ScrollView>
  )
}

export default SellerContactScreen

const styles = StyleSheet.create({
  content: { padding: 16 },
  error: { fontSize: 14, lineHeight: 20, padding: 16 },
  retry: { alignSelf: 'flex-start', borderRadius: 8, borderWidth: 1, marginHorizontal: 16, paddingHorizontal: 14, paddingVertical: 10 },
  retryLabel: { fontSize: 14, fontWeight: '600' },
  screen: { flex: 1 },
  status: { fontSize: 15, lineHeight: 22, padding: 16 },
})
