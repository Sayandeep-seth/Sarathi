import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

class OnboardingScreen extends StatelessWidget {
  const OnboardingScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    final scheme = Theme.of(context).colorScheme;
    return Scaffold(
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Spacer(),
              Semantics(
                header: true,
                child: Text('SARATHI',
                    style: text.displaySmall?.copyWith(
                        color: scheme.primary, fontWeight: FontWeight.w800)),
              ),
              Text('Smart Mobility Connectivity Platform',
                  style: text.titleMedium),
              const SizedBox(height: 32),
              const _Point(
                icon: Icons.qr_code_scanner,
                title: 'Scan the QR code in your cab',
                body: 'Connect to the cab Wi-Fi and verify your trip.',
              ),
              const _Point(
                icon: Icons.data_usage,
                title: 'Free data for your trip',
                body:
                    'Eligible trips include a free data quota. The exact amount is set by the service and shown in the app. Data is measured in GiB (1 GiB = 1,073,741,824 bytes).',
              ),
              const _Point(
                icon: Icons.privacy_tip_outlined,
                title: 'Your data',
                body:
                    'We collect your sign-in identifier, the device used to connect and your data usage for the trip. Details are in Privacy settings.',
              ),
              const Spacer(),
              FilledButton(
                onPressed: () => context.go('/home'),
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
    return Padding(
      padding: const EdgeInsets.only(bottom: 20),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ExcludeSemantics(
              child: Icon(icon, color: Theme.of(context).colorScheme.primary)),
          const SizedBox(width: 16),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: Theme.of(context).textTheme.titleMedium),
                const SizedBox(height: 4),
                Text(body),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
