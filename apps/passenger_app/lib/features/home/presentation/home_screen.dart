import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/backend_status_repository.dart';

class HomeScreen extends ConsumerWidget {
  const HomeScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final status = ref.watch(backendStatusProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('SARATHI')),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: Card(
          child: Padding(
            padding: const EdgeInsets.all(20),
            child: status.when(
              loading: () => const Row(children: [
                SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(strokeWidth: 2)),
                SizedBox(width: 12),
                Text('Checking service…'),
              ]),
              error: (_, __) => const _StatusRow(
                  icon: Icons.error_outline, text: 'Could not check service'),
              data: (s) => switch (s) {
                BackendStatus.reachable => const _StatusRow(
                    icon: Icons.check_circle_outline,
                    text: 'SARATHI service is reachable'),
                BackendStatus.unreachable => const _StatusRow(
                    icon: Icons.cloud_off, text: 'SARATHI service unreachable'),
              },
            ),
          ),
        ),
      ),
    );
  }
}

class _StatusRow extends StatelessWidget {
  const _StatusRow({required this.icon, required this.text});
  final IconData icon;
  final String text;

  @override
  Widget build(BuildContext context) => Semantics(
        liveRegion: true,
        child: Row(children: [
          Icon(icon),
          const SizedBox(width: 12),
          Expanded(child: Text(text)),
        ]),
      );
}
