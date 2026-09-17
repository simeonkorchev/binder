import { useNavigation } from '@react-navigation/native'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { FlatList, Pressable, StyleSheet, Text, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

import { useBinders } from './api/useBinders'
import { CreateBinderSheet } from './components/CreateBinderSheet'
import type { Binder } from './types'

/**
 * What the collector owns: every binder, and the way to make another.
 *
 * The screen keeps the three states apart — a list that could not be read says
 * so and offers the read again, rather than showing the same empty state as a
 * collector who has not started yet.
 */
const BindersScreen = (): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const navigation = useNavigation()
  const binders = useBinders()
  const [isNaming, setIsNaming] = useState(false)

  const open = (binder: Binder): void => {
    navigation.navigate('BinderPage', { binderId: binder.id, binderName: binder.name })
  }

  const create = async (name: string): Promise<void> => {
    const created = await binders.createBinder(name)
    // Confirmed, not assumed: the sheet stays open on a failure and says so.
    if (created === null) return
    setIsNaming(false)
    open(created)
  }

  return (
    <View style={[styles.screen, { backgroundColor: colors.background }]}>
      <Pressable
        onPress={() => setIsNaming(true)}
        accessibilityRole="button"
        accessibilityLabel={t('binder.list.create')}
        style={[styles.create, { backgroundColor: colors.accent }]}
      >
        <Text style={[styles.createLabel, { color: colors.onAccent }]}>
          {t('binder.list.create')}
        </Text>
      </Pressable>

      {binders.state.status === 'loading' ? (
        <Text style={[styles.status, { color: colors.textSecondary }]}>
          {t('binder.list.loading')}
        </Text>
      ) : null}

      {binders.state.status === 'error' ? (
        <View style={styles.failure}>
          <Text style={[styles.error, { color: colors.error }]}>{t('binder.list.loadError')}</Text>
          <Pressable
            onPress={binders.reload}
            accessibilityRole="button"
            accessibilityLabel={t('binder.list.retry')}
            style={[styles.retry, { borderColor: colors.accent }]}
          >
            <Text style={[styles.retryLabel, { color: colors.accent }]}>
              {t('binder.list.retry')}
            </Text>
          </Pressable>
        </View>
      ) : null}

      {binders.state.status === 'ready' ? (
        <FlatList
          data={binders.state.binders}
          keyExtractor={(binder) => binder.id}
          contentContainerStyle={styles.list}
          ListEmptyComponent={
            <Text style={[styles.status, { color: colors.textSecondary }]}>
              {t('binder.list.empty')}
            </Text>
          }
          renderItem={({ item }) => (
            <Pressable
              onPress={() => open(item)}
              accessibilityRole="button"
              accessibilityLabel={t('binder.list.openLabel', { name: item.name })}
              style={[styles.row, { backgroundColor: colors.surface, borderColor: colors.border }]}
            >
              <Text style={[styles.rowName, { color: colors.textPrimary }]} numberOfLines={1}>
                {item.name}
              </Text>
            </Pressable>
          )}
        />
      ) : null}

      {isNaming ? (
        <CreateBinderSheet
          isCreating={binders.isCreating}
          hasFailed={binders.createFailed}
          onCreate={(name) => void create(name)}
          onClose={() => setIsNaming(false)}
        />
      ) : null}
    </View>
  )
}

export default BindersScreen

const styles = StyleSheet.create({
  create: { alignSelf: 'flex-start', borderRadius: 8, marginBottom: 4, marginHorizontal: 16, marginTop: 16, paddingHorizontal: 16, paddingVertical: 12 },
  createLabel: { fontSize: 15, fontWeight: '700' },
  error: { fontSize: 14, lineHeight: 20 },
  failure: { padding: 16 },
  list: { padding: 16 },
  retry: { alignSelf: 'flex-start', borderRadius: 8, borderWidth: 1, marginTop: 12, paddingHorizontal: 14, paddingVertical: 10 },
  retryLabel: { fontSize: 14, fontWeight: '600' },
  row: { borderRadius: 10, borderWidth: 1, marginBottom: 10, padding: 16 },
  rowName: { fontSize: 16, fontWeight: '600' },
  screen: { flex: 1 },
  status: { fontSize: 14, lineHeight: 20, padding: 16 },
})
