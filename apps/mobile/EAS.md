# Building the app without a local toolchain

`npx expo run:android` and `run:ios` compile natively on your machine, which
means a working Android SDK + NDK + JDK, or Xcode + CocoaPods. That is the thing
most likely to cost you an afternoon, and none of it is required: EAS builds on
Expo's infrastructure and hands you an installable app.

## One-time

```bash
npm i -g eas-cli
eas login
cd apps/mobile && eas init      # creates the project and writes extra.eas.projectId
```

`eas init` is what adds the `projectId` this repo does not have yet. Commit the
change it makes to `app.config.ts`.

## A development build — what you want for the scanner

The scanner needs a **dev client**: VisionCamera frame processors, ML Kit and
Apple sign-in cannot run in Expo Go, and this profile is the replacement for it.

```bash
eas build --profile development --platform android   # APK, installable directly
eas build --profile development --platform ios       # needs a paid Apple account
eas build --profile development-simulator --platform ios  # simulator, no account
```

Android is the easier first target: `buildType: apk` with `distribution:
internal` gives you a file you download and install, with no Play Console.

Then run the JS locally against it:

```bash
npx expo start --dev-client
```

The build is installed once; the JS reloads from your machine every time you
change it. You only rebuild when native dependencies change.

## Where the API address actually comes from

**`EXPO_PUBLIC_*` is inlined when the JS is bundled, not read at runtime** — and
which machine bundles it differs by profile. That distinction is the whole
answer to "why didn't my change take effect".

- **`development`** ships no JS bundle: the dev client loads it from Metro on
  your machine, so the value comes from the environment `expo start` runs in.
  Restarting Metro is enough — `EXPO_PUBLIC_API_URL=… make mobile-start`, or a
  line in the repo-root `.env`, which the Makefile exports. **You do not rebuild
  the APK to change the API address.**
- **`preview` and `production`** embed a bundle, so their `env` block in
  `eas.json` is baked in at build time and changing it needs a new build.

Either way the address must be one the *phone* can reach: your machine's LAN
address, never `localhost`, which on a device means the device itself. An ngrok
tunnel works too and gives you HTTPS — `ngrok http 8080`, then point
`EXPO_PUBLIC_API_URL` at the tunnel. Set a real `SESSION_JWT_SECRET` in `.env`
before you open one: the Makefile's default is committed, so a public URL plus a
published signing secret lets anyone mint a token for any user id, and binderd
wires no inbound rate limiting.

The Google client ids belong in the `env` block once you have them:

```json
"env": {
  "EXPO_PUBLIC_API_URL": "http://192.168.1.23:8080",
  "EXPO_PUBLIC_GOOGLE_CLIENT_ID_ANDROID": "…",
  "EXPO_PUBLIC_GOOGLE_CLIENT_ID_IOS": "…"
}
```

## If you would rather fix the local build

The Gradle failure's real cause is above the part that gets pasted — the tail
only says a task failed. Re-run with:

```bash
cd apps/mobile/android && ./gradlew app:assembleDebug --stacktrace 2>&1 | tail -60
```

and look for the first `FAILURE:` or `error:` line. Common causes on a fresh
machine: no `ANDROID_HOME`/`local.properties`, a JDK that is not 17, or a
missing NDK for the native modules. `android/` is generated, not committed, so
`rm -rf android ios && npx expo prebuild` is always safe to redo.

## What this cannot fix

Nobody has run this app. No screen has been rendered, in either theme. The
first build is also the first look at it — expect visual problems, and treat
`.claude/rules/003-frontend.md`'s theme rules as unverified against pixels.
