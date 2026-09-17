import { useTranslation } from 'react-i18next'
import { Modal, Pressable, StyleSheet, Text, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

import { moveDestination, pocketOf, type MoveAction, type PageShape } from '../lib/positions'
import { hasKnownSet, resolutionLabelKeys } from '../lib/setResolution'
import type { SlotBody } from '../types'

/** Every move a drag can express, as a button. Order reads as one step, then one page. */
const actions = [
  { action: 'pocketBack', labelKey: 'binder.actions.pocketBack' },
  { action: 'pocketForward', labelKey: 'binder.actions.pocketForward' },
  { action: 'previousPage', labelKey: 'binder.actions.previousPage' },
  { action: 'nextPage', labelKey: 'binder.actions.nextPage' },
] as const satisfies readonly { action: MoveAction; labelKey: string }[]

interface PocketActionsProps {
  slot: SlotBody
  shape: PageShape
  onMove: (toPosition: number) => void
  onRemove: () => void
  onClose: () => void
}

/**
 * The non-drag way to do everything a drag does, and the only way to remove a
 * card.
 *
 * It exists because a control reachable only by holding and dragging is
 * reachable by neither a screen reader nor a hand that cannot hold a press
 * steady, and "move it across the page boundary" is not a capability this app
 * may offer to only some of its users (003-frontend.md §10). Both routes end in
 * the same `moveDestination`, so the buttons cannot drift from the gesture.
 */
export const PocketActions = ({
  slot,
  shape,
  onMove,
  onRemove,
  onClose,
}: PocketActionsProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()

  return (
    <Modal visible transparent animationType="fade" onRequestClose={onClose}>
      <View style={styles.backdrop}>
        <View style={[styles.sheet, { backgroundColor: colors.surface, borderColor: colors.border }]}>
          <Text style={[styles.title, { color: colors.textPrimary }]}>
            {t('binder.actions.title')}
          </Text>
          <Text style={[styles.subtitle, { color: colors.textSecondary }]}>
            {t('binder.pocket.cardLabel', {
              pocket: pocketOf(slot.position) + 1,
              resolution: t(resolutionLabelKeys[slot.setResolution]),
            })}
          </Text>
          {hasKnownSet(slot.setResolution) ? null : (
            <Text style={[styles.unknownSet, { color: colors.textSecondary }]}>
              {t('binder.add.note')}
            </Text>
          )}

          {actions.map(({ action, labelKey }) => {
            const destination = moveDestination(action, slot.position, shape)

            return (
              <Pressable
                key={action}
                disabled={destination === null}
                onPress={() => {
                  if (destination !== null) onMove(destination)
                }}
                accessibilityRole="button"
                accessibilityLabel={t(labelKey)}
                accessibilityState={{ disabled: destination === null }}
                style={[
                  styles.action,
                  { borderColor: colors.border },
                  destination === null ? styles.unavailable : null,
                ]}
              >
                <Text style={[styles.actionLabel, { color: colors.textPrimary }]}>{t(labelKey)}</Text>
              </Pressable>
            )
          })}

          <Pressable
            onPress={onRemove}
            accessibilityRole="button"
            accessibilityLabel={t('binder.actions.remove')}
            style={[styles.action, { borderColor: colors.error }]}
          >
            <Text style={[styles.actionLabel, { color: colors.error }]}>
              {t('binder.actions.remove')}
            </Text>
          </Pressable>

          <Pressable
            onPress={onClose}
            accessibilityRole="button"
            accessibilityLabel={t('binder.actions.close')}
            style={[styles.close, { backgroundColor: colors.accent }]}
          >
            <Text style={[styles.closeLabel, { color: colors.onAccent }]}>
              {t('binder.actions.close')}
            </Text>
          </Pressable>
        </View>
      </View>
    </Modal>
  )
}

const styles = StyleSheet.create({
  action: { borderRadius: 8, borderWidth: 1, marginTop: 8, paddingHorizontal: 14, paddingVertical: 12 },
  actionLabel: { fontSize: 15, fontWeight: '600' },
  backdrop: { flex: 1, justifyContent: 'flex-end' },
  close: { alignItems: 'center', borderRadius: 8, marginTop: 16, paddingVertical: 12 },
  closeLabel: { fontSize: 15, fontWeight: '700' },
  sheet: { borderTopLeftRadius: 16, borderTopRightRadius: 16, borderWidth: StyleSheet.hairlineWidth, padding: 20 },
  subtitle: { fontSize: 13, marginTop: 4 },
  title: { fontSize: 18, fontWeight: '700' },
  unavailable: { opacity: 0.4 },
  unknownSet: { fontSize: 13, lineHeight: 18, marginTop: 8 },
})
