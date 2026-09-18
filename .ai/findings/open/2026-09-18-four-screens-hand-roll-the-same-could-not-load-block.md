# Four screens hand-roll the same "could not be loaded / try again" block, in four slightly different sizes

- **Category**: dup
- **Severity**: low
- **Path**: `apps/mobile/src/features/binder/BindersScreen.tsx`, `apps/mobile/src/features/binder/BinderPageScreen.tsx`, `apps/mobile/src/features/market/MarketScreen.tsx`, `apps/mobile/src/features/market/SellerContactScreen.tsx`
- **Found**: 2026-09-18 (T073 clean-code walk)

Every read in the app has three states, and every screen renders the same two of
them: a muted status line while it loads, and an error line with a bordered
"Try again" button. `000-principles.md` §6 says a shared UI pattern becomes a
component. There are four copies:

| Screen | Shape | Retry style |
|--------|-------|-------------|
| `BindersScreen` | inline blocks under the create button | `alignSelf: 'flex-start'`, `marginTop: 12`, label 14 |
| `MarketScreen` | inline blocks under the filters | identical to BindersScreen, byte for byte |
| `SellerContactScreen` | early returns, full-screen | `alignSelf: 'flex-start'`, `marginHorizontal: 16`, label 14 |
| `BinderPageScreen` | a local `Notice` component, centred | `marginTop: 16`, label 15, no `alignSelf` |

`ScanScreen`'s "no camera" notice is a fifth copy of the centred variant, with
`styles.notice` / `styles.noticeText` identical to `BinderPageScreen`'s.
`AddCardSheet` and `CardSearchPicker` each carry a sixth and seventh copy of the
status line, together with the same four-branch decision over the same
`useCardSearch` result (searching → failed → nothing-asked-yet → nothing-found).

Not fixed on the spot because the copies are *near*-identical rather than
identical: unifying them moves pixels — a padded line at the top of a list
becomes a centred full-screen notice, or a 14pt label becomes 15pt. `CLAUDE.md`
requires screenshots of an affected screen in **both** themes before a UI change
is done, and this container has no emulator (T072), so no agent here can prove
the result looks right.

## Proposed fix

One `src/components/` module holding what `BinderPageScreen.Notice` already is —
`{ text, tone: 'muted' | 'error', retry?: { label, onPress } }` — plus a
`placement: 'inline' | 'centred'` so a list screen keeps its top-aligned line and
a whole-screen failure keeps its centred one. Then delete the five local copies
and the duplicated `status` / `error` / `retry` / `retryLabel` style entries. Pick
one font size and one margin set for the retry button as part of the change, and
screenshot the four screens in both themes.

The scan-result status lines (`AddCardSheet`, `CardSearchPicker`) are a smaller
second step: the four-branch decision belongs next to `useCardSearch` as a named
status — `'searching' | 'failed' | 'unasked' | 'nothing-found' | 'found'` — with
each component keeping its own copy and its own row layout.

## Drafted test

No new test is needed to keep behaviour: `App.test.tsx`,
`MarketScreen.test.tsx`, `SellerContactScreen.test.tsx`,
`BinderPageGrid.test.tsx` and `ReviewSheet.test.tsx` already select the retry
buttons and the status sentences by role and accessible name, so a wrong
consolidation fails them. The new test the change *should* add is one per screen
asserting the shared component is what renders the failure — role `button`, name
from the screen's own `retry` key — so a sixth copy cannot be added later without
noticing.

## Blast radius

Five screens/components, one new shared component, and the screen suites above.
Visual: needs T072's emulator.
