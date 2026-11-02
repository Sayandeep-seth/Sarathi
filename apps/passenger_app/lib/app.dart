import 'package:flutter/material.dart';

import 'core/router/app_router.dart';
import 'core/theme/app_theme.dart';
import 'features/auth/data/demo_auth_session.dart';

class SarathiPassengerApp extends StatefulWidget {
  const SarathiPassengerApp({super.key});

  @override
  State<SarathiPassengerApp> createState() => _SarathiPassengerAppState();
}

class _SarathiPassengerAppState extends State<SarathiPassengerApp> {
  late final DemoAuthSession _session;
  late final _router = buildRouter(_session);

  @override
  void initState() {
    super.initState();
    _session = DemoAuthSession();
  }

  @override
  void dispose() {
    _router.dispose();
    _session.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => MaterialApp.router(
        title: 'SARATHI',
        theme: AppTheme.light(),
        routerConfig: _router,
      );
}
