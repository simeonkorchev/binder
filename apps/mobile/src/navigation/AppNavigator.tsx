import { createBottomTabNavigator } from '@react-navigation/bottom-tabs'
import { NavigationContainer, type NavigatorScreenParams } from '@react-navigation/native'
import { createNativeStackNavigator } from '@react-navigation/native-stack'
import { useTranslation } from 'react-i18next'
import { StyleSheet, Text, View } from 'react-native'

import SignInScreen from '@/features/auth/SignInScreen'
import { SignOutButton } from '@/features/auth/components/SignOutButton'
import { useSession } from '@/features/auth/useSession'
import BinderPageScreen from '@/features/binder/BinderPageScreen'
import BindersScreen from '@/features/binder/BindersScreen'
import MarketScreen from '@/features/market/MarketScreen'
import SellerContactScreen from '@/features/market/SellerContactScreen'
import ScanScreen from '@/features/scan/ScanScreen'
import { useTheme } from '@/theme/useTheme'

import { navigationTheme } from './navigationTheme'

/** The three places a signed-in app can be in — scan, what you own, what is for sale. */
export type RootTabParamList = {
  Scan: undefined
  Binders: undefined
  Market: undefined
}

/**
 * Every route in the app, signed in or out. A route's params are its whole input.
 *
 * `BrowseMarket` is the market as a signed-out visitor reaches it, pushed from
 * the sign-in screen rather than sitting in a tab bar that does not exist yet.
 * It is the same screen as the `Market` tab; the two names exist because the two
 * ways in do.
 */
export type RootStackParamList = {
  SignIn: undefined
  BrowseMarket: undefined
  Tabs: NavigatorScreenParams<RootTabParamList> | undefined
  // The name travels with the id because nothing on the page can look it up:
  // `GET /binders/{id}` answers with a page of slots, and the list is the only
  // place a binder's name is ever sent.
  BinderPage: { binderId: string; binderName: string }
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
    // The binders are where a signed-in collector lands: the scanner is the
    // product's differentiator, but it has nowhere to put a card until a binder
    // exists (T083).
    <Tab.Navigator initialRouteName="Binders">
      <Tab.Screen name="Scan" component={ScanScreen} options={{ title: t('nav.scan') }} />
      <Tab.Screen
        name="Binders"
        component={BindersScreen}
        options={{
          title: t('nav.binders'),
          // The one tab that is entirely this account's own data, which makes
          // its header the place to leave the account from.
          headerRight: () => <SignOutButton />,
        }}
      />
      <Tab.Screen name="Market" component={MarketScreen} options={{ title: t('nav.market') }} />
    </Tab.Navigator>
  )
}

/**
 * What a signed-in collector can reach: their binders, the scanner, the market,
 * and a seller's contact details.
 */
const SignedIn = (): React.JSX.Element => {
  const { t } = useTranslation()

  return (
    <Stack.Navigator>
      {/* The tabs draw their own headers; a second one here would stack. */}
      <Stack.Screen name="Tabs" component={TabsNavigator} options={{ headerShown: false }} />
      <Stack.Screen
        name="BinderPage"
        component={BinderPageScreen}
        // The header is the one place the binder's name fits, and the route
        // carries it for exactly that.
        options={({ route }) => ({ title: route.params.binderName })}
      />
      <Stack.Screen
        name="SellerContact"
        component={SellerContactScreen}
        options={{ title: t('nav.sellerContact') }}
      />
    </Stack.Navigator>
  )
}

/**
 * What a visitor can reach: the way in, and the market.
 *
 * Browse is public by design — `GET /listings` is the one endpoint that takes no
 * actor — so it stays reachable with no account. A seller's details do need one,
 * which is why `SellerContact` is here too: it is reachable from a listing, and
 * it says that signing in is what it needs rather than failing as a load error.
 */
const SignedOut = (): React.JSX.Element => {
  const { t } = useTranslation()

  return (
    <Stack.Navigator>
      <Stack.Screen name="SignIn" component={SignInScreen} options={{ headerShown: false }} />
      <Stack.Screen
        name="BrowseMarket"
        component={MarketScreen}
        options={{ title: t('nav.market') }}
      />
      <Stack.Screen
        name="SellerContact"
        component={SellerContactScreen}
        options={{ title: t('nav.sellerContact') }}
      />
    </Stack.Navigator>
  )
}

export const AppNavigator = (): React.JSX.Element => {
  const theme = useTheme()
  const session = useSession()

  // The keychain read at launch. Showing the sign-in screen for the length of it
  // would flash it at somebody who is signed in, and showing the tabs would
  // flash a binder list that cannot load yet.
  if (session.status === 'restoring') return <Opening />

  return (
    <NavigationContainer theme={navigationTheme(theme)}>
      {/* Swapping the whole stack is what takes a user back to sign-in when the
          session goes: a 401 anywhere clears it, this re-renders, and the
          screens that needed a token leave with the stack that held them.
          Nothing has to pop a route (T082, T083). */}
      {session.status === 'signed-in' ? <SignedIn /> : <SignedOut />}
    </NavigationContainer>
  )
}

/** The launch screen, for as long as one keychain read takes. */
const Opening = (): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()

  return (
    <View style={[styles.opening, { backgroundColor: colors.background }]}>
      <Text style={[styles.openingLabel, { color: colors.textSecondary }]}>
        {t('auth.opening')}
      </Text>
    </View>
  )
}

const styles = StyleSheet.create({
  opening: { alignItems: 'center', flex: 1, justifyContent: 'center' },
  openingLabel: { fontSize: 15 },
})
