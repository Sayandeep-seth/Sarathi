import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../../core/theme/app_theme.dart';

class OnboardingScreen extends StatelessWidget {
  const OnboardingScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    final scheme = Theme.of(context).colorScheme;
    return Scaffold(
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(24, 20, 24, 28),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Container(
                    width: 46,
                    height: 46,
                    decoration: BoxDecoration(
                      color: scheme.primary,
                      borderRadius: BorderRadius.circular(16),
                      boxShadow: [
                        BoxShadow(
                          color: AppTheme.translucent(scheme.primary, 0.2),
                          blurRadius: 16,
                          offset: const Offset(0, 7),
                        ),
                      ],
                    ),
                    child: const Icon(Icons.route_rounded, color: Colors.white),
                  ),
                  const SizedBox(width: 12),
                  Semantics(
                    header: true,
                    child: Text(
                      'SARATHI',
                      style: text.titleLarge?.copyWith(
                        color: scheme.primary,
                        fontWeight: FontWeight.w900,
                        letterSpacing: 1.5,
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 36),
              Container(
                height: 154,
                width: double.infinity,
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(26),
                  gradient: const LinearGradient(
                    begin: Alignment.topLeft,
                    end: Alignment.bottomRight,
                    colors: [Color(0xFF145D50), Color(0xFF073B38)],
                  ),
                ),
                child: Stack(
                  clipBehavior: Clip.hardEdge,
                  children: [
                    Positioned(
                      right: -24,
                      top: -62,
                      child: Container(
                        width: 180,
                        height: 180,
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          border: Border.all(
                            color: AppTheme.translucent(Colors.white, 0.12),
                            width: 18,
                          ),
                        ),
                      ),
                    ),
                    Positioned(
                      left: 20,
                      top: 20,
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            'MOVE WITH EASE',
                            style: text.labelSmall?.copyWith(
                              color: const Color(0xFFA8E3B7),
                              letterSpacing: 1.5,
                              fontWeight: FontWeight.w800,
                            ),
                          ),
                          const SizedBox(height: 9),
                          Text(
                            'Every ride,\na little smarter.',
                            style: text.headlineSmall?.copyWith(
                              color: Colors.white,
                              height: 1.05,
                              fontWeight: FontWeight.w800,
                            ),
                          ),
                        ],
                      ),
                    ),
                    Positioned(
                      right: 18,
                      bottom: 13,
                      child: Transform.rotate(
                        angle: -0.12,
                        child: Icon(
                          Icons.directions_car_filled_rounded,
                          size: 78,
                          color: AppTheme.translucent(
                            const Color(0xFFD8F0DF),
                            0.9,
                          ),
                          shadows: const [
                            Shadow(
                              color: Color(0x66000000),
                              blurRadius: 14,
                              offset: Offset(0, 8),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 27),
              Text(
                'Smart Mobility Connectivity Platform',
                style: text.titleLarge?.copyWith(fontWeight: FontWeight.w800),
              ),
              const SizedBox(height: 8),
              Text(
                'Explore route planning, rides, daily commute, rewards and in-cab Wi-Fi.',
                style: text.bodyMedium?.copyWith(
                  color: scheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 22),
              const _Point(
                icon: Icons.alt_route_rounded,
                title: 'Plan a smarter route',
                body:
                    'Compare sample road widths for a selected car and find example petrol or EV stops.',
              ),
              const _Point(
                icon: Icons.local_taxi_rounded,
                title: 'Choose your kind of ride',
                body:
                    'Preview everyday cabs, premium and vintage cars, tours, daily passes and demo drives.',
              ),
              const _Point(
                icon: Icons.wifi_rounded,
                title: 'Stay connected on the go',
                body:
                    'Explore a sample 2 GiB Wi-Fi allowance and top-ups. Usage and purchases are not live.',
              ),
              const _Point(
                icon: Icons.privacy_tip_outlined,
                title: 'Your data',
                body:
                    'Phone sign-in is a local demo. No SMS is sent and no personal data is saved or sent to a server.',
              ),
              const SizedBox(height: 4),
              FilledButton(
                onPressed: () => context.go('/auth'),
                child: const Text('Get started'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _Point extends StatelessWidget {
  const _Point({required this.icon, required this.title, required this.body});
  final IconData icon;
  final String title;
  final String body;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.only(bottom: 11),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 42,
            height: 42,
            decoration: BoxDecoration(
              color: AppTheme.translucent(scheme.primary, 0.09),
              borderRadius: BorderRadius.circular(14),
            ),
            child: ExcludeSemantics(
              child: Icon(icon, color: scheme.primary, size: 21),
            ),
          ),
          const SizedBox(width: 13),
          Expanded(
            child: Padding(
              padding: const EdgeInsets.only(top: 1),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    title,
                    style: Theme.of(context).textTheme.titleSmall?.copyWith(
                          fontWeight: FontWeight.w800,
                        ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    body,
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(
                          color: scheme.onSurfaceVariant,
                          height: 1.4,
                        ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
