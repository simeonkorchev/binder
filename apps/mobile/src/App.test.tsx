import { render, screen } from '@testing-library/react-native'
import { useColorScheme } from 'react-native'

import App from './App'

jest.mock('react-native/Libraries/Utilities/useColorScheme')

const mockUseColorScheme = jest.mocked(useColorScheme)

describe('App', () => {
  beforeEach(() => {
    mockUseColorScheme.mockReturnValue('light')
  })

  it('renders the app name as a header', async () => {
    await render(<App />)

    expect(screen.getByRole('header', { name: 'Binder' })).toBeOnTheScreen()
  })

  it('uses the dark palette when the device is in dark mode', async () => {
    mockUseColorScheme.mockReturnValue('dark')

    await render(<App />)

    expect(screen.getByRole('header', { name: 'Binder' })).toHaveStyle({ color: '#fdfdfd' })
  })
})
