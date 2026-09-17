import { render, screen, userEvent } from '@testing-library/react-native'
import type {
  CameraDevice,
  CameraPermissionRequestResult,
  CameraPermissionStatus,
} from 'react-native-vision-camera'

import '@/i18n/i18n'

import ScanScreen from './ScanScreen'

let mockPermissionStatus: CameraPermissionStatus = 'granted'
let mockRequestResult: CameraPermissionRequestResult = 'granted'
let mockDevice: CameraDevice | undefined

jest.mock('react-native-vision-camera', () => {
  const Camera = (): null => null
  Camera.getCameraPermissionStatus = (): string => mockPermissionStatus
  Camera.requestCameraPermission = (): Promise<string> => Promise.resolve(mockRequestResult)
  return {
    Camera,
    useCameraDevice: () => mockDevice,
    useFrameProcessor: (frameProcessor: unknown) => ({ frameProcessor, type: 'readonly' }),
    runAtTargetFps: (_fps: number, work: () => void) => {
      work()
    },
  }
})

// `useIsFocused` needs a navigator above it, and the screen uses it for one
// thing: keeping the camera off while the user is on another tab.
jest.mock('@react-navigation/native', () => ({ useIsFocused: () => true }))

const AIMING_HELP = 'Fill the frame with the card and keep the printed code inside the marked strip.'
const EMPTY_STRIP = 'Nothing captured yet. Sweep the page and cards land here.'
const NO_CAMERA = 'Binder found no camera on this phone, so there is nothing to scan with.'

const backCamera = (): CameraDevice =>
  ({ id: 'back', position: 'back', name: 'Back Camera' }) as CameraDevice

describe('ScanScreen', () => {
  beforeEach(() => {
    mockPermissionStatus = 'granted'
    mockRequestResult = 'granted'
    mockDevice = backCamera()
  })

  it('shows the aiming help and the empty capture strip on a working camera', async () => {
    await render(<ScanScreen />)

    expect(screen.getByText(AIMING_HELP)).toBeOnTheScreen()
    expect(screen.getByText(EMPTY_STRIP)).toBeOnTheScreen()
  })

  it('says why there is no viewfinder when the phone has no usable camera', async () => {
    mockDevice = undefined

    await render(<ScanScreen />)

    expect(screen.getByText(NO_CAMERA)).toBeOnTheScreen()
    expect(screen.queryByText(AIMING_HELP)).not.toBeOnTheScreen()
  })

  it('opens the viewfinder once the user allows the camera', async () => {
    mockPermissionStatus = 'not-determined'
    const user = userEvent.setup()

    await render(<ScanScreen />)
    expect(screen.queryByText(AIMING_HELP)).not.toBeOnTheScreen()

    await user.press(screen.getByRole('button', { name: 'Allow camera' }))

    expect(await screen.findByText(AIMING_HELP)).toBeOnTheScreen()
  })

  it('sends the user to the settings app when the camera can no longer be asked for', async () => {
    mockPermissionStatus = 'not-determined'
    mockRequestResult = 'denied'
    const user = userEvent.setup()

    await render(<ScanScreen />)
    mockPermissionStatus = 'denied'

    await user.press(screen.getByRole('button', { name: 'Allow camera' }))

    expect(await screen.findByRole('button', { name: 'Open settings' })).toBeOnTheScreen()
    expect(screen.getByText('Camera access is turned off')).toBeOnTheScreen()
  })

  it('offers another ask when the refusal left the dialog available', async () => {
    mockPermissionStatus = 'not-determined'
    mockRequestResult = 'denied'
    const user = userEvent.setup()

    await render(<ScanScreen />)

    await user.press(screen.getByRole('button', { name: 'Allow camera' }))

    expect(await screen.findByText('The camera stayed off')).toBeOnTheScreen()
    expect(screen.getByRole('button', { name: 'Allow camera' })).toBeOnTheScreen()
  })
})
