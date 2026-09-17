import { useTranslation } from 'react-i18next'
import { StyleSheet, Text, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

/**
 * Scaffolding: what a route renders until the wave that owns it lands.
 *
 * Every route in `AppNavigator` is registered with this so the five
 * destinations exist and can be navigated between today; W9–W11 replace the
 * registrations one at a time with the real screen. The header title comes
 * from the navigator, so this body only has to say why the screen is bare.
 */
export const PlaceholderScreen = (): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()

  return (
    <View style={[styles.container, { backgroundColor: colors.background }]}>
      <Text style={[styles.message, { color: colors.textSecondary }]}>
        {t('common.screenNotBuilt')}
      </Text>
    </View>
  )
}

const styles = StyleSheet.create({
  container: { alignItems: 'center', flex: 1, justifyContent: 'center', padding: 24 },
  message: { fontSize: 16, textAlign: 'center' },
})
