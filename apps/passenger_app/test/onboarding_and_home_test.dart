import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:qr_flutter/qr_flutter.dart';
import 'package:sarathi_passenger/app.dart';
import 'package:sarathi_passenger/features/home/data/backend_status_repository.dart';
import 'package:sarathi_passenger/features/mobility/data/mobility_catalog.dart';

Widget _app() => ProviderScope(
      overrides: [
        backendStatusProvider
            .overrideWith((ref) async => BackendStatus.unreachable),
      ],
      child: const SarathiPassengerApp(),
    );

Future<void> _openHome(WidgetTester tester) async {
  final getStarted = find.text('Get started');
  await tester.ensureVisible(getStarted);
  await tester.tap(getStarted);
  await tester.pumpAndSettle();

  final continueButton = find.byKey(const Key('authContinueButton'));
  await tester.ensureVisible(continueButton);
  await tester.enterText(
    find.byKey(const Key('authPhoneField')),
    '9876543210',
  );
  await tester.tap(continueButton);
  await tester.pumpAndSettle();

  final verifyButton = find.byKey(const Key('authVerifyButton'));
  await tester.ensureVisible(verifyButton);
  await tester.enterText(
    find.byKey(const Key('authCodeField')),
    '123456',
  );
  await tester.tap(verifyButton);
  await tester.pumpAndSettle();
}

