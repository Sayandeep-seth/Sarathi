/// Build-time configuration. Nothing secret belongs here: the app is a public client.
///
/// Override with: flutter run --dart-define=SARATHI_API_BASE_URL=https://api.example.com
class AppConfig {
  const AppConfig._();

  /// Android emulator reaches the host machine at 10.0.2.2.
  static const String apiBaseUrl = String.fromEnvironment(
    'SARATHI_API_BASE_URL',
    defaultValue: 'http://10.0.2.2:8080',
  );

  /// Development fixtures are compiled in only for non-release builds.
  static const bool enableDemoFixtures = !bool.fromEnvironment('dart.vm.product');
}
