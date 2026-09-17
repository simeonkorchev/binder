module.exports = function babelConfig(api) {
  api.cache(true)
  return {
    presets: ['babel-preset-expo'],
    // A frame processor runs on VisionCamera's own JS runtime, not the app's.
    // This plugin is what compiles a function carrying the `'worklet'`
    // directive for that runtime; without it the scanner's frame processor
    // throws on the first frame.
    plugins: ['react-native-worklets-core/plugin'],
  }
}