Future<void> _openTab(WidgetTester tester, String label) async {
  final destination = find.text(label).last;
  await tester.ensureVisible(destination);
  await tester.tap(destination);
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('onboarding introduces SARATHI and explains GiB data units',
      (tester) async {
    await tester.pumpWidget(_app());
    expect(find.text('SARATHI'), findsOneWidget);
    expect(find.text('Smart Mobility Connectivity Platform'), findsOneWidget);
    expect(find.textContaining('GiB'), findsOneWidget);
    expect(find.text('Your data'), findsOneWidget);
    expect(find.text('Get started'), findsOneWidget);
  });

  testWidgets('phone sign-in validates, verifies and signs out',
      (tester) async {
    await tester.pumpWidget(_app());
    final getStarted = find.text('Get started');
    await tester.ensureVisible(getStarted);
    await tester.tap(getStarted);
    await tester.pumpAndSettle();

    final continueButton = find.byKey(const Key('authContinueButton'));
    await tester.ensureVisible(continueButton);
    await tester.tap(continueButton);
    await tester.pumpAndSettle();
    expect(
      find.text('Enter a valid 10-digit Indian mobile number.'),
      findsOneWidget,
    );

    await tester.enterText(
      find.byKey(const Key('authPhoneField')),
      '9876543210',
    );
    await tester.ensureVisible(continueButton);
    await tester.tap(continueButton);
    await tester.pumpAndSettle();
    expect(find.text('DEMO CODE · NO SMS SENT'), findsOneWidget);
    expect(find.text('123456'), findsOneWidget);

    await tester.enterText(find.byKey(const Key('authCodeField')), '000000');
    final verifyButton = find.byKey(const Key('authVerifyButton'));
    await tester.ensureVisible(verifyButton);
    await tester.tap(verifyButton);
    await tester.pumpAndSettle();
    expect(find.text('That code is not correct. Try the demo code.'),
        findsOneWidget);

    await tester.enterText(find.byKey(const Key('authCodeField')), '123456');
    await tester.ensureVisible(verifyButton);
    await tester.tap(find.byKey(const Key('authVerifyButton')));
    await tester.pumpAndSettle();
    expect(find.byTooltip('Account'), findsOneWidget);

    await tester.tap(find.byTooltip('Account'));
    await tester.pumpAndSettle();
    expect(find.text('+91 9876543210'), findsOneWidget);
    await tester.tap(find.text('Sign out'));
    await tester.pumpAndSettle();
    expect(find.text('Get started'), findsOneWidget);
  });

  testWidgets('map updates vehicle energy and route recommendations',
      (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          backendStatusProvider
              .overrideWith((ref) async => BackendStatus.reachable),
        ],
        child: const SarathiPassengerApp(),
      ),
    );
    await _openHome(tester);

    final carPicker = find.byType(DropdownButtonFormField<VehicleSpec>);
    await tester.ensureVisible(carPicker);
    await tester.tap(carPicker);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Tata Nexon EV').last);
    await tester.pumpAndSettle();

    expect(find.text('BATTERY CHARGE (%)'), findsOneWidget);
    expect(find.textContaining('3.995 × 1.802 × 1.625 m'), findsOneWidget);
    await tester.enterText(find.byType(TextField).last, '1');
    await tester.pumpAndSettle();
    expect(find.text('3 km estimated range'), findsOneWidget);
    final routeButton = find.text('Show route options');
    await tester.ensureVisible(routeButton);
    await tester.tap(routeButton);
    await tester.pumpAndSettle();

    expect(find.text('Ring Road'), findsOneWidget);
    expect(find.text('Old Town Lane'), findsOneWidget);
    expect(find.text('Plan an energy stop before this route'), findsWidgets);
    await tester.ensureVisible(find.text('EV charging on the way'));
    expect(find.text('ChargeGrid Hub'), findsOneWidget);

    final saveRoute = find.text('Save as daily commute');
    await tester.ensureVisible(saveRoute);
    await tester.tap(saveRoute);
    await tester.pumpAndSettle();
    await _openTab(tester, 'Daily');
    expect(find.text('SAVED FROM MAP'), findsOneWidget);
  });

  testWidgets('trip booking switches to premium cars and shows a demo quote',
      (tester) async {
    await tester.pumpWidget(_app());
    await _openHome(tester);
    await _openTab(tester, 'Trips');

    expect(find.text('Estimated parking allowance'), findsNothing);
    final pickupToggle = find.text('Pick someone up and return');
    await tester.ensureVisible(pickupToggle);
    await tester.tap(pickupToggle);
    await tester.pumpAndSettle();
    expect(find.text('Estimated parking allowance'), findsOneWidget);

    final premium = find.text('Premium').last;
    await tester.ensureVisible(premium);
    await tester.tap(premium);
    await tester.pumpAndSettle();
    expect(find.text('Mercedes-Benz E-Class'), findsOneWidget);
    expect(find.text('BOOKING DURATION'), findsOneWidget);

    final requestButton = find.text('Request event car');
    await tester.ensureVisible(requestButton);
    await tester.tap(requestButton);
    await tester.pumpAndSettle();
    expect(find.text('Event car request saved'), findsOneWidget);
    expect(find.textContaining('is a demo'), findsOneWidget);
    await tester.tap(find.text('Done'));
    await tester.pumpAndSettle();

    final testDriveButton = find.text('Book area-limited test drive');
    await tester.ensureVisible(testDriveButton);
    await tester.tap(testDriveButton);
    await tester.pumpAndSettle();
    expect(find.text('Test-drive request saved'), findsOneWidget);
    await tester.tap(find.text('Done'));
    await tester.pumpAndSettle();
    expect(find.text('Model test-drive already used'), findsOneWidget);

    final tour = find.text('Tour').last;
    await tester.ensureVisible(tour);
    await tester.tap(tour);
    await tester.pumpAndSettle();
    final stayOption = find.text('Include a sample overnight stay');
    await tester.ensureVisible(stayOption);
    await tester.tap(stayOption);
    await tester.pumpAndSettle();
    expect(find.text('Sample lodging add-on'), findsOneWidget);
    expect(find.text('₹13400'), findsOneWidget);
  });

  testWidgets('daily commute creates a clearly marked demo QR preview',
      (tester) async {
    await tester.pumpWidget(_app());
    await _openHome(tester);
    await _openTab(tester, 'Daily');

    final previewButton = find.text('Preview demo trip QR');
    await tester.ensureVisible(previewButton);
    await tester.tap(previewButton);
    await tester.pumpAndSettle();
    expect(find.byType(QrImageView), findsOneWidget);
    expect(find.text('Your demo trip QR'), findsOneWidget);
    expect(find.textContaining('not a ticket'), findsOneWidget);
    expect(find.text('₹45'), findsOneWidget);
    await tester.ensureVisible(find.text('Train'));
    await tester.tap(find.text('Train'));
    await tester.pumpAndSettle();
    expect(find.text('₹85'), findsOneWidget);
  });

  testWidgets('shop switches categories, adds a product and totals the cart',
      (tester) async {
    await tester.pumpWidget(_app());
    await _openHome(tester);
    await _openTab(tester, 'Shop');

    await tester.tap(find.text('Festival & ceremony'));
    await tester.pumpAndSettle();
    expect(find.text('Celebration gift box'), findsOneWidget);

    await tester.tap(find.byTooltip('Add Celebration gift box to cart'));
    await tester.pumpAndSettle();
    expect(find.text('1 item · ₹1799'), findsOneWidget);

    final reviewButton = find.text('Review cart');
    await tester.ensureVisible(reviewButton);
    await tester.tap(reviewButton);
    await tester.pumpAndSettle();
    expect(find.text('Demo cart ready'), findsOneWidget);
  });

  testWidgets('Wi-Fi shows the 2 GiB allowance and refreshes service status',
      (tester) async {
    var checks = 0;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          backendStatusProvider.overrideWith((ref) async {
            checks++;
            return checks == 1
                ? BackendStatus.unreachable
                : BackendStatus.reachable;
          }),
        ],
        child: const SarathiPassengerApp(),
      ),
    );
    await _openHome(tester);
    await _openTab(tester, 'Wi-Fi');
    expect(find.textContaining('2 GiB included'), findsOneWidget);

    final offline = find.text('SARATHI service unreachable');
    await tester.ensureVisible(offline);
    expect(offline, findsOneWidget);

    await tester.tap(find.byTooltip('Refresh service status'));
    await tester.pumpAndSettle();

    expect(checks, 2);
    final online = find.text('SARATHI service is reachable');
    await tester.ensureVisible(online);
    expect(online, findsOneWidget);
  });

  testWidgets('navigation stays usable at a narrow phone width',
      (tester) async {
    tester.view.physicalSize = const Size(320, 640);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(_app());
    await _openHome(tester);

    expect(find.text('Find your way.'), findsOneWidget);
    expect(find.text('Explore'), findsOneWidget);
    expect(tester.takeException(), isNull);

    for (final tab in ['Trips', 'Daily', 'Shop', 'Wi-Fi']) {
      await _openTab(tester, tab);
      final exception = tester.takeException();
      expect(exception, isNull, reason: '$tab narrow layout: $exception');
    }
  });
}
