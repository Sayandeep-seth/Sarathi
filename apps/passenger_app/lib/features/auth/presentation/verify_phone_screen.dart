import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';

import '../../../core/theme/app_theme.dart';
import '../data/demo_auth_session.dart';

class VerifyPhoneScreen extends StatefulWidget {
  const VerifyPhoneScreen({required this.session, super.key});

  final DemoAuthSession session;

  @override
  State<VerifyPhoneScreen> createState() => _VerifyPhoneScreenState();
}

class _VerifyPhoneScreenState extends State<VerifyPhoneScreen> {
  final _codeController = TextEditingController();
  String? _error;

  @override
  void dispose() {
    _codeController.dispose();
    super.dispose();
  }

  void _verify() {
    final code = _codeController.text.trim();
    if (!RegExp(r'^\d{6}$').hasMatch(code)) {
      setState(() => _error = 'Enter the 6-digit demo code.');
      return;
    }
    if (!widget.session.verifyCode(code)) {
      setState(() => _error = 'That code is not correct. Try the demo code.');
      return;
    }
    context.go('/home');
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colors = theme.colorScheme;
    final phone = widget.session.phoneNumber ?? '';
    return Scaffold(
      appBar: AppBar(
        leading: IconButton(
          tooltip: 'Change phone number',
          onPressed: () => context.go('/auth'),
          icon: const Icon(Icons.arrow_back_rounded),
        ),
        title: const Text('Verify number'),
        backgroundColor: Theme.of(context).scaffoldBackgroundColor,
      ),
      body: SafeArea(
        top: false,
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.fromLTRB(24, 20, 24, 32),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 440),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Container(
                    width: 64,
                    height: 64,
                    decoration: BoxDecoration(
                      color: AppTheme.translucent(colors.primary, 0.09),
                      borderRadius: BorderRadius.circular(20),
                    ),
                    child: Icon(
                      Icons.mark_email_read_outlined,
                      color: colors.primary,
                      size: 30,
                    ),
                  ),
                  const SizedBox(height: 24),
                  Text(
                    'Verify it’s you.',
                    style: theme.textTheme.headlineMedium?.copyWith(
                      fontWeight: FontWeight.w900,
                      letterSpacing: -0.8,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    'Enter the 6-digit code for +91 $phone.',
                    style: theme.textTheme.bodyLarge?.copyWith(
                      color: colors.onSurfaceVariant,
                    ),
                  ),
                  const SizedBox(height: 26),
                  TextField(
                    key: const Key('authCodeField'),
                    controller: _codeController,
                    autofocus: true,
                    keyboardType: TextInputType.number,
                    textInputAction: TextInputAction.done,
                    inputFormatters: [
                      FilteringTextInputFormatter.digitsOnly,
                      LengthLimitingTextInputFormatter(6),
                    ],
                    onSubmitted: (_) => _verify(),
                    decoration: InputDecoration(
                      labelText: 'Verification code',
                      hintText: '6 digits',
                      prefixIcon: const Icon(Icons.password_rounded),
                      errorText: _error,
                    ),
                  ),
                  const SizedBox(height: 14),
                  Container(
                    width: double.infinity,
                    padding: const EdgeInsets.all(16),
                    decoration: BoxDecoration(
                      color: const Color(0xFFEAF2EF),
                      borderRadius: BorderRadius.circular(17),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Wrap(
                          spacing: 8,
                          runSpacing: 6,
                          crossAxisAlignment: WrapCrossAlignment.center,
                          children: [
                            Icon(Icons.science_outlined,
                                color: colors.primary, size: 19),
                            Text(
                              'DEMO CODE · NO SMS SENT',
                              style: theme.textTheme.labelSmall?.copyWith(
                                color: colors.primary,
                                fontWeight: FontWeight.w900,
                                letterSpacing: 0.7,
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 8),
                        Text(
                          DemoAuthSession.demoCode,
                          style: theme.textTheme.headlineSmall?.copyWith(
                            fontWeight: FontWeight.w900,
                            letterSpacing: 5,
                          ),
                        ),
                        const SizedBox(height: 5),
                        Text(
                          'This public test code is only for previewing the app. It does not verify a real phone number.',
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: colors.onSurfaceVariant,
                            height: 1.4,
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 19),
                  SizedBox(
                    width: double.infinity,
                    child: FilledButton(
                      key: const Key('authVerifyButton'),
                      onPressed: _verify,
                      child: const Text('Verify and continue'),
                    ),
                  ),
                  const SizedBox(height: 12),
                  Center(
                    child: TextButton(
                      onPressed: () {
                        _codeController.clear();
                        setState(() => _error = null);
                        ScaffoldMessenger.of(context).showSnackBar(
                          const SnackBar(
                            content: Text(
                              'Demo code is shown above. No message was sent.',
                            ),
                          ),
                        );
                      },
                      child: const Text('Show demo code again'),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
