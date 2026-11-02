import 'package:flutter/material.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../../../core/theme/app_theme.dart';
import '../data/mobility_catalog.dart';
import 'mobility_widgets.dart';

class CommuteScreen extends StatefulWidget {
  const CommuteScreen({
    required this.savedRoute,
    super.key,
  });

  final SavedRoute? savedRoute;

  @override
  State<CommuteScreen> createState() => _CommuteScreenState();
}

class _CommuteScreenState extends State<CommuteScreen> {
  static const _sampleFares = {
    'Train': 40,
    'Metro': 30,
    'Bus': 15,
    'Cab': 85,
  };

  late final TextEditingController _origin;
  late final TextEditingController _destination;
  final Set<String> _transitModes = {'Metro', 'Bus'};
  bool _showQr = false;
  bool _reservedSeat = false;

  int get _sampleFare => _transitModes.fold(
        0,
        (total, mode) => total + (_sampleFares[mode] ?? 0),
      );

  @override
  void initState() {
    super.initState();
    _origin = TextEditingController(
      text: widget.savedRoute?.origin ?? 'Indiranagar',
    );
    _destination = TextEditingController(
      text: widget.savedRoute?.destination ?? 'Tech Park',
    );
  }

  @override
  void didUpdateWidget(covariant CommuteScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.savedRoute != oldWidget.savedRoute &&
        widget.savedRoute != null) {
      _origin.text = widget.savedRoute!.origin;
      _destination.text = widget.savedRoute!.destination;
      _showQr = false;
    }
  }

  @override
  void dispose() {
    _origin.dispose();
    _destination.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(20, 16, 20, 32),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const MobilityHeader(
            eyebrow: 'For your workday',
            title: 'Daily commute.',
            subtitle:
                'Save a regular route and preview a multi-mode journey pass.',
          ),
          const SizedBox(height: 20),
          AppSurface(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    const Expanded(child: SectionHeading('Your regular route')),
                    if (widget.savedRoute != null) const _SavedTag(),
                  ],
                ),
                const SizedBox(height: 14),
                _CommuteField(
                  controller: _origin,
                  label: 'FROM',
                  icon: Icons.trip_origin_rounded,
                ),
                const SizedBox(height: 10),
                _CommuteField(
                  controller: _destination,
                  label: 'TO',
                  icon: Icons.location_on_rounded,
                ),
                const SizedBox(height: 17),
                Text(
                  'TRANSIT MODES',
                  style: theme.textTheme.labelSmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    letterSpacing: 0.9,
                    fontWeight: FontWeight.w800,
                  ),
                ),
                const SizedBox(height: 8),
                Wrap(
                  spacing: 7,
                  children: [
                    for (final mode in ['Train', 'Metro', 'Bus', 'Cab'])
                      FilterChip(
                        label: Text(mode),
                        selected: _transitModes.contains(mode),
                        onSelected: (selected) {
                          setState(() {
                            if (selected) {
                              _transitModes.add(mode);
                            } else if (_transitModes.length > 1) {
                              _transitModes.remove(mode);
                            }
                          });
                        },
                      ),
                  ],
                ),
                const SizedBox(height: 10),
                Container(
                  padding: const EdgeInsets.all(14),
                  decoration: BoxDecoration(
                    color: const Color(0xFFEAF2EF),
                    borderRadius: BorderRadius.circular(15),
                  ),
                  child: Row(
                    children: [
                      const Icon(Icons.payments_outlined,
                          color: AppTheme.emerald),
                      const SizedBox(width: 10),
                      const Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              'Whole-route fare estimate',
                              style: TextStyle(fontWeight: FontWeight.w700),
                            ),
                            Text(
                              'Sample total for selected connections',
                              style: TextStyle(fontSize: 11),
                            ),
                          ],
                        ),
                      ),
                      Text(
                        '₹$_sampleFare',
                        style: theme.textTheme.titleLarge?.copyWith(
                          color: AppTheme.emerald,
                          fontWeight: FontWeight.w900,
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 13),
                SizedBox(
                  width: double.infinity,
                  child: FilledButton.icon(
                    onPressed: _transitModes.isEmpty
                        ? null
                        : () {
                            if (_origin.text.trim().isEmpty ||
                                _destination.text.trim().isEmpty) {
                              ScaffoldMessenger.of(context).showSnackBar(
                                const SnackBar(
                                  content: Text('Enter both route locations.'),
                                ),
                              );
                              return;
                            }
                            setState(() => _showQr = true);
                          },
                    icon: const Icon(Icons.qr_code_2_rounded),
                    label: const Text('Preview demo trip QR'),
                  ),
                ),
              ],
            ),
          ),
          if (_showQr) ...[
            const SizedBox(height: 14),
            AppSurface(
              child: Column(
                children: [
                  const SectionHeading('Your demo trip QR'),
                  const SizedBox(height: 8),
                  Text(
                    '${_origin.text}  →  ${_destination.text}',
                    textAlign: TextAlign.center,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  const SizedBox(height: 14),
                  Container(
                    padding: const EdgeInsets.all(14),
                    decoration: BoxDecoration(
                      color: Colors.white,
                      borderRadius: BorderRadius.circular(20),
                      border:
                          Border.all(color: theme.colorScheme.outlineVariant),
                    ),
                    child: QrImageView(
                      data: [
                        'SARATHI-DEMO',
                        _origin.text.trim(),
                        _destination.text.trim(),
                        _transitModes.join(','),
                        'UNPAID-NOT-VALID-FOR-BOARDING',
                      ].join('|'),
                      version: QrVersions.auto,
                      size: 190,
                      backgroundColor: Colors.white,
                    ),
                  ),
                  const SizedBox(height: 13),
                  const SampleNotice(
                    icon: Icons.lock_outline_rounded,
                    message:
                        'Preview only. This QR is not a ticket, payment receipt or valid boarding pass. Fare payment and transit-operator scanning are not connected.',
                  ),
                ],
              ),
            ),
          ],
          const SizedBox(height: 18),
          AppSurface(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Container(
                      width: 42,
                      height: 42,
                      decoration: BoxDecoration(
                        color: const Color(0xFFEAF2EF),
                        borderRadius: BorderRadius.circular(14),
                      ),
                      child: const Icon(
                        Icons.airport_shuttle_rounded,
                        color: AppTheme.emerald,
                      ),
                    ),
                    const SizedBox(width: 11),
                    const Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            'Reserved-seat work bus',
                            style: TextStyle(fontWeight: FontWeight.w800),
                          ),
                          SizedBox(height: 3),
                          Text(
                            'A comfortable commute, without standing.',
                            style: TextStyle(fontSize: 12),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 13),
                const _BenefitRow(
                  icon: Icons.event_seat_rounded,
                  text: 'Seat reservation for every passenger',
                ),
                const _BenefitRow(
                  icon: Icons.business_rounded,
                  text: 'Workplace drop-off when the route allows',
                ),
                const _BenefitRow(
                  icon: Icons.savings_outlined,
                  text: '₹160/day sample vs ₹420 one-way cab estimate',
                ),
                SwitchListTile.adaptive(
                  contentPadding: EdgeInsets.zero,
                  value: _reservedSeat,
                  onChanged: (value) => setState(() => _reservedSeat = value),
                  title: const Text('Join the commute interest list'),
                  subtitle: const Text('No seat is actually reserved yet'),
                ),
                if (_reservedSeat)
                  SizedBox(
                    width: double.infinity,
                    child: OutlinedButton(
                      onPressed: () {
                        showDemoConfirmation(
                          context,
                          title: 'Interest saved',
                          detail:
                              'Your interest in a reserved-seat bus is saved for this demo only. A real route, operator and seat are not assigned.',
                        );
                      },
                      child: const Text('Save my interest'),
                    ),
                  ),
              ],
            ),
          ),
          const SizedBox(height: 14),
          const SampleNotice(
            message:
                'Saved route and pass preview stay on this device during this session. Daily payments, seat inventory and transit integrations are not connected.',
          ),
        ],
      ),
    );
  }
}

