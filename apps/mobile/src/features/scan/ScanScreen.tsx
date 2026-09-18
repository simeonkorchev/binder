import { useIsFocused, useNavigation } from '@react-navigation/native'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { StyleSheet, Text, View } from 'react-native'
import { Camera, useCameraDevice } from 'react-native-vision-camera'

import { useTheme } from '@/theme/useTheme'

import { CameraAccessNotice } from './components/CameraAccessNotice'
import { CapturedStrip } from './components/CapturedStrip'
import { ReviewSheet } from './components/ReviewSheet'
import { ScanGuideFrame } from './components/ScanGuideFrame'
import { useCameraAccess } from './useCameraAccess'
import { useScanSession } from './useScanSession'

/**
 * The sweep: point the phone at a binder page and cards land in the session
 * without a tap.
 *
 * Everything that decides anything lives in a hook — `useCameraAccess` owns
 * the permission, `useScanSession` owns the loop from frame to captured card.
 * What is left here is which of the three things the tab can show: a way to
 * get the camera back, a reason there is no camera at all, or the viewfinder.
 *
 * The one thing it decides is where a filed sweep leaves the user: in the binder
 * that now holds it. A sweep that ended back at the viewfinder would leave the
 * collector with no sign the cards had landed anywhere.
 */
const ScanScreen = (): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const { access, request, openSettings } = useCameraAccess()
  const device = useCameraDevice('back')
  const session = useScanSession()
  const navigation = useNavigation()
  // The tab keeps its screens mounted, so without this the camera would keep
  // reading frames — and draining the battery — while the user is in a binder.
  const isFocused = useIsFocused()
  // Whether the sheet is open is where the user is, not what the session knows.
  const [isReviewOpen, setIsReviewOpen] = useState(false)

  if (access !== 'granted') {
    return <CameraAccessNotice access={access} onRequest={request} onOpenSettings={openSettings} />
  }

  if (device === undefined) {
    return (
      <View style={[styles.notice, { backgroundColor: colors.background }]}>
        <Text style={[styles.noticeText, { color: colors.textSecondary }]}>{t('scan.noCamera')}</Text>
      </View>
    )
  }

  return (
    <View style={[styles.screen, { backgroundColor: colors.background }]}>
      <View style={styles.viewfinder}>
        <Camera
          style={styles.camera}
          device={device}
          isActive={isFocused}
          frameProcessor={session.frameProcessor}
        />
        <ScanGuideFrame />
      </View>
      <CapturedStrip
        cards={session.captured}
        isOffline={session.isOffline}
        onRetry={session.retryPending}
        undecidedCount={session.review.undecidedCount}
        onReview={() => setIsReviewOpen(true)}
      />
      <ReviewSheet
        visible={isReviewOpen}
        review={session.review}
        commit={session.commit}
        onFiled={(binder) => {
          // The sheet closes first, and not for tidiness: a `Modal` is a native
          // overlay above the whole app, so the binder would open underneath it.
          setIsReviewOpen(false)
          navigation.navigate('BinderPage', { binderId: binder.id, binderName: binder.name })
        }}
        onClose={() => setIsReviewOpen(false)}
      />
    </View>
  )
}

export default ScanScreen

const styles = StyleSheet.create({
  camera: { bottom: 0, left: 0, position: 'absolute', right: 0, top: 0 },
  notice: { alignItems: 'center', flex: 1, justifyContent: 'center', padding: 32 },
  noticeText: { fontSize: 16, lineHeight: 24, textAlign: 'center' },
  screen: { flex: 1 },
  viewfinder: { flex: 1, overflow: 'hidden' },
})
