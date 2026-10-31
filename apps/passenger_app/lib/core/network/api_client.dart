import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../config/app_config.dart';

/// Single configured Dio instance. Timeouts are explicit; there is deliberately
/// no automatic retry interceptor yet, because retries must never be applied
/// blindly to non-idempotent calls (payments). A safe-method-only retry policy
/// is added in Milestone 2.
final dioProvider = Provider<Dio>((ref) {
  return Dio(
    BaseOptions(
      baseUrl: AppConfig.apiBaseUrl,
      connectTimeout: const Duration(seconds: 5),
      receiveTimeout: const Duration(seconds: 10),
      sendTimeout: const Duration(seconds: 10),
      headers: {'Accept': 'application/json'},
    ),
  );
});
