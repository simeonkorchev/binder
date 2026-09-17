import { useTranslation } from 'react-i18next'
import { Pressable, StyleSheet, Text, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

import type { CameraAccess } from '../useCameraAccess'

/** Every state that is not a working camera. A granted one renders the scanner. */
type RefusedAccess = Exclude<CameraAccess, 'granted'>

/**
 * The copy each refusal gets. They are three separate entries rather than one
 * apologetic paragraph because the user's next move differs: one has not been
 * asked yet, one said no and can say yes, and one can only be changed in the
 * settings app.
 */
const noticeKeys = {
  undetermined: { title: 'scan.permission.title', reason: 'scan.permission.reason' },
  denied: { title: 'scan.permission.deniedTitle', reason: 'scan.permission.deniedReason' },
  blocked: { title: 'scan.permission.blockedTitle', reason: 'scan.permission.blockedReason' },
} as const satisfies Record<RefusedAccess, { title: string; reason: string }>

interface CameraAccessNoticeProps {
  access: RefusedAccess
  /** Asks the OS again — the way out of everything but `blocked`. */
  onRequest: () => void
  /** Opens the settings app — the only way out of `blocked`. */
  onOpenSettings: () => void
}

/**
 * What the scan tab shows instead of a viewfinder when there is no camera to
 * show. Every state here ends in a button that can actually change it: a
 * scanner that dead-ends on a refusal is broken, and this is the first screen
 * a new user sees.
 */
export const CameraAccessNotice = ({
  access,
  onRequest,
  onOpenSettings,
}: CameraAccessNoticeProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()

  const isBlocked = access === 'blocked'
  const actionLabel = t(isBlocked ? 'scan.permission.openSettings' : 'scan.permission.allow')

  return (
    <View style={[styles.container, { backgroundColor: colors.background }]}>
      <Text style={[styles.title, { color: colors.textPrimary }]}>{t(noticeKeys[access].title)}</Text>
      <Text style={[styles.reason, { color: colors.textSecondary }]}>
        {t(noticeKeys[access].reason)}
      </Text>
      <Pressable
        style={[styles.action, { backgroundColor: colors.accent }]}
        onPress={isBlocked ? onOpenSettings : onRequest}
        accessibilityRole="button"
        accessibilityLabel={actionLabel}
      >
        <Text style={[styles.actionLabel, { color: colors.onAccent }]}>{actionLabel}</Text>
      </Pressable>
    </View>
  )
}

const styles = StyleSheet.create({
  action: {
    borderRadius: 10,
    marginTop: 28,
    paddingHorizontal: 24,
    paddingVertical: 14,
  },
  actionLabel: { fontSize: 16, fontWeight: '600' },
  container: {
    alignItems: 'center',
    flex: 1,
    justifyContent: 'center',
    padding: 32,
  },
  reason: { fontSize: 15, lineHeight: 22, marginTop: 12, textAlign: 'center' },
  title: { fontSize: 20, fontWeight: '600', textAlign: 'center' },
})
