const expoConfig = require('eslint-config-expo/flat')

module.exports = [
  ...expoConfig,
  {
    ignores: ['node_modules/**', '.expo/**', 'coverage/**'],
  },
  {
    // The build-tool configs run in Node, not in the app bundle, so they see
    // CommonJS globals the React Native environment does not have.
    files: ['*.js', '*.cjs'],
    languageOptions: {
      sourceType: 'commonjs',
      globals: { __dirname: 'readonly', module: 'writable', require: 'readonly' },
    },
  },
]
