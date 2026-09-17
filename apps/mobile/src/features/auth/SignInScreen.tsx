import { useNavigation } from '@react-navigation/native'
import { useTranslation } from 'react-i18next'
import { Platform, Pressable, StyleSheet, Text, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

import { useSignIn } from './api/useSignIn'
import type { AuthProvider } from './types'

/**
 * The way in, and the one screen a signed-out user lands on.
 *
 * It is not a wall. Browsing what other collectors are selling needs no account
 * — `GET /listings` is the one public endpoint, by design — so the market is
 * reachable from here without signing in. Everything else (a binder, a scan, a
 * listing of your own) is somebody's own data and needs the session.
 *
 * Apple sign-in is offered on iOS only: the native sheet is an iOS API, and
 * offering Google without Apple is what the App Store refuses (D1).
 */
const SignInScreen = (): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const navigation = useNavigation()
  const { status, signIn } = useSignIn()

  return (
    <View style={[styles.screen, { backgroundColor: colors.background }]}>
      <Text style={[styles.title, { color: colors.textPrimary }]}>{t('auth.title')}</Text>
      <Text style={[styles.intro, { color: colors.textSecondary }]}>{t('auth.intro')}</Text>

      {Platform.OS === 'ios' ? (
        <ProviderButton
          label={t('auth.apple')}
          provider="apple"
          busy={status === 'signing-in'}
          onSignIn={signIn}
        />
      ) : null}

      <ProviderButton
        label={t('auth.google')}
        provider="google"
        busy={status === 'signing-in'}
        onSignIn={signIn}
      />

      {status === 'signing-in' ? (
        <Text style={[styles.note, { color: colors.textSecondary }]}>{t('auth.signingIn')}</Text>
      ) : null}

      {/* One sentence for a refused sign-in, and no detail behind it: the server
          does not say which check failed, and nothing the provider handed over
          may reach the screen (004-security.md). */}
      {status === 'failed' ? (
        <Text style={[styles.error, { color: colors.error }]}>{t('auth.failed')}</Text>
      ) : null}

      {status === 'unavailable' ? (
        <Text style={[styles.error, { color: colors.error }]}>{t('auth.unavailable')}</Text>
      ) : null}

      <Pressable
        onPress={() => navigation.navigate('BrowseMarket')}
        accessibilityRole="button"
        accessibilityLabel={t('auth.browse')}
        style={styles.browse}
      >
        <Text style={[styles.browseLabel, { color: colors.accent }]}>{t('auth.browse')}</Text>
      </Pressable>
    </View>
  )
}

export default SignInScreen

interface ProviderButtonProps {
  label: string
  provider: AuthProvider
  busy: boolean
  onSignIn: (provider: AuthProvider) => Promise<void>
}

/**
 * One provider, one press.
 *
 * Both buttons are drawn from the theme tokens rather than with each provider's
 * own colours: a hardcoded palette is what `003-frontend.md` rules out, and the
 * branded Apple button is a native view that cannot be screenshotted in this
 * environment to check it against either theme.
 */
const ProviderButton = ({
  label,
  provider,
  busy,
  onSignIn,
}: ProviderButtonProps): React.JSX.Element => {
  const { colors } = useTheme()

  return (
    <Pressable
      onPress={() => void onSignIn(provider)}
      disabled={busy}
      accessibilityRole="button"
      accessibilityLabel={label}
      accessibilityState={{ disabled: busy }}
      style={[styles.provider, { backgroundColor: colors.accent, opacity: busy ? 0.6 : 1 }]}
    >
      <Text style={[styles.providerLabel, { color: colors.onAccent }]}>{label}</Text>
    </Pressable>
  )
}

const styles = StyleSheet.create({
  browse: { alignSelf: 'center', marginTop: 32, paddingHorizontal: 16, paddingVertical: 12 },
  browseLabel: { fontSize: 15, fontWeight: '600' },
  error: { fontSize: 14, lineHeight: 20, marginTop: 16, textAlign: 'center' },
  intro: { fontSize: 15, lineHeight: 22, marginBottom: 32, textAlign: 'center' },
  note: { fontSize: 14, lineHeight: 20, marginTop: 16, textAlign: 'center' },
  provider: { borderRadius: 10, marginTop: 12, paddingHorizontal: 20, paddingVertical: 14 },
  providerLabel: { fontSize: 16, fontWeight: '600', textAlign: 'center' },
  screen: { flex: 1, justifyContent: 'center', padding: 24 },
  title: { fontSize: 28, fontWeight: '700', marginBottom: 12, textAlign: 'center' },
})
