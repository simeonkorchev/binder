import { useTranslation } from 'react-i18next'
import { StyleSheet, Text, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

/**
 * A trading card is 59 × 86 mm, so a frame the user can fill exactly has that
 * ratio and nothing else. A frame the card does not fill is the main cause of
 * a code too small for ML Kit to read.
 */
const cardAspectRatio = 59 / 86

/**
 * Where the printed code sits on the card, as a fraction of the card's height
 * and width. It is **below the artwork**, on the bottom edge and to the right
 * — never over the art — and that is the whole reason this component exists:
 * the code line is a few millimetres tall, so the difference between a read and
 * a miss is whether the user put *that strip* under the lens, not whether the
 * card is roughly in view. D5 decided the frame is fixed, with no edge
 * detection, so this hint is the only aiming help there is.
 */
const codeStrip = { bottom: '3%', right: '5%', width: '56%', height: '9%' } as const

/**
 * The aiming overlay: a card-shaped frame with the code line's position marked.
 *
 * It draws over the camera preview and takes no touches — nothing here is
 * interactive, and swallowing a tap would break the screen underneath.
 */
export const ScanGuideFrame = (): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()

  return (
    <View style={styles.overlay} pointerEvents="none">
      <View style={[styles.frame, { borderColor: colors.accent }]}>
        <View style={[styles.codeStrip, { borderColor: colors.accent }]}>
          <Text style={[styles.codeLabel, { backgroundColor: colors.surface, color: colors.textSecondary }]}>
            {t('scan.guide.codeLabel')}
          </Text>
        </View>
      </View>
      <Text style={[styles.caption, { backgroundColor: colors.surface, color: colors.textPrimary }]}>
        {t('scan.guide.caption')}
      </Text>
    </View>
  )
}

// The frame and the caption sit on the camera preview, which is arbitrary
// photographic content — no theme token can promise contrast against it. Every
// piece of text therefore carries a `surface` fill of its own, which is a
// pairing `tokens.test.ts` does check.
const styles = StyleSheet.create({
  caption: {
    borderRadius: 8,
    fontSize: 14,
    marginTop: 20,
    maxWidth: '84%',
    paddingHorizontal: 12,
    paddingVertical: 8,
    textAlign: 'center',
  },
  codeLabel: {
    borderRadius: 4,
    fontSize: 11,
    letterSpacing: 1,
    overflow: 'hidden',
    paddingHorizontal: 6,
    paddingVertical: 2,
  },
  codeStrip: {
    alignItems: 'center',
    borderRadius: 6,
    borderStyle: 'dashed',
    borderWidth: 2,
    bottom: codeStrip.bottom,
    height: codeStrip.height,
    justifyContent: 'center',
    position: 'absolute',
    right: codeStrip.right,
    width: codeStrip.width,
  },
  frame: {
    aspectRatio: cardAspectRatio,
    borderRadius: 12,
    borderWidth: 2,
    width: '84%',
  },
  overlay: {
    alignItems: 'center',
    bottom: 0,
    justifyContent: 'center',
    left: 0,
    position: 'absolute',
    right: 0,
    top: 0,
  },
})
