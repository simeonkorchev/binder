import { AndroidHaptics, ImpactFeedbackStyle, impactAsync, performAndroidHapticsAsync } from 'expo-haptics'
import { Platform } from 'react-native'

/**
 * The tick the phone gives when a card lands in the session.
 *
 * It is the only confirmation a sweep can give without asking the user to look
 * away from the binder page, which is the whole point of a scanner that never
 * needs a tap.
 *
 * Android takes a different call for a reason that reaches the manifest:
 * `impactAsync` drives Android's `Vibrator`, which needs the `VIBRATE`
 * permission, while `performAndroidHapticsAsync` uses the view's haptic
 * feedback and needs none. `app.config.ts` says the camera is the only
 * permission this app asks for, and this keeps that true.
 *
 * A failure is swallowed: a device with no haptic engine, or one that refuses,
 * must not take a captured card down with it.
 */
export const captureTick = async (): Promise<void> => {
  try {
    await (Platform.OS === 'android'
      ? performAndroidHapticsAsync(AndroidHaptics.Confirm)
      : impactAsync(ImpactFeedbackStyle.Light))
  } catch {
    return
  }
}
