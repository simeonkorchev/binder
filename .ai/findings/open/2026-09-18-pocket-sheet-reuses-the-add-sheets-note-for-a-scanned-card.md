# The pocket sheet tells a scanned card's owner to scan it, by reusing the add sheet's note

- **Category**: i18n
- **Severity**: medium
- **Path**: `apps/mobile/src/features/binder/components/PocketActions.tsx`
- **Found**: 2026-09-18 (T073 clean-code walk)

`PocketActions` shows a note for any card whose set is unknown:

```tsx
{hasKnownSet(slot.setResolution) ? null : (
  <Text …>{t('binder.add.note')}</Text>
)}
```

`binder.add.note` is the add sheet's own sentence: *"A card added by name is
filed with no set. Scanning it records the set it was printed in."* /
*"Карта, добавена по име, се записва без серия. Сканирането ѝ записва серията, в
която е отпечатана."*

`hasKnownSet` is false for **two** resolutions (`lib/setResolution.ts`):
`by_name` — a card added by name, where the sentence is exactly right — and
`unresolved` — a card that **was** scanned and whose set the ladder still could
not determine. For the second one both halves are wrong: nobody added it by
name, and scanning it is what already failed. The advice sends the user back to
do the thing that produced the row they are looking at.

This is the "code matches its own … i18n key" row of
`007-clean-code-checklist.md`: a key named for the add sheet, read in the pocket
sheet, for a case the add sheet cannot produce.

Not fixed on the spot because it changes what a user reads, in both locales.

## Proposed fix

One key per situation, both under the sheet that shows them:

```json
"binder.actions.noSetByName":     "This card was added by name, so its set is unknown. Scanning it records the set it was printed in.",
"binder.actions.noSetUnresolved": "The scan could not tell which set this card is from. Re-scan it in better light, or leave it as it is."
```

and in `PocketActions`, one lookup keyed on the resolution rather than on the
`hasKnownSet` boolean — a map like `resolutionLabelKeys`, so a rung added in Go
stops it compiling:

```ts
const noSetNoteKeys = {
  by_name: 'binder.actions.noSetByName',
  unresolved: 'binder.actions.noSetUnresolved',
} as const satisfies Record<'by_name' | 'unresolved', string>
```

The Bulgarian strings need a translator, not a machine: the rest of `bg.json` is
written, not generated.

## Drafted RED test

```diff
--- a/apps/mobile/src/features/binder/components/PocketActions.test.tsx
+++ b/apps/mobile/src/features/binder/components/PocketActions.test.tsx
+  it('does not tell the owner of a scanned card to scan it', async () => {
+    render(<PocketActions slot={slot({ setResolution: 'unresolved' })} … />)
+
+    expect(
+      screen.queryByText(
+        'A card added by name is filed with no set. Scanning it records the set it was printed in.',
+      ),
+    ).toBeNull()
+    expect(
+      await screen.findByText(
+        'The scan could not tell which set this card is from. Re-scan it in better light, or leave it as it is.',
+      ),
+    ).toBeOnTheScreen()
+  })
```

`PocketActions.test.tsx` already builds slots with a chosen `setResolution`, so
the fixture is there.

## Blast radius

One component, two new keys in both locales, one test. The add sheet keeps
`binder.add.note` unchanged. Text only — no layout change, so T072's screenshots
are not a blocker, though the sheet is worth a look once an emulator exists.
