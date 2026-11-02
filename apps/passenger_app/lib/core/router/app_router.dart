import 'package:go_router/go_router.dart';

import '../../features/auth/data/demo_auth_session.dart';
import '../../features/auth/presentation/phone_sign_in_screen.dart';
import '../../features/auth/presentation/verify_phone_screen.dart';
import '../../features/mobility/presentation/mobility_app_screen.dart';
import '../../features/onboarding/presentation/onboarding_screen.dart';

GoRouter buildRouter(DemoAuthSession session) => GoRouter(
      initialLocation: '/',
      refreshListenable: session,
      redirect: (_, state) {
        final location = state.matchedLocation;
        if (session.isAuthenticated) {
          return location == '/home' ? null : '/home';
        }
        if (location == '/home') return '/';
        if (location == '/verify' && session.phoneNumber == null) {
          return '/auth';
        }
        return null;
      },
      routes: [
        GoRoute(path: '/', builder: (_, __) => const OnboardingScreen()),
        GoRoute(
          path: '/auth',
          builder: (_, __) => PhoneSignInScreen(session: session),
        ),
        GoRoute(
          path: '/verify',
          builder: (_, __) => VerifyPhoneScreen(session: session),
        ),
        GoRoute(
          path: '/home',
          builder: (_, __) => MobilityAppScreen(
            phoneNumber: session.phoneNumber ?? '',
            onSignOut: session.signOut,
          ),
        ),
      ],
    );
