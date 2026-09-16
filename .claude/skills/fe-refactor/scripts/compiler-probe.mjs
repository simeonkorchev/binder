#!/usr/bin/env node
// Proves whether the React Compiler actually compiles the components and hooks
// in a file. Removing manual memoization is only safe when it does.
//
// The compiler's default mode ("infer") compiles a function only if it is a
// component (PascalCase, creates JSX) or a hook (use* name AND calls something
// whose *identifier* starts with `use`). Three ways a function is silently
// left un-memoized:
//   1. BAILOUT - a Rules-of-React violation, or an eslint-disable of a
//      react-hooks rule anywhere in the function. Fix the cause.
//   2. no hook call / no JSX - e.g. a use* hook that only returns closures.
//   3. the only hook call goes through an import alias whose name does not
//      start with `use` (e.g. `useGetRecipes as sdkUseGetRecipes`).
// For 2 and 3, add 'use memo' as the function's first statement (or un-alias).
//
// A function whose first statement is 'use no memo' is a fourth case and not a
// failure: it is a recorded decision to keep the compiler out (the one thing
// the compiler cannot memoize is a value reconciled against the previous
// render's output, which needs a ref read during render). The compiler still
// runs its validation over such a function and logs the violations as
// CompileError events — it emits no CompileSkip — so the directive has to be
// read from the source, and those events are reported under it rather than as
// a bailout.
//
// One more thing it reports, separately from compilation: a RENDER READ. The
// compiler tracks every property path a memoized closure reads as a
// dependency, so a JS-side handler that reads `sharedValue.value` compiles to
// `$[n] !== sharedValue.value` in the cache check — a Reanimated shared-value
// read during render, which strict mode warns about on every re-render.
// Read it with `.get()` there (a method call is opaque to the compiler);
// worklet bodies are not memoized by the compiler and keep `.value`.
//
// Usage (from the app directory, so the app's own compiler version resolves):
//   node ../../.claude/skills/fe-refactor/scripts/compiler-probe.mjs src/path/file.ts [more files]
// Exit 1 when any use*/PascalCase function in a file was not compiled.
import { createRequire } from 'node:module'
import path from 'node:path'
import fs from 'node:fs'

const require = createRequire(path.join(process.cwd(), 'package.json'))
const babel = require('@babel/core')
const compilerPlugin = require('babel-plugin-react-compiler')

const files = process.argv.slice(2)
if (files.length === 0) {
  console.error('usage: compiler-probe.mjs <file.ts|tsx> [...]')
  process.exit(2)
}

// Top-level component/hook declarations with their line numbers.
const declaredFunctions = (source) => {
  const found = []
  // The arrow branch tolerates a return-type annotation between the parameter
  // list and `=>` (`(): Result => {`), which every house hook and component carries.
  const re = /^(?:export\s+)?(?:default\s+)?(?:async\s+)?(?:function\s+([A-Za-z_$][\w$]*)|(?:const|let)\s+([A-Za-z_$][\w$]*)\s*(?::[^=]+)?=\s*(?:React\.)?(?:memo|forwardRef)?\(?\s*(?:async\s*)?(?:function\b|\([^)]*\)\s*(?::[^=]+?)?=>|[\w$]+\s*=>))/gm
  for (const m of source.matchAll(re)) {
    const name = m[1] ?? m[2]
    if (!/^use[A-Z]/.test(name) && !/^[A-Z]/.test(name)) continue
    found.push({ name, line: source.slice(0, m.index).split('\n').length })
  }
  return found
}

const usesAliasedHook = (source) => /import\s*\{[^}]*\buse[A-Z]\w*\s+as\s+(?!use[A-Z])[\w$]+/.test(source)

// Whether the function declared at `line` opens with an opt-out directive: the
// first statement of its body, on the declaration line or the next few.
const optedOut = (source, line) => {
  const head = source.split('\n').slice(line - 1, line + 3).join('\n')
  return /\{\s*(['"])use no (?:memo|forget)\1/.test(head)
}

let failed = false
for (const file of files) {
  const source = fs.readFileSync(file, 'utf8')
  const events = []
  const { code: compiled } = babel.transformFileSync(file, {
    configFile: false,
    babelrc: false,
    filename: file,
    parserOpts: { plugins: ['typescript', 'jsx'] },
    plugins: [[compilerPlugin, { logger: { logEvent: (_f, e) => events.push(e) } }]],
  })
  // `.value` of anything the compiler made a memo dependency — see RENDER READ above.
  const renderReads = [...new Set([...compiled.matchAll(/\$\[\d+\] !== ([\w$]+)\.value\b/g)].map((m) => m[1]))]
  const successLines = new Set(events.filter((e) => e.kind === 'CompileSuccess').map((e) => e.fnLoc?.start?.line))
  const successNames = new Set(events.filter((e) => e.kind === 'CompileSuccess').map((e) => e.fnName))
  const candidates = declaredFunctions(source)
  const optedOutLines = new Set(candidates.filter(({ line }) => optedOut(source, line)).map(({ line }) => line))
  const bailed = events.filter((e) => e.kind !== 'CompileSuccess' && !optedOutLines.has(e.fnLoc?.start?.line))
  console.log(`== ${file}`)
  for (const name of renderReads) {
    console.log(`  RENDER READ  ${name}.value is a memo dependency, so it is read during render; read it with ${name}.get() in the handler`)
    failed = true
  }
  for (const e of bailed) {
    const d = e.detail ?? {}
    const line = d.loc?.start?.line ?? e.fnLoc?.start?.line
    console.log(`  BAILOUT${line ? ` at line ${line}` : ''}: ${d.reason ?? e.reason ?? e.kind}`)
  }
  for (const { name, line } of candidates) {
    if (successNames.has(name) || successLines.has(line)) {
      console.log(`  compiled     ${name} (line ${line})`)
      continue
    }
    if (optedOutLines.has(line)) {
      console.log(`  opted out    ${name} (line ${line}) - 'use no memo' is its first statement; a decision, read its comment`)
      continue
    }
    let why
    if (bailed.length) why = 'see the bailout above; fix its cause (an eslint-disable of a react-hooks rule counts)'
    else if (usesAliasedHook(source)) why = "its hook call goes through an import alias not named use*, so infer mode does not see a hook; add 'use memo' as the first statement or drop the alias"
    else why = "not a component or hook in infer mode (no JSX, no call to a use*-named identifier); add 'use memo' as the first statement"
    console.log(`  NOT COMPILED ${name} (line ${line}) - ${why}`)
    failed = true
  }
  if (candidates.length === 0) console.log('  (no component or hook declarations found)')
}
process.exit(failed ? 1 : 0)
