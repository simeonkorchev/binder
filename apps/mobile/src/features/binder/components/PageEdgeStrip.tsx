import { useTranslation } from 'react-i18next'
import { StyleSheet, Text, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

interface PageEdgeStripProps {
  labelKey: 'binder.page.previous' | 'binder.page.next'
  /** True while a dragged card is over the strip, so the drop reads as armed. */
  isActive: boolean
}

/** The page boundary, made droppable: the only way a drag reaches a page the grid is not showing. */
export const PageEdgeStrip = ({ labelKey, isActive }: PageEdgeStripProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()

  return (
    <View
      style={[styles.strip, { backgroundColor: isActive ? colors.accent : colors.surfaceMuted }]}
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
    >
      {isActive ? (
        <Text style={[styles.stripLabel, { color: colors.onAccent }]} numberOfLines={3}>
          {t(labelKey)}
        </Text>
      ) : null}
    </View>
  )
}

const styles = StyleSheet.create({
  strip: { borderRadius: 6, justifyContent: 'center', marginVertical: 5, paddingHorizontal: 2, width: 24 },
  stripLabel: { fontSize: 9, fontWeight: '700', textAlign: 'center' },
})
