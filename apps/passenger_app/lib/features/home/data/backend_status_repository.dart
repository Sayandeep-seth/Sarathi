import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/api_client.dart';

enum BackendStatus { reachable, unreachable }

/// Checks that the SARATHI API answers its readiness endpoint.
/// This reports *service* reachability only; it says nothing about Wi-Fi quota.
class BackendStatusRepository {
  BackendStatusRepository(this._dio);
  final Dio _dio;

  Future<BackendStatus> check() async {
    try {
      final res = await _dio.get<Object>('/healthz');
      return res.statusCode == 200
          ? BackendStatus.reachable
          : BackendStatus.unreachable;
    } on DioException {
      return BackendStatus.unreachable;
    }
  }
}

final backendStatusRepositoryProvider = Provider<BackendStatusRepository>(
  (ref) => BackendStatusRepository(ref.watch(dioProvider)),
);

final backendStatusProvider = FutureProvider.autoDispose<BackendStatus>(
  (ref) => ref.watch(backendStatusRepositoryProvider).check(),
);
