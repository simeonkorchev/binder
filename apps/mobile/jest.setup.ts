// `SafeAreaProvider` renders nothing until the native view reports its insets,
// so without this every test of a screen under it sees an empty tree. The
// library ships this mock for exactly that reason; it supplies a 320×640 frame
// with zero insets.
jest.mock('react-native-safe-area-context', () =>
  jest.requireActual<{ default: unknown }>('react-native-safe-area-context/jest/mock').default,
)
