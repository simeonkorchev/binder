import { useTranslation } from 'react-i18next'
import { FlatList, Pressable, StyleSheet, Text, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

import type { CapturedCard } from '../lib/capturedCards'

/**
 * A tile is a fixed width so the list can be measured without laying it out —
 * which is what lets `getItemLayout` exist, and a sweep can put a hundred cards
 * in here.
 */
const tileWidth = 132
const tileGap = 10

/**
 * What a tile says under the code while the resolver has not named the card.
 * Every state is a word, never a colour on its own: a strip that showed failure
 * as a red border alone would say nothing to a colour-blind user, and nothing
 * at all to a screen reader.
 */
const statusKeys = {
  pending: 'scan.captured.pending',
  unmatched: 'scan.captured.unmatched',
  refused: 'scan.captured.refused',
} as const satisfies Record<Exclude<CapturedCard['status'], 'matched'>, string>

interface CapturedStripProps {
  /** The sweep's captures, newest first. */
  cards: CapturedCard[]
  /** True when the resolver could not be reached — the captures are waiting, not lost. */
  isOffline: boolean
  onRetry: () => void
}

/**
 * The sweep's running record: how many cards have been captured and which ones.
 *
 * It is the only thing on the screen that survives a card leaving the guide
 * frame, so it is what the user checks to know the sweep is working — the
 * haptic tick says *something* was captured, and this says *what*.
 */
export const CapturedStrip = ({
  cards,
  isOffline,
  onRetry,
}: CapturedStripProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()

  return (
    <View style={[styles.container, { backgroundColor: colors.surface, borderTopColor: colors.border }]}>
      <View
        style={styles.header}
        accessible
        accessibilityLabel={t('scan.captured.countAccessible', { captured: cards.length })}
      >
        <Text style={[styles.count, { color: colors.textPrimary }]}>{cards.length}</Text>
        <Text style={[styles.countLabel, { color: colors.textSecondary }]}>
          {t('scan.captured.label')}
        </Text>
      </View>

      {isOffline ? (
        <View style={styles.offline}>
          <Text style={[styles.offlineMessage, { color: colors.error }]}>
            {t('scan.offline.message')}
          </Text>
          <Pressable
            onPress={onRetry}
            accessibilityRole="button"
            accessibilityLabel={t('scan.offline.retry')}
            style={[styles.retry, { borderColor: colors.accent }]}
          >
            <Text style={[styles.retryLabel, { color: colors.accent }]}>
              {t('scan.offline.retry')}
            </Text>
          </Pressable>
        </View>
      ) : null}

      {cards.length === 0 ? (
        <Text style={[styles.empty, { color: colors.textSecondary }]}>
          {t('scan.captured.empty')}
        </Text>
      ) : (
        <FlatList
          horizontal
          data={cards}
          keyExtractor={(card) => card.key}
          showsHorizontalScrollIndicator={false}
          contentContainerStyle={styles.list}
          getItemLayout={(_data, index) => ({
            length: tileWidth + tileGap,
            offset: (tileWidth + tileGap) * index,
            index,
          })}
          renderItem={({ item }) => (
            <View
              style={[styles.tile, { backgroundColor: colors.surfaceMuted, borderColor: colors.border }]}
            >
              <Text style={[styles.tileCode, { color: colors.textPrimary }]} numberOfLines={1}>
                {item.code}
              </Text>
              <Text style={[styles.tileName, { color: colors.textSecondary }]} numberOfLines={2}>
                {item.status === 'matched' ? item.name : t(statusKeys[item.status])}
              </Text>
            </View>
          )}
        />
      )}
    </View>
  )
}

const styles = StyleSheet.create({
  container: { borderTopWidth: StyleSheet.hairlineWidth, paddingBottom: 16, paddingTop: 12 },
  count: { fontSize: 24, fontWeight: '700' },
  countLabel: { fontSize: 14, marginLeft: 8 },
  empty: { fontSize: 14, paddingHorizontal: 16, paddingVertical: 12, textAlign: 'center' },
  header: { alignItems: 'baseline', flexDirection: 'row', paddingHorizontal: 16 },
  list: { gap: tileGap, paddingHorizontal: 16, paddingVertical: 12 },
  offline: { paddingHorizontal: 16, paddingTop: 8 },
  offlineMessage: { fontSize: 13, lineHeight: 18 },
  retry: { alignSelf: 'flex-start', borderRadius: 8, borderWidth: 1, marginTop: 8, paddingHorizontal: 14, paddingVertical: 8 },
  retryLabel: { fontSize: 14, fontWeight: '600' },
  tile: { borderRadius: 10, borderWidth: 1, padding: 10, width: tileWidth },
  tileCode: { fontSize: 14, fontWeight: '600' },
  tileName: { fontSize: 12, marginTop: 4 },
})
