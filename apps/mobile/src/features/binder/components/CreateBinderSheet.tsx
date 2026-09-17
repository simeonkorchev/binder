import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Modal, Pressable, StyleSheet, Text, TextInput, View } from 'react-native'

import { useTheme } from '@/theme/useTheme'

interface CreateBinderSheetProps {
  isCreating: boolean
  /** True after a create that did not land, so the sheet stays open and says why. */
  hasFailed: boolean
  onCreate: (name: string) => void
  onClose: () => void
}

/** Naming a new binder. The name is the only thing a binder needs to exist. */
export const CreateBinderSheet = ({
  isCreating,
  hasFailed,
  onCreate,
  onClose,
}: CreateBinderSheetProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const [name, setName] = useState('')
  const isNamed = name.trim() !== ''

  return (
    <Modal visible transparent animationType="slide" onRequestClose={onClose}>
      <View style={styles.backdrop}>
        <View style={[styles.sheet, { backgroundColor: colors.surface, borderColor: colors.border }]}>
          <Text style={[styles.title, { color: colors.textPrimary }]}>
            {t('binder.list.createTitle')}
          </Text>

          <TextInput
            value={name}
            onChangeText={setName}
            onSubmitEditing={() => {
              if (isNamed) onCreate(name.trim())
            }}
            placeholder={t('binder.list.namePlaceholder')}
            placeholderTextColor={colors.textSecondary}
            accessibilityLabel={t('binder.list.nameLabel')}
            autoFocus
            returnKeyType="done"
            style={[
              styles.input,
              { backgroundColor: colors.surfaceMuted, borderColor: colors.border, color: colors.textPrimary },
            ]}
          />

          {hasFailed ? (
            <Text style={[styles.error, { color: colors.error }]}>{t('binder.list.createError')}</Text>
          ) : null}

          <View style={styles.buttons}>
            <Pressable
              onPress={onClose}
              accessibilityRole="button"
              accessibilityLabel={t('binder.list.cancel')}
              style={[styles.cancel, { borderColor: colors.border }]}
            >
              <Text style={[styles.cancelLabel, { color: colors.textPrimary }]}>
                {t('binder.list.cancel')}
              </Text>
            </Pressable>
            <Pressable
              onPress={() => onCreate(name.trim())}
              disabled={!isNamed || isCreating}
              accessibilityRole="button"
              accessibilityLabel={t('binder.list.save')}
              accessibilityState={{ disabled: !isNamed || isCreating }}
              style={[
                styles.save,
                { backgroundColor: colors.accent },
                !isNamed || isCreating ? styles.unavailable : null,
              ]}
            >
              <Text style={[styles.saveLabel, { color: colors.onAccent }]}>
                {t('binder.list.save')}
              </Text>
            </Pressable>
          </View>
        </View>
      </View>
    </Modal>
  )
}

const styles = StyleSheet.create({
  backdrop: { flex: 1, justifyContent: 'flex-end' },
  buttons: { flexDirection: 'row', gap: 12, justifyContent: 'flex-end', marginTop: 20 },
  cancel: { borderRadius: 8, borderWidth: 1, paddingHorizontal: 16, paddingVertical: 12 },
  cancelLabel: { fontSize: 15, fontWeight: '600' },
  error: { fontSize: 13, lineHeight: 18, marginTop: 10 },
  input: { borderRadius: 8, borderWidth: 1, fontSize: 16, marginTop: 16, paddingHorizontal: 12, paddingVertical: 10 },
  save: { borderRadius: 8, paddingHorizontal: 16, paddingVertical: 12 },
  saveLabel: { fontSize: 15, fontWeight: '700' },
  sheet: { borderTopLeftRadius: 16, borderTopRightRadius: 16, borderWidth: StyleSheet.hairlineWidth, padding: 20 },
  title: { fontSize: 20, fontWeight: '700' },
  unavailable: { opacity: 0.4 },
})
