import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/theme/app_theme.dart';
import '../data/backend_status_repository.dart';

class HomeScreen extends ConsumerWidget {
  const HomeScreen({super.key});

  Future<void> _refresh(WidgetRef ref) async {
    ref.invalidate(backendStatusProvider);
    await ref.read(backendStatusProvider.future);
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final status = ref.watch(backendStatusProvider);
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;

    return Scaffold(
      body: SafeArea(
        child: RefreshIndicator(
          onRefresh: () => _refresh(ref),
          child: CustomScrollView(
            physics: const AlwaysScrollableScrollPhysics(),
            slivers: [
              SliverPadding(
                padding: const EdgeInsets.fromLTRB(22, 16, 22, 32),
                sliver: SliverList(
                  delegate: SliverChildListDelegate([
                    LayoutBuilder(
                      builder: (context, constraints) {
                        final compact = constraints.maxWidth < 350;
                        return Row(
                          children: [
                            Container(
                              width: 44,
                              height: 44,
                              decoration: BoxDecoration(
                                color: colors.primary,
                                borderRadius: BorderRadius.circular(15),
                                boxShadow: [
                                  BoxShadow(
                                    color: AppTheme.translucent(
                                      colors.primary,
                                      0.2,
                                    ),
                                    blurRadius: 14,
                                    offset: const Offset(0, 7),
                                  ),
                                ],
                              ),
                              child: const Icon(
                                Icons.route_rounded,
                                color: Colors.white,
                                size: 23,
                              ),
                            ),
                            const SizedBox(width: 12),
                            Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  'SARATHI',
                                  style: text.titleMedium?.copyWith(
                                    fontWeight: FontWeight.w900,
                                    letterSpacing: 1.5,
                                  ),
                                ),
                                if (!compact)
                                  Text(
                                    'SMART MOBILITY',
                                    style: text.labelSmall?.copyWith(
                                      color: colors.onSurfaceVariant,
                                      letterSpacing: 1.2,
                                      fontWeight: FontWeight.w700,
                                    ),
                                  ),
                              ],
                            ),
                            const Spacer(),
                            Container(
                              padding: EdgeInsets.symmetric(
                                horizontal: compact ? 10 : 12,
                                vertical: 8,
                              ),
                              decoration: BoxDecoration(
                                color: Colors.white,
                                borderRadius: BorderRadius.circular(30),
                                border:
                                    Border.all(color: colors.outlineVariant),
                              ),
                              child: Row(
                                children: [
                                  Icon(
                                    Icons.shield_outlined,
                                    size: 15,
                                    color: colors.primary,
                                  ),
                                  if (!compact) ...[
                                    const SizedBox(width: 6),
                                    Text(
                                      'PASSENGER',
                                      style: text.labelSmall?.copyWith(
                                        fontWeight: FontWeight.w800,
                                        letterSpacing: 0.5,
                                      ),
                                    ),
                                  ],
                                ],
                              ),
                            ),
                          ],
                        );
                      },
                    ),
                    const SizedBox(height: 32),
                    Text(
                      'Your journey,\nconnected.',
                      style: text.headlineLarge?.copyWith(
                        fontWeight: FontWeight.w800,
                        height: 1.04,
                        letterSpacing: -1.2,
                      ),
                    ),
                    const SizedBox(height: 10),
                    Text(
                      'A smarter ride starts with a better connection.',
                      style: text.bodyLarge?.copyWith(
                        color: colors.onSurfaceVariant,
                      ),
                    ),
                    const SizedBox(height: 24),
                    const _JourneyCard(),
                    const SizedBox(height: 26),
                    Row(
                      children: [
                        Text(
                          'SERVICE CHECK',
                          style: text.labelMedium?.copyWith(
                            color: colors.onSurfaceVariant,
                            letterSpacing: 1.3,
                            fontWeight: FontWeight.w800,
                          ),
                        ),
                        const Spacer(),
                        Icon(
                          Icons.sync_rounded,
                          size: 15,
                          color: colors.onSurfaceVariant,
                        ),
                        const SizedBox(width: 5),
                        Text(
                          'LIVE',
                          style: text.labelSmall?.copyWith(
                            color: colors.onSurfaceVariant,
                            letterSpacing: 1,
                            fontWeight: FontWeight.w800,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 10),
                    _ServiceStatusCard(
                      status: status,
                      onRefresh: () => _refresh(ref),
                    ),
                    const SizedBox(height: 20),
                    Container(
                      padding: const EdgeInsets.all(16),
                      decoration: BoxDecoration(
                        color: const Color(0xFFEAF2EF),
                        borderRadius: BorderRadius.circular(18),
                      ),
                      child: Row(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Icon(
                            Icons.info_outline_rounded,
                            size: 19,
                            color: colors.primary,
                          ),
                          const SizedBox(width: 10),
                          Expanded(
                            child: Text(
                              'This check reports SARATHI service availability only. '
                              'Trip verification and in-cab Wi-Fi features are not available yet.',
                              style: text.bodySmall?.copyWith(
                                color: colors.onSurfaceVariant,
                                height: 1.45,
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(height: 20),
                    Center(
                      child: Text(
                        'MADE FOR THE WAY YOU MOVE',
                        style: text.labelSmall?.copyWith(
                          color: colors.onSurfaceVariant,
                          letterSpacing: 1.5,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ),
                  ]),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _JourneyCard extends StatelessWidget {
  const _JourneyCard();

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;

    return Container(
      height: 226,
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(28),
        gradient: const LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [Color(0xFF145D50), Color(0xFF073B38)],
        ),
        boxShadow: const [
          BoxShadow(
            color: Color(0x2611423A),
            blurRadius: 24,
            offset: Offset(0, 12),
          ),
        ],
      ),
      child: LayoutBuilder(
        builder: (context, constraints) {
          final compact = constraints.maxWidth < 330;
          return Stack(
            children: [
              Positioned(
                right: -42,
                top: -92,
                child: Container(
                  width: 230,
                  height: 230,
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    border: Border.all(
                      color: AppTheme.translucent(Colors.white, 0.08),
                    ),
                  ),
                ),
              ),
              Positioned(
                right: 4,
                top: -65,
                child: Container(
                  width: 175,
                  height: 175,
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    border: Border.all(
                      color: AppTheme.translucent(Colors.white, 0.08),
                    ),
                  ),
                ),
              ),
              Positioned(
                left: 20,
                top: 20,
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 10,
                    vertical: 7,
                  ),
                  decoration: BoxDecoration(
                    color: AppTheme.translucent(Colors.white, 0.12),
                    borderRadius: BorderRadius.circular(30),
                    border: Border.all(
                      color: AppTheme.translucent(Colors.white, 0.12),
                    ),
                  ),
                  child: Text(
                    'MOVE WITH EASE',
                    style: text.labelSmall?.copyWith(
                      color: Colors.white,
                      letterSpacing: 1.3,
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                ),
              ),
              Positioned(
                left: 22,
                top: 66,
                child: SizedBox(
                  width: compact ? 135 : 180,
                  child: Text(
                    'A better\nride is\non the way.',
                    style: text.headlineMedium?.copyWith(
                      color: Colors.white,
                      fontSize: compact ? 26 : null,
                      height: 1.02,
                      letterSpacing: -0.7,
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                ),
              ),
              Positioned(
                bottom: 0,
                right: compact ? -8 : -12,
                width: compact ? 145 : 190,
                height: compact ? 130 : 158,
                child: ExcludeSemantics(
                  child: CustomPaint(painter: _CabPainter()),
                ),
              ),
              Positioned(
                left: 22,
                bottom: 18,
                child: Row(
                  children: [
                    const Icon(
                      Icons.bolt_rounded,
                      color: Color(0xFFA8E3B7),
                      size: 17,
                    ),
                    const SizedBox(width: 5),
                    Text(
                      'SMARTER MOBILITY',
                      style: text.labelSmall?.copyWith(
                        color: AppTheme.translucent(Colors.white, 0.85),
                        letterSpacing: 1.1,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _CabPainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final scale = size.width / 220;
    canvas.save();
    canvas.scale(scale);

    final glow = Paint()
      ..color = AppTheme.translucent(const Color(0xFF8FE0B2), 0.14)
      ..maskFilter = const MaskFilter.blur(BlurStyle.normal, 22);
    canvas.drawOval(const Rect.fromLTWH(38, 91, 166, 48), glow);

    final shadow = Paint()..color = const Color(0xFF062C2D);
    canvas.drawOval(const Rect.fromLTWH(30, 124, 180, 22), shadow);

    final body = Paint()
      ..shader = const LinearGradient(
        begin: Alignment.topCenter,
        end: Alignment.bottomCenter,
        colors: [Color(0xFFE5F5EA), Color(0xFF9CD4B5)],
      ).createShader(const Rect.fromLTWH(28, 68, 184, 70));
    final bodyPath = Path()
      ..moveTo(27, 105)
      ..lineTo(42, 91)
      ..lineTo(67, 87)
      ..lineTo(87, 67)
      ..quadraticBezierTo(92, 62, 100, 62)
      ..lineTo(142, 62)
      ..quadraticBezierTo(151, 62, 158, 70)
      ..lineTo(176, 89)
      ..lineTo(199, 96)
      ..quadraticBezierTo(208, 99, 210, 108)
      ..lineTo(210, 121)
      ..quadraticBezierTo(210, 129, 201, 130)
      ..lineTo(37, 130)
      ..quadraticBezierTo(26, 129, 26, 120)
      ..lineTo(26, 112)
      ..quadraticBezierTo(26, 108, 27, 105)
      ..close();
    canvas.drawPath(bodyPath, body);

    final glass = Paint()..color = const Color(0xFF397E75);
    final frontWindow = Path()
      ..moveTo(94, 70)
      ..lineTo(104, 68)
      ..lineTo(124, 68)
      ..lineTo(124, 85)
      ..lineTo(80, 85)
      ..close();
    final backWindow = Path()
      ..moveTo(131, 68)
      ..lineTo(142, 68)
      ..quadraticBezierTo(147, 68, 152, 74)
      ..lineTo(163, 85)
      ..lineTo(131, 85)
      ..close();
    canvas.drawPath(frontWindow, glass);
    canvas.drawPath(backWindow, glass);

    final trim = Paint()
      ..color = const Color(0xFF538F7D)
      ..strokeWidth = 2
      ..style = PaintingStyle.stroke;
    canvas.drawLine(const Offset(126, 67), const Offset(126, 124), trim);
    canvas.drawLine(const Offset(39, 114), const Offset(198, 114), trim);

    final headlight = Paint()..color = const Color(0xFFFFF1B8);
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        const Rect.fromLTWH(197, 101, 9, 6),
        const Radius.circular(3),
      ),
      headlight,
    );
    final wheel = Paint()..color = const Color(0xFF153F3B);
    final hub = Paint()..color = const Color(0xFFB7DDC6);
    for (final center in [const Offset(68, 128), const Offset(169, 128)]) {
      canvas.drawCircle(center, 16, wheel);
      canvas.drawCircle(center, 7, hub);
      canvas.drawCircle(center, 2, Paint()..color = const Color(0xFF397E75));
    }

    final road = Paint()
      ..color = AppTheme.translucent(Colors.white, 0.15)
      ..strokeWidth = 2;
    canvas.drawLine(const Offset(15, 151), const Offset(205, 151), road);
    canvas.drawLine(const Offset(17, 146), const Offset(32, 146), road);
    canvas.drawLine(const Offset(194, 146), const Offset(209, 146), road);
    canvas.restore();
  }

  @override
  bool shouldRepaint(covariant _CabPainter oldDelegate) => false;
}

class _ServiceStatusCard extends StatelessWidget {
  const _ServiceStatusCard({required this.status, required this.onRefresh});

  final AsyncValue<BackendStatus> status;
  final VoidCallback onRefresh;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final (icon, label, detail, color) = switch (status) {
      AsyncData(value: BackendStatus.reachable) => (
          Icons.check_circle_rounded,
          'SARATHI service is reachable',
          'The service responded successfully.',
          const Color(0xFF16805E),
        ),
      AsyncData(value: BackendStatus.unreachable) => (
          Icons.cloud_off_rounded,
          'SARATHI service unreachable',
          'Check your connection and try again.',
          const Color(0xFFC44D42),
        ),
      AsyncError() => (
          Icons.error_outline_rounded,
          'Could not check service',
          'Something went wrong while checking availability.',
          const Color(0xFFC44D42),
        ),
      _ => (
          Icons.wifi_tethering_rounded,
          'Checking service…',
          'Connecting securely to SARATHI.',
          colors.primary,
        ),
    };

    return Container(
      padding: const EdgeInsets.all(17),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(22),
        border: Border.all(color: colors.outlineVariant),
        boxShadow: const [
          BoxShadow(
            color: Color(0x080D3B33),
            blurRadius: 18,
            offset: Offset(0, 7),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 46,
                height: 46,
                decoration: BoxDecoration(
                  color: AppTheme.translucent(color, 0.1),
                  borderRadius: BorderRadius.circular(15),
                ),
                child: status.isLoading
                    ? Padding(
                        padding: const EdgeInsets.all(14),
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: color,
                        ),
                      )
                    : Icon(icon, color: color, size: 23),
              ),
              const SizedBox(width: 13),
              Expanded(
                child: Semantics(
                  liveRegion: true,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        label,
                        style: text.titleSmall?.copyWith(
                          fontWeight: FontWeight.w800,
                        ),
                      ),
                      const SizedBox(height: 3),
                      Text(
                        detail,
                        style: text.bodySmall?.copyWith(
                          color: colors.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 16),
          SizedBox(
            width: double.infinity,
            child: OutlinedButton.icon(
              onPressed: status.isLoading ? null : onRefresh,
              icon: const Icon(Icons.refresh_rounded, size: 19),
              label: const Text('Refresh status'),
            ),
          ),
        ],
      ),
    );
  }
}
