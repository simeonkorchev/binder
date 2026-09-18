import { useNavigation } from '@react-navigation/native'
import { useTranslation } from 'react-i18next'
import { StyleSheet, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

import { BinderPicker } from './components/BinderPicker'

/**
 * What the collector owns: every binder, and the way to make another.
 *
 * The list itself is `BinderPicker`, which the scanner's commit shows too — the
 * tab's question and a reviewed sweep's question are the same one, and all this
 * screen adds is where a picked binder goes.
 */
const BindersScreen = (): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const navigation = useNavigation()

  return (
    <View style={[styles.screen, { backgroundColor: colors.background }]}>
      <BinderPicker
        rowLabel={(binder) => t('binder.list.openLabel', { name: binder.name })}
        onPick={(binder) => {
          navigation.navigate('BinderPage', { binderId: binder.id, binderName: binder.name })
        }}
      />
    </View>
  )
}

export default BindersScreen

const styles = StyleSheet.create({
  screen: { flex: 1 },
})
