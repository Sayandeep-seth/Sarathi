import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';

import '../../../core/theme/app_theme.dart';
import '../data/demo_auth_session.dart';

class PhoneSignInScreen extends StatefulWidget {
  const PhoneSignInScreen({required this.session, super.key});

  final DemoAuthSession session;

  @override
  State<PhoneSignInScreen> createState() => _PhoneSignInScreenState();
}

class _PhoneSignInScreenState extends State<PhoneSignInScreen> {
  final _formKey = GlobalKey<FormState>();
  final _phoneController = TextEditingController();

  @override
  void dispose() {
    _phoneController.dispose();
    super.dispose();
  }

  void _continue() {
    if (!_formKey.currentState!.validate()) return;
    widget.session.setPhoneNumber(_phoneController.text.trim());
    context.go('/verify');
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colors = theme.colorScheme;
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 440),
              child: Form(
                key: _formKey,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _BrandMark(color: colors.primary),
                    const SizedBox(height: 44),
                    Container(
                      width: 64,
                      height: 64,
                      decoration: BoxDecoration(
                        color: AppTheme.translucent(colors.primary, 0.09),
                        borderRadius: BorderRadius.circular(20),
                      ),
                      child: Icon(
                        Icons.route_rounded,
                        color: colors.primary,
                        size: 31,
                      ),
                    ),
                    const SizedBox(height: 24),
                    Text(
                      'One account.\nEvery journey.',
                      style: theme.textTheme.headlineMedium?.copyWith(
                        fontWeight: FontWeight.w900,
                        height: 1.08,
                        letterSpacing: -0.8,
                      ),
                    ),
                    const SizedBox(height: 10),
                    Text(
                      'Sign in or get started with your mobile number.',
                      style: theme.textTheme.bodyLarge?.copyWith(
                        color: colors.onSurfaceVariant,
                      ),
                    ),
                    const SizedBox(height: 30),
                    Text(
                      'MOBILE NUMBER',
                      style: theme.textTheme.labelMedium?.copyWith(
                        fontWeight: FontWeight.w800,
                        letterSpacing: 1.1,
                      ),
                    ),
                    const SizedBox(height: 9),
                    TextFormField(
                      key: const Key('authPhoneField'),
                      controller: _phoneController,
                      autofocus: false,
                      keyboardType: TextInputType.phone,
                      textInputAction: TextInputAction.done,
                      inputFormatters: [
                        FilteringTextInputFormatter.digitsOnly,
                        LengthLimitingTextInputFormatter(10),
                      ],
                      onFieldSubmitted: (_) => _continue(),
                      decoration: const InputDecoration(
                        prefixText: '+91  ',
                        hintText: '98765 43210',
                        border: OutlineInputBorder(),
                      ),
                      validator: (value) {
                        final phone = value?.trim() ?? '';
                        if (!RegExp(r'^[6-9]\d{9}$').hasMatch(phone)) {
                          return 'Enter a valid 10-digit Indian mobile number.';
                        }
                        return null;
                      },
                    ),
                    const SizedBox(height: 15),
                    SizedBox(
                      width: double.infinity,
                      child: FilledButton(
                        key: const Key('authContinueButton'),
                        onPressed: _continue,
                        child: const Text('Continue with phone'),
                      ),
                    ),
                    const SizedBox(height: 19),
                    Container(
                      padding: const EdgeInsets.all(14),
                      decoration: BoxDecoration(
                        color: Colors.white,
                        borderRadius: BorderRadius.circular(16),
                        border: Border.all(color: colors.outlineVariant),
                      ),
                      child: Row(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Icon(Icons.lock_outline, color: colors.primary),
                          const SizedBox(width: 11),
                          Expanded(
                            child: Text(
                              'Phone verification is a local demo. No SMS is sent, and your number is not saved or sent to a server.',
                              style: theme.textTheme.bodySmall?.copyWith(
                                color: colors.onSurfaceVariant,
                                height: 1.4,
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(height: 18),
                    Text(
                      'By continuing, you agree to the demo experience. '
                      'Terms and privacy links must be supplied before launch.',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: colors.onSurfaceVariant,
                        height: 1.4,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _BrandMark extends StatelessWidget {
  const _BrandMark({required this.color});

  final Color color;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(Icons.directions_car_filled_rounded, color: color, size: 24),
        const SizedBox(width: 9),
        Text(
          'SARATHI',
          style: Theme.of(context).textTheme.titleMedium?.copyWith(
                color: color,
                fontWeight: FontWeight.w900,
                letterSpacing: 1.5,
              ),
        ),
        const SizedBox(width: 9),
        const _DemoPill(),
      ],
    );
  }
}

class _DemoPill extends StatelessWidget {
  const _DemoPill();

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 5),
      decoration: BoxDecoration(
        color: const Color(0xFFFFF3D9),
        borderRadius: BorderRadius.circular(20),
      ),
      child: const Text(
        'DEMO',
        style: TextStyle(
          color: Color(0xFF805312),
          fontWeight: FontWeight.w800,
          fontSize: 10,
          letterSpacing: 0.5,
        ),
      ),
    );
  }
}
