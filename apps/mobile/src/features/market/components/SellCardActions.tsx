import { useTranslation } from 'react-i18next'
import { Pressable, StyleSheet, Text } from 'react-native'

import { useTheme } from '@/theme/useTheme'

import { useSlotListing, type SaleStatus } from '../api/useSlotListing'

/** What to say about the card's last trip to the marketplace. `idle` says nothing. */
const noteKeys = {
  saving: 'market.sell.saving',
  listed: 'market.sell.listed',
  alreadyListed: 'market.sell.alreadyListed',
  unlisted: 'market.sell.unlisted',
  listFailed: 'market.sell.listFailed',
  unlistFailed: 'market.sell.unlistFailed',
} as const satisfies Record<Exclude<SaleStatus, 'idle'>, string>

/** Which of those sentences is a failure, so it is told in the failure colour too. */
const failures: readonly SaleStatus[] = ['listFailed', 'unlistFailed']

interface SellCardActionsProps {
  /** The binder slot holding the card. The slot is what is sold, not the card: the same card in two binders is two pieces of cardboard. */
  slotId: string
}

/**
 * Marking the card in a pocket for sale, and taking it off sale again.
 *
 * It lives beside the pocket's other actions rather than behind an affordance
 * of its own: the sheet a card opens is where everything that can be done to
 * that card belongs, and a second place to tap a card would be a second place
 * to keep in step with the first.
 */
export const SellCardActions = ({ slotId }: SellCardActionsProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const sale = useSlotListing(slotId)
  const isSaving = sale.status === 'saving'
  const label = t(sale.canUnlist ? 'market.sell.unlist' : 'market.sell.list')

  return (
    <>
      <Pressable
        onPress={() => void (sale.canUnlist ? sale.unlist() : sale.list())}
        disabled={isSaving}
        accessibilityRole="button"
        accessibilityLabel={label}
        accessibilityState={{ disabled: isSaving }}
        style={[styles.action, { borderColor: colors.accent }, isSaving ? styles.busy : null]}
      >
        <Text style={[styles.actionLabel, { color: colors.accent }]}>{label}</Text>
      </Pressable>

      {sale.status === 'idle' ? null : (
        <Text
          style={[
            styles.note,
            { color: failures.includes(sale.status) ? colors.error : colors.textSecondary },
          ]}
        >
          {t(noteKeys[sale.status])}
        </Text>
      )}
    </>
  )
}

const styles = StyleSheet.create({
  action: { borderRadius: 8, borderWidth: 1, marginTop: 8, paddingHorizontal: 14, paddingVertical: 12 },
  actionLabel: { fontSize: 15, fontWeight: '600' },
  busy: { opacity: 0.4 },
  note: { fontSize: 13, lineHeight: 18, marginTop: 8 },
})