class _CommuteField extends StatelessWidget {
  const _CommuteField({
    required this.controller,
    required this.label,
    required this.icon,
  });

  final TextEditingController controller;
  final String label;
  final IconData icon;

  @override
  Widget build(BuildContext context) {
    return TextField(
      controller: controller,
      decoration: InputDecoration(
        labelText: label,
        prefixIcon: Icon(icon),
        border: const OutlineInputBorder(),
      ),
    );
  }
}

class _SavedTag extends StatelessWidget {
  const _SavedTag();

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
        decoration: BoxDecoration(
          color: const Color(0xFFE9F4EC),
          borderRadius: BorderRadius.circular(20),
        ),
        child: const Text(
          'SAVED FROM MAP',
          style: TextStyle(
            color: AppTheme.emerald,
            fontSize: 9,
            fontWeight: FontWeight.w800,
          ),
        ),
      );
}

class _BenefitRow extends StatelessWidget {
  const _BenefitRow({required this.icon, required this.text});
  final IconData icon;
  final String text;

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.only(bottom: 9),
        child: Row(
          children: [
            Icon(icon, size: 17, color: AppTheme.emerald),
            const SizedBox(width: 9),
            Expanded(child: Text(text, style: const TextStyle(fontSize: 12))),
          ],
        ),
      );
}
