import 'package:flutter/material.dart';

import 'core/router/app_router.dart';
import 'core/theme/app_theme.dart';

class SarathiPassengerApp extends StatefulWidget {
  const SarathiPassengerApp({super.key});

  @override
  State<SarathiPassengerApp> createState() => _SarathiPassengerAppState();
}

class _SarathiPassengerAppState extends State<SarathiPassengerApp> {
  final _router = buildRouter();

  @override
  Widget build(BuildContext context) => MaterialApp.router(
        title: 'SARATHI',
        theme: AppTheme.light(),
        routerConfig: _router,
      );
}
