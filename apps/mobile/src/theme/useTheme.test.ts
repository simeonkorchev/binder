import { renderHook } from '@testing-library/react-native'
import { useColorScheme } from 'react-native'

import { themes } from './tokens'
import { useTheme } from './useTheme'

jest.mock('react-native/Libraries/Utilities/useColorScheme')

const mockUseColorScheme = jest.mocked(useColorScheme)

describe('useTheme', () => {
  it('returns the light palette when the device is light', async () => {
    mockUseColorScheme.mockReturnValue('light')

    const { result } = await renderHook(() => useTheme())

    expect(result.current).toEqual({ name: 'light', colors: themes.light })
  })

  it('returns the dark palette when the device is dark', async () => {
    mockUseColorScheme.mockReturnValue('dark')

    const { result } = await renderHook(() => useTheme())

    expect(result.current).toEqual({ name: 'dark', colors: themes.dark })
  })

  it('falls back to light when the device scheme is unspecified', async () => {
    mockUseColorScheme.mockReturnValue('unspecified')

    const { result } = await renderHook(() => useTheme())

    expect(result.current.name).toBe('light')
  })
})
