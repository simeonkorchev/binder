import { renderHook, act, waitFor } from '@testing-library/react-native'
import { AppState, Linking, type AppStateStatus, type NativeEventSubscription } from 'react-native'
import type { CameraPermissionRequestResult, CameraPermissionStatus } from 'react-native-vision-camera'

import { useCameraAccess } from './useCameraAccess'

const mockGetStatus = jest.fn<CameraPermissionStatus, []>()
const mockRequest = jest.fn<Promise<CameraPermissionRequestResult>, []>()

jest.mock('react-native-vision-camera', () => ({
  Camera: {
    getCameraPermissionStatus: () => mockGetStatus(),
    requestCameraPermission: () => mockRequest(),
  },
}))

/** Runs the handler the hook registered for the app coming back to the foreground. */
const resume = async (): Promise<void> => {
  const listeners = jest
    .mocked(AppState.addEventListener)
    .mock.calls.filter(([event]) => event === 'change')
  const handler = listeners.at(-1)?.[1]
  if (handler === undefined) throw new Error('the hook registered no AppState listener')
  await act(async () => {
    handler('active' as AppStateStatus)
  })
}

describe('useCameraAccess', () => {
  beforeEach(() => {
    mockGetStatus.mockReset()
    mockRequest.mockReset()
    jest.restoreAllMocks()
    const subscription: NativeEventSubscription = { remove: jest.fn() }
    jest.spyOn(AppState, 'addEventListener').mockReturnValue(subscription)
    jest.spyOn(Linking, 'openSettings').mockResolvedValue()
  })

  it('starts granted when the permission is already granted', async () => {
    mockGetStatus.mockReturnValue('granted')

    const { result } = await renderHook(() => useCameraAccess())

    expect(result.current.access).toBe('granted')
    expect(mockRequest).not.toHaveBeenCalled()
  })

  it('starts at the explainer when the permission has never been asked for', async () => {
    mockGetStatus.mockReturnValue('not-determined')

    const { result } = await renderHook(() => useCameraAccess())

    expect(result.current.access).toBe('undetermined')
  })

  it('starts at the explainer on a status of denied, which Android also reports before the first ask', async () => {
    mockGetStatus.mockReturnValue('denied')

    const { result } = await renderHook(() => useCameraAccess())

    expect(result.current.access).toBe('undetermined')
  })

  it('grants access when the user accepts the dialog', async () => {
    mockGetStatus.mockReturnValue('not-determined')
    mockRequest.mockResolvedValue('granted')

    const { result } = await renderHook(() => useCameraAccess())
    await act(async () => {
      result.current.request()
    })

    await waitFor(() => {
      expect(result.current.access).toBe('granted')
    })
  })

  it('offers another ask when the refusal left the dialog available', async () => {
    mockGetStatus.mockReturnValue('not-determined')
    mockRequest.mockResolvedValue('denied')

    const { result } = await renderHook(() => useCameraAccess())
    await act(async () => {
      result.current.request()
    })

    await waitFor(() => {
      expect(result.current.access).toBe('denied')
    })
  })

  it('sends the user to the settings app when the dialog is spent', async () => {
    mockGetStatus.mockReturnValue('not-determined')
    mockRequest.mockResolvedValue('denied')

    const { result } = await renderHook(() => useCameraAccess())
    mockGetStatus.mockReturnValue('denied')
    await act(async () => {
      result.current.request()
    })

    await waitFor(() => {
      expect(result.current.access).toBe('blocked')
    })
  })

  it('sends the user to the settings app when device policy restricts the camera', async () => {
    mockGetStatus.mockReturnValue('not-determined')
    mockRequest.mockResolvedValue('denied')

    const { result } = await renderHook(() => useCameraAccess())
    mockGetStatus.mockReturnValue('restricted')
    await act(async () => {
      result.current.request()
    })

    await waitFor(() => {
      expect(result.current.access).toBe('blocked')
    })
  })

  it('offers the settings trip when the request itself could not be made', async () => {
    mockGetStatus.mockReturnValue('not-determined')
    mockRequest.mockRejectedValue(new Error('no activity to ask from'))

    const { result } = await renderHook(() => useCameraAccess())
    await act(async () => {
      result.current.request()
    })

    await waitFor(() => {
      expect(result.current.access).toBe('blocked')
    })
  })

  it('picks up a permission granted in the settings app while it was suspended', async () => {
    mockGetStatus.mockReturnValue('not-determined')
    mockRequest.mockResolvedValue('denied')

    const { result } = await renderHook(() => useCameraAccess())
    mockGetStatus.mockReturnValue('denied')
    await act(async () => {
      result.current.request()
    })
    await waitFor(() => {
      expect(result.current.access).toBe('blocked')
    })

    mockGetStatus.mockReturnValue('granted')
    await resume()

    expect(result.current.access).toBe('granted')
  })

  it('leaves a blocked screen blocked when the user came back without granting it', async () => {
    mockGetStatus.mockReturnValue('not-determined')
    mockRequest.mockResolvedValue('denied')

    const { result } = await renderHook(() => useCameraAccess())
    mockGetStatus.mockReturnValue('denied')
    await act(async () => {
      result.current.request()
    })
    await waitFor(() => {
      expect(result.current.access).toBe('blocked')
    })

    await resume()

    expect(result.current.access).toBe('blocked')
  })

  it('opens the app settings page', async () => {
    mockGetStatus.mockReturnValue('denied')

    const { result } = await renderHook(() => useCameraAccess())
    await act(async () => {
      result.current.openSettings()
    })

    expect(Linking.openSettings).toHaveBeenCalled()
  })
})
