import { useTranslation } from 'react-i18next'
import { Pressable, StyleSheet, Text } from 'react-native'

import { forgetSession } from '@/lib/sessionStore'
import { useTheme } from '@/theme/useTheme'

/**
 * The way out, in the binders header — the tab that holds what is yours.
 *
 * It calls the store directly rather than through a hook of its own: there is
 * nothing to fetch, nothing to derive and no state to hold, and a hook that
 * forwarded one function unchanged would be a layer that does nothing
 * (003-frontend.md §1c). Dropping the session takes the token out of the
 * keychain and publishes `signed-out`, which is what brings the sign-in screen
 * back.
 */
export const SignOutButton = (): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()

  return (
    <Pressable
      onPress={() => void forgetSession()}
      accessibilityRole="button"
      accessibilityLabel={t('auth.signOut')}
      style={styles.button}
    >
      <Text style={[styles.label, { color: colors.accent }]}>{t('auth.signOut')}</Text>
    </Pressable>
  )
}

const styles = StyleSheet.create({
  button: { paddingHorizontal: 12, paddingVertical: 8 },
  label: { fontSize: 15, fontWeight: '600' },
})
