import { createBottomTabNavigator } from '@react-navigation/bottom-tabs'
import { NavigationContainer, type NavigatorScreenParams } from '@react-navigation/native'
import { createNativeStackNavigator } from '@react-navigation/native-stack'
import { useTranslation } from 'react-i18next'

import ScanScreen from '@/features/scan/ScanScreen'
import { useTheme } from '@/theme/useTheme'

import { navigationTheme } from './navigationTheme'
import { PlaceholderScreen } from './PlaceholderScreen'

/** The three places the app can be in — scan, what you own, what is for sale. */
export type RootTabParamList = {
  Scan: undefined
  Binders: undefined
  Market: undefined
}

/** Everything pushed over the tabs. A route's params are its whole input. */
export type RootStackParamList = {
  Tabs: NavigatorScreenParams<RootTabParamList> | undefined
  BinderPage: { binderId: string }
  SellerContact: { sellerId: string }
}

declare global {
  // Makes `useNavigation()` typed everywhere without a screen importing the
  // param list — the idiom React Navigation documents for a single root. The
  // param list has to stay a type alias rather than move in here: React
  // Navigation's `ParamListBase` is an index-signature type, and an interface
  // has no implicit index signature to satisfy it.
  namespace ReactNavigation {
    // eslint-disable-next-line @typescript-eslint/no-empty-object-type -- an empty body is exactly the intent: this merges the routes into the library's global registry, and re-listing them here would be a second source of truth
    interface RootParamList extends RootStackParamList {}
  }
}

const Tab = createBottomTabNavigator<RootTabParamList>()
const Stack = createNativeStackNavigator<RootStackParamList>()

// Titles are read inside the navigators rather than hoisted to a constant:
// `t` must be called during render for a language change to reach the headers.
const TabsNavigator = (): React.JSX.Element => {
  const { t } = useTranslation()

  return (
    <Tab.Navigator>
      <Tab.Screen name="Scan" component={ScanScreen} options={{ title: t('nav.scan') }} />
      <Tab.Screen
        name="Binders"
        component={PlaceholderScreen}
        options={{ title: t('nav.binders') }}
      />
      <Tab.Screen
        name="Market"
        component={PlaceholderScreen}
        options={{ title: t('nav.market') }}
      />
    </Tab.Navigator>
  )
}

export const AppNavigator = (): React.JSX.Element => {
  const { t } = useTranslation()
  const theme = useTheme()

  return (
    <NavigationContainer theme={navigationTheme(theme)}>
      <Stack.Navigator>
        {/* The tabs draw their own headers; a second one here would stack. */}
        <Stack.Screen name="Tabs" component={TabsNavigator} options={{ headerShown: false }} />
        <Stack.Screen
          name="BinderPage"
          component={PlaceholderScreen}
          options={{ title: t('nav.binderPage') }}
        />
        <Stack.Screen
          name="SellerContact"
          component={PlaceholderScreen}
          options={{ title: t('nav.sellerContact') }}
        />
      </Stack.Navigator>
    </NavigationContainer>
  )
}
