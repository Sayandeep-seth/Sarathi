import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sarathi_passenger/app.dart';
import 'package:sarathi_passenger/features/home/data/backend_status_repository.dart';

void main() {
  testWidgets('onboarding explains the service, GiB units and data collection',
      (tester) async {
    await tester.pumpWidget(const ProviderScope(child: SarathiPassengerApp()));
    expect(find.text('SARATHI'), findsOneWidget);
    expect(find.text('Smart Mobility Connectivity Platform'), findsOneWidget);
    expect(find.textContaining('GiB'), findsOneWidget);
    expect(find.text('Your data'), findsOneWidget);
  });

  testWidgets('Get started navigates to home and shows reachable status',
      (tester) async {
    await tester.pumpWidget(ProviderScope(
      overrides: [
        backendStatusProvider
            .overrideWith((ref) async => BackendStatus.reachable),
      ],
      child: const SarathiPassengerApp(),
    ));
    await tester.tap(find.text('Get started'));
    await tester.pumpAndSettle();
    expect(find.text('SARATHI service is reachable'), findsOneWidget);
  });

  testWidgets('home shows unreachable state without crashing', (tester) async {
    await tester.pumpWidget(ProviderScope(
      overrides: [
        backendStatusProvider
            .overrideWith((ref) async => BackendStatus.unreachable),
      ],
      child: const SarathiPassengerApp(),
    ));
    await tester.tap(find.text('Get started'));
    await tester.pumpAndSettle();
    expect(find.text('SARATHI service unreachable'), findsOneWidget);
  });
}
