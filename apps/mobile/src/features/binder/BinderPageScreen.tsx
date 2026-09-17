import type { NativeStackScreenProps } from '@react-navigation/native-stack'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Pressable, StyleSheet, Text, View } from 'react-native'

import type { RootStackParamList } from '@/navigation/AppNavigator'
import { useTheme } from '@/theme/useTheme'

import { useBinderPage } from './api/useBinderPage'
import { useBinderSlots, type SlotAction } from './api/useBinderSlots'
import { AddCardSheet } from './components/AddCardSheet'
import { BinderPageGrid } from './components/BinderPageGrid'
import { PageIndicator } from './components/PageIndicator'
import { PocketActions } from './components/PocketActions'
import { clampPage, filledPockets, pageOf, pagerLength, type PageShape } from './lib/positions'
import type { SlotBody } from './types'

/** What to say about a write that did not land — which one, never "something went wrong". */
const failureKeys = {
  add: 'binder.page.addError',
  move: 'binder.page.moveError',
  remove: 'binder.page.removeError',
} as const satisfies Record<SlotAction, string>

type BinderPageScreenProps = NativeStackScreenProps<RootStackParamList, 'BinderPage'>

/**
 * One 3x3 page of one binder, the way a physical binder shows one.
 *
 * The screen owns which page is open and which sheet is up; everything else is
 * a hook or a pure function. A write is never applied locally — the server owns
 * the binder's density, so it is the only thing that knows where the cards
 * ended up, and the page is read again rather than guessed at.
 */
const BinderPageScreen = ({ route }: BinderPageScreenProps): React.JSX.Element => {
  const { binderId } = route.params
  const { t } = useTranslation()
  const { colors } = useTheme()
  const [page, setPage] = useState(0)
  const read = useBinderPage(binderId, page)
  const writes = useBinderSlots(binderId)
  const [openSlot, setOpenSlot] = useState<SlotBody | null>(null)
  const [isAdding, setIsAdding] = useState(false)

  if (read.state.status === 'loading') {
    return <Notice text={t('binder.page.loading')} tone="muted" />
  }
  if (read.state.status === 'error') {
    return (
      <Notice
        text={t('binder.page.loadError')}
        tone="error"
        retry={{ label: t('binder.page.retry'), onPress: read.reload }}
      />
    )
  }

  const { pockets, pageCount } = read.state.page
  const shape: PageShape = { page, pageCount, filled: filledPockets(pockets) }

  /** Shows a page, re-reading the current one when that is where the card stayed. */
  const showPage = (next: number): void => {
    if (next === page) read.reload()
    else setPage(next)
  }

  const turnPage = (step: -1 | 1): void => setPage(clampPage(page + step, pageCount))

  const move = async (slot: SlotBody, toPosition: number): Promise<void> => {
    setOpenSlot(null)
    if (await writes.moveCard(slot.id, toPosition)) showPage(pageOf(toPosition))
  }

  const remove = async (slot: SlotBody): Promise<void> => {
    setOpenSlot(null)
    if (!(await writes.removeCard(slot.id))) return
    // The binder closes the gap, so taking the only card off a page takes the
    // page with it and there is nothing left here to look at.
    showPage(shape.filled === 1 ? clampPage(page - 1, pageCount - 1) : page)
  }

  const add = async (cardId: string): Promise<void> => {
    setIsAdding(false)
    const added = await writes.addCard(cardId)
    if (added !== null) showPage(added.page)
  }

  return (
    <View style={[styles.screen, { backgroundColor: colors.background }]}>
      {pageCount === 0 ? (
        <Text style={[styles.emptyBinder, { color: colors.textSecondary }]}>
          {t('binder.page.emptyBinder')}
        </Text>
      ) : null}

      <BinderPageGrid
        pockets={pockets}
        shape={shape}
        onOpenCard={setOpenSlot}
        onAddCard={() => setIsAdding(true)}
        onMove={(slot, toPosition) => void move(slot, toPosition)}
        onTurnPage={turnPage}
      />

      <Text style={[styles.hint, { color: colors.textSecondary }]}>{t('binder.page.dragHint')}</Text>

      {writes.isSaving ? (
        <Text style={[styles.saving, { color: colors.textSecondary }]}>{t('binder.page.saving')}</Text>
      ) : null}

      {writes.failedAction === null ? null : (
        <Pressable
          onPress={writes.dismissFailure}
          accessibilityRole="button"
          accessibilityLabel={t(failureKeys[writes.failedAction])}
          style={[styles.failure, { borderColor: colors.error }]}
        >
          <Text style={[styles.failureText, { color: colors.error }]}>
            {t(failureKeys[writes.failedAction])}
          </Text>
        </Pressable>
      )}

      <PageIndicator page={page} pageCount={pagerLength(pageCount)} onTurnPage={turnPage} />

      {openSlot === null ? null : (
        <PocketActions
          slot={openSlot}
          shape={shape}
          onMove={(toPosition) => void move(openSlot, toPosition)}
          onRemove={() => void remove(openSlot)}
          onClose={() => setOpenSlot(null)}
        />
      )}

      {isAdding ? (
        <AddCardSheet onPick={(card) => void add(card.id)} onClose={() => setIsAdding(false)} />
      ) : null}
    </View>
  )
}

export default BinderPageScreen

interface NoticeProps {
  text: string
  tone: 'muted' | 'error'
  /** Absent while loading: there is nothing to try again yet. */
  retry?: { label: string; onPress: () => void }
}

/** The screen before there is a page to draw: still loading, or unable to. */
const Notice = ({ text, tone, retry }: NoticeProps): React.JSX.Element => {
  const { colors } = useTheme()

  return (
    <View style={[styles.notice, { backgroundColor: colors.background }]}>
      <Text style={[styles.noticeText, { color: tone === 'error' ? colors.error : colors.textSecondary }]}>
        {text}
      </Text>
      {retry === undefined ? null : (
        <Pressable
          onPress={retry.onPress}
          accessibilityRole="button"
          accessibilityLabel={retry.label}
          style={[styles.retry, { borderColor: colors.accent }]}
        >
          <Text style={[styles.retryLabel, { color: colors.accent }]}>{retry.label}</Text>
        </Pressable>
      )}
    </View>
  )
}

const styles = StyleSheet.create({
  emptyBinder: { fontSize: 14, lineHeight: 20, paddingHorizontal: 16, paddingTop: 12 },
  failure: { borderRadius: 8, borderWidth: 1, marginHorizontal: 16, marginTop: 8, paddingHorizontal: 12, paddingVertical: 10 },
  failureText: { fontSize: 13, lineHeight: 18 },
  hint: { fontSize: 12, lineHeight: 16, paddingHorizontal: 16, paddingTop: 10 },
  notice: { alignItems: 'center', flex: 1, justifyContent: 'center', padding: 32 },
  noticeText: { fontSize: 16, lineHeight: 24, textAlign: 'center' },
  retry: { borderRadius: 8, borderWidth: 1, marginTop: 16, paddingHorizontal: 14, paddingVertical: 10 },
  retryLabel: { fontSize: 15, fontWeight: '600' },
  saving: { fontSize: 13, paddingHorizontal: 16, paddingTop: 6 },
  screen: { flex: 1 },
})
