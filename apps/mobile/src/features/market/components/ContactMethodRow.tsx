import { useTranslation } from 'react-i18next'
import { Pressable, StyleSheet, Text, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

import type { ContactChannel, ContactMethod } from '../lib/contactMethods'
import type { ContactOutcome } from '../lib/useContactActions'

/** What each kind of detail is called, and what pressing it does, in the buyer's words. */
const channelKeys = {
  email: {
    label: 'market.contact.email',
    open: 'market.contact.openEmail',
    copy: 'market.contact.copyEmail',
  },
  phone: {
    label: 'market.contact.phone',
    open: 'market.contact.openPhone',
    copy: 'market.contact.copyPhone',
  },
} as const satisfies Record<ContactChannel, { label: string; open: string; copy: string }>

/** The sentence each outcome gets. A failure is said in words, never in a colour alone. */
const outcomeKeys = {
  copied: 'market.contact.copied',
  openFailed: 'market.contact.openFailed',
  copyFailed: 'market.contact.copyFailed',
} as const satisfies Record<ContactOutcome['kind'], string>

interface ContactMethodRowProps {
  method: ContactMethod
  /** What the last action on **this** detail did, or null when it was another one. */
  note: ContactOutcome['kind'] | null
  onOpen: () => void
  onCopy: () => void
}

/**
 * One way to reach the seller, with both ways to use it.
 *
 * The detail itself is the button that opens it, so the thing a buyer reads and
 * the thing they press are the same thing. Copy sits beside it because opening
 * can fail for reasons the app cannot fix — a phone with no mail account
 * configured has nothing to hand a `mailto:` to — and because a number that can
 * only be read off a screen is a number that gets retyped wrong.
 */
export const ContactMethodRow = ({
  method,
  note,
  onOpen,
  onCopy,
}: ContactMethodRowProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const keys = channelKeys[method.channel]

  return (
    <View style={[styles.row, { backgroundColor: colors.surface, borderColor: colors.border }]}>
      <Text style={[styles.channel, { color: colors.textSecondary }]}>{t(keys.label)}</Text>

      <Pressable
        onPress={onOpen}
        accessibilityRole="button"
        accessibilityLabel={t(keys.open, { value: method.value })}
        style={styles.value}
      >
        <Text style={[styles.valueText, { color: colors.accent }]}>{method.value}</Text>
      </Pressable>

      <Pressable
        onPress={onCopy}
        accessibilityRole="button"
        accessibilityLabel={t(keys.copy, { value: method.value })}
        style={[styles.copy, { borderColor: colors.border }]}
      >
        <Text style={[styles.copyLabel, { color: colors.textPrimary }]}>
          {t('market.contact.copy')}
        </Text>
      </Pressable>

      {note === null ? null : (
        <Text
          style={[styles.note, { color: note === 'copied' ? colors.textSecondary : colors.error }]}
        >
          {t(outcomeKeys[note])}
        </Text>
      )}
    </View>
  )
}

const styles = StyleSheet.create({
  channel: { fontSize: 13 },
  copy: { alignSelf: 'flex-start', borderRadius: 8, borderWidth: 1, marginTop: 12, paddingHorizontal: 14, paddingVertical: 10 },
  copyLabel: { fontSize: 14, fontWeight: '600' },
  note: { fontSize: 13, lineHeight: 18, marginTop: 10 },
  row: { borderRadius: 10, borderWidth: 1, marginBottom: 12, padding: 16 },
  value: { alignSelf: 'flex-start', marginTop: 4 },
  valueText: { fontSize: 18, fontWeight: '600' },
})
