import { StatusBar } from 'expo-status-bar'
import { StyleSheet, Text, useColorScheme, View } from 'react-native'

// The palette lives here while App is its only consumer. The second screen that
// needs it is the one that extracts it (000-principles.md section 6).
const palette = {
  light: { background: '#fdfdfd', text: '#14110f' },
  dark: { background: '#14110f', text: '#fdfdfd' },
} as const

export default function App(): React.JSX.Element {
  const scheme = useColorScheme() === 'dark' ? 'dark' : 'light'
  const colors = palette[scheme]

  return (
    <View style={[styles.container, { backgroundColor: colors.background }]}>
      <StatusBar style={scheme === 'dark' ? 'light' : 'dark'} />
      <Text accessibilityRole="header" style={[styles.title, { color: colors.text }]}>
        Binder
      </Text>
    </View>
  )
}

const styles = StyleSheet.create({
  container: { alignItems: 'center', flex: 1, justifyContent: 'center' },
  title: { fontSize: 28, fontWeight: '600' },
})
