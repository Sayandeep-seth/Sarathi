import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/theme/app_theme.dart';
import '../../home/data/backend_status_repository.dart';
import 'mobility_widgets.dart';

class WifiScreen extends ConsumerStatefulWidget {
  const WifiScreen({super.key});

  @override
  ConsumerState<WifiScreen> createState() => _WifiScreenState();
}

class _WifiScreenState extends ConsumerState<WifiScreen> {
  static const double _includedQuotaGiB = 2;
  double _usedGiB = 0.6;
  int _selectedTopUpGiB = 1;

  @override
  Widget build(BuildContext context) {
    final serviceStatus = ref.watch(backendStatusProvider);
    final remaining =
        (_includedQuotaGiB - _usedGiB).clamp(0, _includedQuotaGiB);
    final colors = Theme.of(context).colorScheme;
    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(20, 16, 20, 32),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const MobilityHeader(
            eyebrow: 'Stay connected',
            title: 'Cab Wi-Fi.',
            subtitle: 'A sample 2 GiB allowance for a more connected journey.',
          ),
          const SizedBox(height: 20),
          Container(
            padding: const EdgeInsets.all(19),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(23),
              gradient: const LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [Color(0xFF145D50), Color(0xFF073B38)],
              ),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Container(
                      width: 43,
                      height: 43,
                      decoration: BoxDecoration(
                        color: AppTheme.translucent(Colors.white, 0.14),
                        borderRadius: BorderRadius.circular(14),
                      ),
                      child: const Icon(
                        Icons.satellite_alt_rounded,
                        color: Color(0xFFA8E3B7),
                        size: 23,
                      ),
                    ),
                    const SizedBox(width: 11),
                    const Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            'In-ride Wi-Fi allowance',
                            style: TextStyle(
                              color: Colors.white,
                              fontWeight: FontWeight.w800,
                            ),
                          ),
                          SizedBox(height: 3),
                          Text(
                            'Starlink™ integration · planned',
                            style: TextStyle(
                              color: Color(0xFFD0E4D9),
                              fontSize: 11,
                            ),
                          ),
                        ],
                      ),
                    ),
                    const Icon(Icons.wifi_rounded, color: Colors.white),
                  ],
                ),
                const SizedBox(height: 22),
                Row(
                  crossAxisAlignment: CrossAxisAlignment.end,
                  children: [
                    Text(
                      remaining.toStringAsFixed(1),
                      style: Theme.of(context).textTheme.displaySmall?.copyWith(
                            color: Colors.white,
                            fontWeight: FontWeight.w900,
                            height: 1,
                          ),
                    ),
                    const SizedBox(width: 7),
                    const Expanded(
                      child: Padding(
                        padding: EdgeInsets.only(bottom: 4),
                        child: Text(
                          'GiB remaining',
                          style: TextStyle(color: Colors.white),
                        ),
                      ),
                    ),
                    const SizedBox(width: 8),
                    Text(
                      'of ${_includedQuotaGiB.toStringAsFixed(0)} GiB',
                      style: const TextStyle(color: Color(0xFFD0E4D9)),
                    ),
                  ],
                ),
                const SizedBox(height: 13),
                ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: LinearProgressIndicator(
                    value: remaining / _includedQuotaGiB,
                    minHeight: 8,
                    backgroundColor: AppTheme.translucent(Colors.white, 0.2),
                    color: const Color(0xFFA8E3B7),
                  ),
                ),
                const SizedBox(height: 11),
                Text(
                  '${_usedGiB.toStringAsFixed(1)} GiB used · 2 GiB included per eligible trip',
                  style: const TextStyle(
                    color: Color(0xFFD0E4D9),
                    fontSize: 11,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 14),
          AppSurface(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SectionHeading('Usage preview'),
                const SizedBox(height: 4),
                Text(
                  'Move the slider to preview quota usage.',
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                        color: colors.onSurfaceVariant,
                      ),
                ),
                Slider(
                  value: _usedGiB,
                  min: 0,
                  max: 3,
                  divisions: 30,
                  label: '${_usedGiB.toStringAsFixed(1)} GiB',
                  onChanged: (value) => setState(() => _usedGiB = value),
                ),
                if (_usedGiB >= _includedQuotaGiB)
                  const SampleNotice(
                    icon: Icons.warning_amber_rounded,
                    message:
                        'The sample allowance is used. Purchase controls below are a preview only.',
                  ),
              ],
            ),
          ),
          const SizedBox(height: 14),
          AppSurface(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SectionHeading('Add data'),
                const SizedBox(height: 12),
                for (final (amount, price) in [(1, 49), (5, 199)])
                  RadioListTile<int>(
                    value: amount,
                    groupValue: _selectedTopUpGiB,
                    contentPadding: EdgeInsets.zero,
                    title: Text('+$amount GiB data pack'),
                    subtitle: const Text('Usable after the included allowance'),
                    secondary: Text(
                      '₹$price',
                      style: const TextStyle(
                        color: AppTheme.emerald,
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                    onChanged: (value) {
                      if (value != null) {
                        setState(() => _selectedTopUpGiB = value);
                      }
                    },
                  ),
                const SizedBox(height: 5),
                SizedBox(
                  width: double.infinity,
                  child: FilledButton.icon(
                    onPressed: () => showDemoConfirmation(
                      context,
                      title: 'Top-up preview',
                      detail:
                          'The +$_selectedTopUpGiB GiB data pack is a demo. No payment is processed and no data is added.',
                    ),
                    icon: const Icon(Icons.add_rounded),
                    label: const Text('Preview data top-up'),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 14),
          _ServiceStatusCard(
            status: serviceStatus,
            onRefresh: () async {
              ref.invalidate(backendStatusProvider);
              await ref.read(backendStatusProvider.future);
            },
          ),
          const SizedBox(height: 14),
          const SampleNotice(
            message:
                '2 GiB here means 2 × 1,073,741,824 bytes. Usage, Starlink availability, network enforcement and purchases are not connected to a live account.',
          ),
        ],
      ),
    );
  }
}

class _ServiceStatusCard extends StatelessWidget {
  const _ServiceStatusCard({
    required this.status,
    required this.onRefresh,
  });

  final AsyncValue<BackendStatus> status;
  final VoidCallback onRefresh;

  @override
  Widget build(BuildContext context) {
    final reachable = status.valueOrNull == BackendStatus.reachable;
    final loading = status.isLoading;
    final color = loading
        ? Theme.of(context).colorScheme.primary
        : reachable
            ? const Color(0xFF16805E)
            : const Color(0xFFC44D42);
    return AppSurface(
      padding: const EdgeInsets.all(14),
      child: Row(
        children: [
          Icon(
            loading
                ? Icons.sync_rounded
                : reachable
                    ? Icons.check_circle_outline_rounded
                    : Icons.cloud_off_rounded,
            color: color,
          ),
          const SizedBox(width: 9),
          Expanded(
            child: Text(
              loading
                  ? 'Checking SARATHI service…'
                  : reachable
                      ? 'SARATHI service is reachable'
                      : 'SARATHI service unreachable',
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    fontWeight: FontWeight.w700,
                  ),
            ),
          ),
          IconButton(
            onPressed: loading ? null : onRefresh,
            tooltip: 'Refresh service status',
            icon: const Icon(Icons.refresh_rounded),
          ),
        ],
      ),
    );
  }
}
