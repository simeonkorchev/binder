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

const mockNavigate = jest.fn<void, [string, object]>()

// Both hooks need a navigator above them, and the screen uses each for one
// thing: keeping the camera off while the user is on another tab, and opening the
// binder a filed sweep landed in.
jest.mock('@react-navigation/native', () => ({
  useIsFocused: () => true,
  useNavigation: () => ({ navigate: mockNavigate }),
}))

const AIMING_HELP = 'Fill the frame with the card and keep the printed code inside the marked strip.'
const EMPTY_STRIP = 'Nothing captured yet. Sweep the page and cards land here.'
const NO_CAMERA = 'Binder found no camera on this phone, so there is nothing to scan with.'

const theBinder = {
  id: 'binder-1',
  name: 'Duplicates',
  createdAt: '2026-09-18T10:00:00Z',
  updatedAt: '2026-09-18T10:00:00Z',
}

const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

/** The collector's binders, and the batch that files a sweep in one of them. */
const binderServer = (input: RequestInfo | URL): Promise<Response> => {
  if (String(input).endsWith('/slots/batch')) {
    return Promise.resolve(new Response(JSON.stringify({ slots: [] }), { status: 201 }))
  }
  return Promise.resolve(new Response(JSON.stringify({ binders: [theBinder] }), { status: 200 }))
}

// The screen forwards the device to `<Camera>` and reads nothing off it, so
// the fixture carries only what identifies it.
const backCamera = (): CameraDevice =>
  ({ id: 'back', position: 'back', name: 'Back Camera' }) as CameraDevice

describe('ScanScreen', () => {
  beforeEach(() => {
    mockPermissionStatus = 'granted'
    mockRequestResult = 'granted'
    mockDevice = backCamera()
    mockNavigate.mockReset()
    mockFetch.mockReset()
    mockFetch.mockImplementation(binderServer)
    globalThis.fetch = mockFetch
    process.env.EXPO_PUBLIC_API_URL = 'https://api.binder.test'
  })

  afterEach(() => {
    delete process.env.EXPO_PUBLIC_API_URL
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

  // US3 is satisfied in the review sheet, not in the sweep, so the sweep has to
  // be able to reach it — and at zero flagged too, which is how a user confirms
  // the page they just swept needs nothing fixed.
  it('reaches the review sheet from the sweep, with nothing flagged yet', async () => {
    const user = userEvent.setup()

    await render(<ScanScreen />)

    await user.press(screen.getByRole('button', { name: 'Review flagged cards: 0 still to check' }))

    expect(
      await screen.findByText(
        'Nothing to check. Every card this sweep captured was matched to a set.',
      ),
    ).toBeOnTheScreen()
  })

  // The end of the product's loop: a swept card is not in a collection until it
  // is in a binder, and the collector has to be able to see that it is.
  it('lands the user in the binder a filed sweep went into', async () => {
    const user = userEvent.setup()

    await render(<ScanScreen />)

    await user.press(screen.getByRole('button', { name: 'Review flagged cards: 0 still to check' }))
    await user.press(await screen.findByRole('button', { name: /^File this sweep in a binder/ }))
    await user.press(await screen.findByRole('button', { name: 'File this sweep in Duplicates' }))

    expect(mockNavigate).toHaveBeenCalledWith('BinderPage', {
      binderId: 'binder-1',
      binderName: 'Duplicates',
    })
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
