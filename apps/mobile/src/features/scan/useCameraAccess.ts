import { useEffect, useState } from 'react'
import { AppState, Linking } from 'react-native'
import { Camera, type CameraPermissionStatus } from 'react-native-vision-camera'

/**
 * Where the user stands with the camera, as far as the scanner is concerned.
 *
 * `denied` and `blocked` are both refusals, and they are separate states
 * because they have different ways out: a denied permission can be asked for
 * again, a blocked one can only be changed in the system settings. Offering the
 * wrong one is how a scanner dead-ends — a "try again" button that the OS
 * silently ignores, or a trip to Settings the user never needed to make.
 */
export type CameraAccess = 'undetermined' | 'denied' | 'blocked' | 'granted'

/**
 * What a refusal means *after we have asked in this session*.
 *
 * VisionCamera reports `not-determined` whenever the OS is still willing to
 * show the dialog — on Android it maps a denied permission back to
 * `not-determined` while `shouldShowRequestPermissionRationale` holds — so a
 * refusal that leaves the status at `not-determined` is one the user can undo
 * from this screen. Anything else (`denied`, or `restricted` by device policy)
 * means the dialog is spent and only the settings app can change the answer.
 */
const accessAfterRefusal = (status: CameraPermissionStatus): CameraAccess =>
  status === 'not-determined' ? 'denied' : 'blocked'

/**
 * What the status means *before* we have asked.
 *
 * Only a grant is trusted here. Android returns `denied` both for a permission
 * that was permanently blocked and for one that has never been requested —
 * `shouldShowRequestPermissionRationale` is false until the first ask — so
 * reading `denied` as "blocked" would send a first-run user to the settings
 * app for a dialog they were never shown. Asking is cheap and its answer is
 * unambiguous, so every ungranted status starts at the explainer.
 */
const accessBeforeRequest = (status: CameraPermissionStatus): CameraAccess =>
  status === 'granted' ? 'granted' : 'undetermined'

export interface CameraAccessState {
  access: CameraAccess
  /** Asks the OS for the camera and records what it answered. */
  request: () => void
  /** Opens this app's page in the system settings — the only way out of `blocked`. */
  openSettings: () => void
}

/**
 * Owns the camera permission for the scan screen: the state, the request, and
 * the trip to the settings app.
 *
 * The permission can change while the app is suspended — that is the whole
 * point of the settings trip — so the status is re-read every time the app
 * comes back to the foreground. Only a grant is acted on there: a status that
 * is still not granted leaves the screen where it was, because whether the OS
 * will show the dialog again is something we learned from the *request*, not
 * from the status.
 */
export const useCameraAccess = (): CameraAccessState => {
  const [access, setAccess] = useState<CameraAccess>(() =>
    accessBeforeRequest(Camera.getCameraPermissionStatus()),
  )

  useEffect(() => {
    const subscription = AppState.addEventListener('change', (nextState) => {
      if (nextState !== 'active') return
      if (Camera.getCameraPermissionStatus() === 'granted') setAccess('granted')
    })
    return () => {
      subscription.remove()
    }
  }, [])

  const request = (): void => {
    void (async (): Promise<void> => {
      try {
        const result = await Camera.requestCameraPermission()
        setAccess(
          result === 'granted' ? 'granted' : accessAfterRefusal(Camera.getCameraPermissionStatus()),
        )
      } catch {
        // `requestCameraPermission` throws when it could not even ask. There is
        // nothing left for this screen to try, so it offers the one destination
        // that can still change the answer rather than leaving a button that
        // does nothing.
        setAccess('blocked')
      }
    })()
  }

  return {
    access,
    request,
    openSettings: (): void => {
      void Linking.openSettings()
    },
  }
}
