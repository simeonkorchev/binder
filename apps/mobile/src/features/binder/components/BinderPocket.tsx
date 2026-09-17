import { useTranslation } from 'react-i18next'
import { StyleSheet, Text, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

import { hasKnownSet, resolutionLabelKeys } from '../lib/setResolution'
import type { SlotBody } from '../types'

interface BinderPocketProps {
  /** Zero-based position on the page; shown to the user as 1 to 9. */
  pocket: number
  slot: SlotBody | null
  /** True for the one empty pocket a card would land in — the rest cannot take one. */
  canAdd: boolean
  /** True while this pocket's card is being dragged, so the pocket reads as vacated. */
  isLifted: boolean
  /** Screen-reader activation. Touch is handled by the grid, which owns the drag. */
  onActivate: () => void
}

/**
 * One pocket of the page: a card, the pocket a card would go into, or an empty
 * pocket.
 *
 * A card whose set was never determined is marked twice over — a dashed border
 * *and* the words for the rung it stopped at — because a border colour alone
 * says nothing to a colour-blind user and nothing at all to a screen reader
 * (003-frontend.md §10).
 *
 * There is no card name here, and not for want of asking: `SlotBody` carries a
 * card **id**, a printing id and a resolution, and the API has no lookup that
 * turns an id into a name. Inventing one would mean showing a name this screen
 * cannot know is right.
 */
export const BinderPocket = ({
  pocket,
  slot,
  canAdd,
  isLifted,
  onActivate,
}: BinderPocketProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const number = pocket + 1

  if (slot === null) {
    return (
      <View
        style={styles.cell}
        accessible
        accessibilityRole={canAdd ? 'button' : 'none'}
        accessibilityLabel={t(canAdd ? 'binder.pocket.addLabel' : 'binder.pocket.emptyLabel', {
          pocket: number,
        })}
        onAccessibilityTap={canAdd ? onActivate : undefined}
      >
        <View
          style={[
            styles.card,
            styles.emptyCard,
            { backgroundColor: colors.surfaceMuted, borderColor: colors.border },
          ]}
        >
          <Text style={[styles.pocketNumber, { color: colors.textSecondary }]}>{number}</Text>
          <Text style={[styles.emptyLabel, { color: colors.textSecondary }]} numberOfLines={2}>
            {t(canAdd ? 'binder.pocket.add' : 'binder.pocket.empty')}
          </Text>
        </View>
      </View>
    )
  }

  const known = hasKnownSet(slot.setResolution)
  const resolution = t(resolutionLabelKeys[slot.setResolution])

  return (
    <View
      style={styles.cell}
      accessible
      accessibilityRole="button"
      accessibilityLabel={t('binder.pocket.cardLabel', { pocket: number, resolution })}
      onAccessibilityTap={onActivate}
    >
      <View
        style={[
          styles.card,
          known ? styles.knownCard : styles.unknownCard,
          {
            backgroundColor: isLifted ? colors.surfaceMuted : colors.surface,
            borderColor: known ? colors.border : colors.textSecondary,
          },
        ]}
      >
        <Text style={[styles.pocketNumber, { color: colors.textSecondary }]}>{number}</Text>
        {isLifted ? null : (
          <Text style={[styles.resolution, { color: colors.textPrimary }]} numberOfLines={3}>
            {resolution}
          </Text>
        )}
      </View>
    </View>
  )
}

const styles = StyleSheet.create({
  // A cell is exactly a third of the grid in each direction, with the gap made
  // by the card's margin inside it. That keeps the boxes the finger is hit
  // tested against (`lib/dropTarget.ts`) the same boxes the eye sees.
  card: { borderRadius: 8, flex: 1, margin: 5, padding: 8 },
  cell: { flex: 1 },
  emptyCard: { borderStyle: 'dashed', borderWidth: 1 },
  emptyLabel: { fontSize: 12, marginTop: 6 },
  knownCard: { borderWidth: 1 },
  pocketNumber: { fontSize: 11, fontWeight: '600' },
  resolution: { fontSize: 12, lineHeight: 16, marginTop: 6 },
  unknownCard: { borderStyle: 'dashed', borderWidth: 2 },
})
