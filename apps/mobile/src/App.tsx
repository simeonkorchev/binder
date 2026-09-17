import { StatusBar } from 'expo-status-bar'
import { SafeAreaProvider } from 'react-native-safe-area-context'

import '@/i18n/i18n'
import { AppNavigator } from '@/navigation/AppNavigator'

export default function App(): React.JSX.Element {
  return (
    <SafeAreaProvider>
      {/* `auto` follows the device appearance, the same source `useTheme` reads. */}
      <StatusBar style="auto" />
      <AppNavigator />
    </SafeAreaProvider>
  )
}
