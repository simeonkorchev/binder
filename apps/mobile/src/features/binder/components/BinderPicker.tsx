import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { FlatList, Pressable, StyleSheet, Text, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

import { useBinders } from '../api/useBinders'
import type { Binder } from '../types'

import { CreateBinderSheet } from './CreateBinderSheet'

interface BinderPickerProps {
  /**
   * One binder's accessible name, which is what pressing it *does* — and only
   * the caller knows that: the binders tab opens it, the scanner's commit files
   * a sweep in it. A row that announced "open" and then filed a sweep would be a
   * label that does not match its control.
   */
  rowLabel: (binder: Binder) => string
  /** The binder the collector picked. */
  onPick: (binder: Binder) => void
}

/**
 * Every binder the collector owns, one press that picks one, and the way to make
 * another.
 *
 * It is the binders tab's list *and* the target picker a reviewed sweep needs,
 * because both ask the same question — which binder? — and only the answer's use
 * differs. The three states stay apart: a list that could not be read says so
 * and offers the read again, rather than looking like a collector who has not
 * started yet.
 *
 * **Creating a binder picks it.** A collector with no binder at all reaches the
 * scanner's commit with nowhere to put the cards, and sending them to another tab
 * to make one would be a reviewed sweep abandoned halfway.
 */
export const BinderPicker = ({ rowLabel, onPick }: BinderPickerProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const binders = useBinders()
  const [isNaming, setIsNaming] = useState(false)

  const create = async (name: string): Promise<void> => {
    const created = await binders.createBinder(name)
    // Confirmed, not assumed: the sheet stays open on a failure and says so.
    if (created === null) return
    setIsNaming(false)
    onPick(created)
  }

  return (
    <View style={styles.picker}>
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
              onPress={() => onPick(item)}
              accessibilityRole="button"
              accessibilityLabel={rowLabel(item)}
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

const styles = StyleSheet.create({
  create: { alignSelf: 'flex-start', borderRadius: 8, marginBottom: 4, marginHorizontal: 16, marginTop: 16, paddingHorizontal: 16, paddingVertical: 12 },
  createLabel: { fontSize: 15, fontWeight: '700' },
  error: { fontSize: 14, lineHeight: 20 },
  failure: { padding: 16 },
  list: { padding: 16 },
  picker: { flex: 1 },
  retry: { alignSelf: 'flex-start', borderRadius: 8, borderWidth: 1, marginTop: 12, paddingHorizontal: 14, paddingVertical: 10 },
  retryLabel: { fontSize: 14, fontWeight: '600' },
  row: { borderRadius: 10, borderWidth: 1, marginBottom: 10, padding: 16 },
  rowName: { fontSize: 16, fontWeight: '600' },
  status: { fontSize: 14, lineHeight: 20, padding: 16 },
})
