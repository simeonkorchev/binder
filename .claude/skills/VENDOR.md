# Forked skills — attribution

These skills were copied from other projects and then **edited to this
repo's rules**. They are ours now: never refreshed from upstream, no
override layer, no text that disagrees with `.claude/rules/`. `tools/memory-check.sh`
fails CI if an upstream-only phrase creeps back in.

## `golang-*` (Go backend)

Forked from [samber/cc-skills-golang](https://github.com/samber/cc-skills-golang)
(MIT) at commit `0944f801aab5169fcd0981d66f8013cba4ed2c16` (2026-09-02):
`golang-refactoring`, `golang-code-style`, `golang-modernize`, `golang-safety`,
`golang-naming`, `golang-error-handling`. Edited for: Ginkgo not `t.Run`,
`%w` at every layer, no `samber/oops`, stdlib `slog` + `pkg/eslog`, consumer-side
interfaces named by role, `Get*` query methods, Go 1.25.6, no sub-agents in
Routines, cross-references pointed at the house rule each one corresponds to.

| Skill | Used for |
|---|---|
| `golang-refactoring` (+ `references/safety-net.md`, `catalog.md`) | the core loop, risk tiers, characterization specs before every change |
| `golang-code-style` | control-flow clarity, function shape, when a comment helps |
| `golang-modernize` | version-driven idiom updates (`slices`, `maps`, `min`/`max`, range-over-int); 1.26+ sections are marked not-yet |
| `golang-safety` | internal-correctness hazards a refactor must not introduce |
| `golang-naming` | what to rename an identifier *to* |
| `golang-error-handling` | wrapping and sentinel technique, aligned with `002-go-conventions.md` §4 |

## `react-native-best-practices` (client-mobile)

Three references forked from
[software-mansion-labs/skills](https://github.com/software-mansion-labs/skills)
(MIT per its README and marketplace manifest) at commit
`fed69f566b78d3289b6f00e4bccc1d4a696ffde3` (2026-09-02): `animations/`
(Reanimated `~4.1`), `gestures/` (Gesture Handler `~2.28`, v2 builder API),
`svg/` (`15.12`). Edited for: no `useMemo` (React Compiler), `StyleSheet.create`
for static styles, no `any`, no upstream-doc fetching, migrations as spec
decisions. Not forked: `multithreading`, `enable-worklets-bundle-mode`,
`audio`, `on-device-ai`, `rich-text`, `jsi` (libraries the app does not use).
