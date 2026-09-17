module.exports = {
  // RNTL v13+ registers its own matchers (toBeOnTheScreen, toHaveStyle, ...)
  // on import, so there is no setup file to add here.
  preset: 'jest-expo',
  setupFiles: ['<rootDir>/jest.setup.ts'],
  moduleNameMapper: { '^@/(.*)$': '<rootDir>/src/$1' },
  collectCoverageFrom: ['src/**/*.{ts,tsx}'],
}
