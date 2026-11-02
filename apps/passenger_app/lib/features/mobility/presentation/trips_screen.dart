import 'package:flutter/material.dart';

import '../../../core/theme/app_theme.dart';
import 'mobility_widgets.dart';

enum _TripProduct { ride, premium, tour }

class TripsScreen extends StatefulWidget {
  const TripsScreen({required this.onRideBooked, super.key});

  final VoidCallback onRideBooked;

  @override
  State<TripsScreen> createState() => _TripsScreenState();
}

class _TripsScreenState extends State<TripsScreen> {
  _TripProduct _product = _TripProduct.ride;
  bool _pickupAndReturn = false;
  int _seatCount = 5;
  final _pickupLocation = TextEditingController(text: 'Indiranagar, Bengaluru');
  final _dropLocation = TextEditingController(text: 'Airport / destination');
  String _premiumCar = 'Mercedes-Benz E-Class';
  int _hours = 3;
  String _tour = 'Nandi Hills';
  int _days = 2;
  bool _includeStay = false;
  String _dealer = 'Urban Wheels · Indiranagar';
  String _testDriveModel = 'Tata Nexon EV';
  final Set<String> _usedTestDrives = {};
  int _testDriveRadiusKm = 5;
  bool _driveAsCab = true;

  @override
  void dispose() {
    _pickupLocation.dispose();
    _dropLocation.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(20, 16, 20, 32),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const MobilityHeader(
            eyebrow: 'Ride your way',
            title: 'Choose a trip.',
            subtitle: 'From everyday rides to special occasions and getaways.',
          ),
          const SizedBox(height: 20),
          _ProductSelector(
            selected: _product,
            onChanged: (product) => setState(() => _product = product),
          ),
          const SizedBox(height: 16),
          if (_product == _TripProduct.ride) _rideForm(context),
          if (_product == _TripProduct.premium) _premiumForm(context),
          if (_product == _TripProduct.tour) _tourForm(context),
          const SizedBox(height: 18),
          _testDriveForm(context),
          const SizedBox(height: 14),
          const SampleNotice(
            message:
                'Fare estimates and availability are illustrative. No driver is dispatched and no payment is collected.',
          ),
        ],
      ),
    );
  }

  Widget _rideForm(BuildContext context) {
    final baseFare = _seatCount == 5 ? 420 : 620;
    final parking = _pickupAndReturn ? 120 : 0;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AppSurface(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const SectionHeading('Everyday cab'),
              const SizedBox(height: 7),
              Text(
                'Choose capacity, not a car model.',
                style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
              ),
              const SizedBox(height: 15),
              _AddressRow(
                icon: Icons.trip_origin_rounded,
                label: 'START / PICKUP LOCATION',
                hint: 'Enter your pickup location',
                controller: _pickupLocation,
              ),
              const Padding(
                padding: EdgeInsets.only(left: 14),
                child: SizedBox(
                  height: 17,
                  child: VerticalDivider(width: 1, thickness: 1),
                ),
              ),
              _AddressRow(
                icon: Icons.location_on_rounded,
                label: _pickupAndReturn ? 'GUEST PICKUP POINT' : 'DESTINATION',
                hint: _pickupAndReturn
                    ? 'Airport, railway or metro station'
                    : 'Enter your destination',
                controller: _dropLocation,
                destination: true,
              ),
              const SizedBox(height: 16),
              Text(
                'PASSENGER CAPACITY',
                style: Theme.of(context).textTheme.labelSmall?.copyWith(
                      letterSpacing: 0.9,
                      fontWeight: FontWeight.w800,
                    ),
              ),
              const SizedBox(height: 9),
              Row(
                children: [
                  Expanded(
                    child: _ChoiceTile(
                      icon: Icons.directions_car_filled_rounded,
                      title: '5 seater',
                      selected: _seatCount == 5,
                      onTap: () => setState(() => _seatCount = 5),
                    ),
                  ),
                  const SizedBox(width: 9),
                  Expanded(
                    child: _ChoiceTile(
                      icon: Icons.airport_shuttle_rounded,
                      title: '7 seater',
                      selected: _seatCount == 7,
                      onTap: () => setState(() => _seatCount = 7),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 17),
              SwitchListTile.adaptive(
                contentPadding: EdgeInsets.zero,
                value: _pickupAndReturn,
                onChanged: (value) => setState(() => _pickupAndReturn = value),
                title: const Text('Pick someone up and return'),
                subtitle: Text(
                  _pickupAndReturn
                      ? 'Collect a guest at the destination and return; parking is estimated separately.'
                      : 'Go directly to your destination; no parking fee added.',
                ),
              ),
              const Divider(height: 20),
              _FareRow(label: 'Estimated ride fare', amount: baseFare),
              if (_pickupAndReturn)
                const _FareRow(
                  label: 'Estimated parking allowance',
                  amount: 120,
                ),
              const Divider(height: 20),
              _FareRow(
                label: 'Estimated total',
                amount: baseFare + parking,
                emphasize: true,
              ),
              const SizedBox(height: 14),
              SizedBox(
                width: double.infinity,
                child: FilledButton.icon(
                  onPressed: () {
                    if (_pickupLocation.text.trim().isEmpty ||
                        _dropLocation.text.trim().isEmpty) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(
                          content: Text('Enter both trip locations.'),
                        ),
                      );
                      return;
                    }
                    _book(
                      context,
                      'Cab request saved',
                      'Your $_seatCount-seater ${_pickupAndReturn ? 'pickup-and-return' : 'one-way'} request from ${_pickupLocation.text} is a demo. Parking is ${_pickupAndReturn ? 'included as an estimate' : 'not added'}.',
                    );
                  },
                  icon: const Icon(Icons.local_taxi_rounded),
                  label: const Text('Request a cab'),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _premiumForm(BuildContext context) {
    final models = [
      'Mercedes-Benz E-Class',
      'BMW 5 Series',
      'Jaguar XF',
      'Vintage Ambassador',
      'Classic Volkswagen Beetle',
    ];
    final isVintage =
        _premiumCar.contains('Vintage') || _premiumCar.contains('Classic');
    final rate = isVintage ? 3200 : 2400;
    return AppSurface(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SectionHeading('Premium & vintage'),
          const SizedBox(height: 6),
          Text(
            'Choose a specific car for your event or ceremony.',
            style: Theme.of(context).textTheme.bodySmall?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
          ),
          const SizedBox(height: 15),
          DropdownButtonFormField<String>(
            value: _premiumCar,
            isExpanded: true,
            decoration: const InputDecoration(
              labelText: 'CAR MODEL',
              prefixIcon: Icon(Icons.directions_car_filled_outlined),
              border: OutlineInputBorder(),
            ),
            items: models
                .map((model) => DropdownMenuItem(
                      value: model,
                      child: Text(model, overflow: TextOverflow.ellipsis),
                    ))
                .toList(),
            onChanged: (model) {
              if (model != null) setState(() => _premiumCar = model);
            },
          ),
          const SizedBox(height: 18),
          Row(
            children: [
              const Expanded(child: Text('BOOKING DURATION')),
              Text(
                '$_hours hours',
                style: const TextStyle(
                  color: AppTheme.emerald,
                  fontWeight: FontWeight.w800,
                ),
              ),
            ],
          ),
          Slider(
            value: _hours.toDouble(),
            min: 2,
            max: 8,
            divisions: 6,
            label: '$_hours hours',
            onChanged: (value) => setState(() => _hours = value.round()),
          ),
          const Divider(height: 18),
          _FareRow(label: 'Sample rate', amount: rate, suffix: ' / hour'),
          const SizedBox(height: 9),
          _FareRow(
            label: 'Estimated total · $_hours hours',
            amount: rate * _hours,
            emphasize: true,
          ),
          const SizedBox(height: 16),
          SizedBox(
            width: double.infinity,
            child: FilledButton.icon(
              onPressed: () => _book(
                context,
                'Event car request saved',
                'Your $_hours-hour request for $_premiumCar is a demo. Final availability and hourly pricing require a partner fleet.',
              ),
              icon: const Icon(Icons.auto_awesome_rounded),
              label: const Text('Request event car'),
            ),
          ),
        ],
      ),
    );
  }

  Widget _tourForm(BuildContext context) {
    const destinations = [
      'Nandi Hills',
      'Mysuru heritage tour',
      'Coorg getaway',
      'Ooty hill escape',
    ];
    final dailyRate = switch (_tour) {
      'Coorg getaway' => 6800,
      'Ooty hill escape' => 7200,
      'Mysuru heritage tour' => 5200,
      _ => 3900,
    };
    const stayRatePerDay = 2800;
    return AppSurface(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SectionHeading('Tour & stay'),
          const SizedBox(height: 6),
          Text(
            'Plan a getaway with a vehicle for the full duration.',
            style: Theme.of(context).textTheme.bodySmall?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
          ),
          const SizedBox(height: 15),
          DropdownButtonFormField<String>(
            value: _tour,
            isExpanded: true,
            decoration: const InputDecoration(
              labelText: 'DESTINATION',
              prefixIcon: Icon(Icons.landscape_outlined),
              border: OutlineInputBorder(),
            ),
            items: destinations
                .map((destination) => DropdownMenuItem(
                      value: destination,
                      child: Text(destination),
                    ))
                .toList(),
            onChanged: (destination) {
              if (destination != null) setState(() => _tour = destination);
            },
          ),
          const SizedBox(height: 18),
          Row(
            children: [
              const Expanded(child: Text('DURATION')),
              Text(
                '$_days ${_days == 1 ? 'day' : 'days'}',
                style: const TextStyle(
                  color: AppTheme.emerald,
                  fontWeight: FontWeight.w800,
                ),
              ),
            ],
          ),
          Slider(
            value: _days.toDouble(),
            min: 1,
            max: 7,
            divisions: 6,
            label: '$_days days',
            onChanged: (value) => setState(() => _days = value.round()),
          ),
          CheckboxListTile(
            contentPadding: EdgeInsets.zero,
            value: _includeStay,
            onChanged: (value) => setState(() => _includeStay = value ?? false),
            title: const Text('Include a sample overnight stay'),
            subtitle: const Text('Illustrative hotel add-on · ₹2,800 / day'),
            controlAffinity: ListTileControlAffinity.leading,
          ),
          const Divider(height: 18),
          _FareRow(
            label: 'Sample package rate',
            amount: dailyRate,
            suffix: ' / day',
          ),
          if (_includeStay)
            const _FareRow(
              label: 'Sample lodging add-on',
              amount: stayRatePerDay,
              suffix: ' / day',
            ),
          const SizedBox(height: 9),
          _FareRow(
            label: 'Estimated trip total',
            amount: (dailyRate + (_includeStay ? stayRatePerDay : 0)) * _days,
            emphasize: true,
          ),
          const SizedBox(height: 16),
          SizedBox(
            width: double.infinity,
            child: FilledButton.icon(
              onPressed: () => _book(
                context,
                'Tour request saved',
                'Your $_days-day $_tour tour request is a demo. ${_includeStay ? 'An illustrative stay was included.' : 'Lodging was not included.'} Final fare and accommodation are not booked.',
              ),
              icon: const Icon(Icons.luggage_rounded),
              label: const Text('Plan this tour'),
            ),
          ),
        ],
      ),
    );
  }

  Widget _testDriveForm(BuildContext context) {
    const showrooms = [
      'Urban Wheels · Indiranagar',
      'City Auto Mall · Whitefield',
    ];
    const models = [
      'Tata Nexon EV',
      'Hyundai Creta',
      'Maruti Fronx',
      'Honda Elevate',
    ];
    final bookingKey = '$_dealer|$_testDriveModel';
    final alreadyUsed = _usedTestDrives.contains(bookingKey);
    return AppSurface(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SectionHeading('Test-drive ride'),
          const SizedBox(height: 5),
          Text(
            'Try a dealer vehicle in a defined area—or request it as a demo cab.',
            style: Theme.of(context).textTheme.bodySmall?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
          ),
          const SizedBox(height: 14),
          DropdownButtonFormField<String>(
            value: _dealer,
            isExpanded: true,
            decoration: const InputDecoration(
              labelText: 'PARTNER SHOWROOM',
              prefixIcon: Icon(Icons.storefront_outlined),
              border: OutlineInputBorder(),
            ),
            items: showrooms
                .map((showroom) => DropdownMenuItem(
                      value: showroom,
                      child: Text(showroom, overflow: TextOverflow.ellipsis),
                    ))
                .toList(),
            onChanged: (showroom) {
              if (showroom != null) setState(() => _dealer = showroom);
            },
          ),
          const SizedBox(height: 11),
          DropdownButtonFormField<String>(
            value: _testDriveModel,
            isExpanded: true,
            decoration: const InputDecoration(
              labelText: 'CAR MODEL',
              prefixIcon: Icon(Icons.directions_car_filled_outlined),
              border: OutlineInputBorder(),
            ),
            items: models
                .map((model) => DropdownMenuItem(
                      value: model,
                      child: Text(model),
                    ))
                .toList(),
            onChanged: (model) {
              if (model != null) setState(() => _testDriveModel = model);
            },
          ),
          const SizedBox(height: 11),
          SwitchListTile.adaptive(
            contentPadding: EdgeInsets.zero,
            value: _driveAsCab,
            onChanged: (value) => setState(() => _driveAsCab = value),
            title: const Text('Offer this as a chauffeured demo cab'),
            subtitle: const Text('A licensed driver is required'),
          ),
          Row(
            children: [
              const Expanded(child: Text('Service area radius')),
              Text(
                '$_testDriveRadiusKm km',
                style: const TextStyle(
                  color: AppTheme.emerald,
                  fontWeight: FontWeight.w800,
                ),
              ),
            ],
          ),
          Slider(
            value: _testDriveRadiusKm.toDouble(),
            min: 3,
            max: 10,
            divisions: 7,
            label: '$_testDriveRadiusKm km',
            onChanged: (value) =>
                setState(() => _testDriveRadiusKm = value.round()),
          ),
          if (alreadyUsed) ...[
            const SampleNotice(
              icon: Icons.block_rounded,
              message:
                  'This model has already been used for a test drive at this showroom in this demo.',
            ),
            const SizedBox(height: 9),
          ],
          SizedBox(
            width: double.infinity,
            child: OutlinedButton.icon(
              onPressed: alreadyUsed
                  ? null
                  : () {
                      setState(() => _usedTestDrives.add(bookingKey));
                      _book(
                        context,
                        'Test-drive request saved',
                        'Your $_testDriveModel request at $_dealer is limited to a $_testDriveRadiusKm km area. ${_driveAsCab ? 'Chauffeured demo-cab mode is selected.' : 'Customer test-drive mode is selected.'} One demo booking per model and showroom.',
                      );
                    },
              icon: const Icon(Icons.key_rounded),
              label: Text(
                alreadyUsed
                    ? 'Model test-drive already used'
                    : 'Book area-limited test drive',
              ),
            ),
          ),
        ],
      ),
    );
  }

  void _book(BuildContext context, String title, String detail) {
    widget.onRideBooked();
    showDemoConfirmation(context, title: title, detail: detail);
  }
}

class _ProductSelector extends StatelessWidget {
  const _ProductSelector({required this.selected, required this.onChanged});
  final _TripProduct selected;
  final ValueChanged<_TripProduct> onChanged;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        for (final (product, label, icon) in [
          (_TripProduct.ride, 'Everyday', Icons.local_taxi_rounded),
          (_TripProduct.premium, 'Premium', Icons.workspace_premium_rounded),
          (_TripProduct.tour, 'Tour', Icons.landscape_rounded),
        ])
          Expanded(
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 3),
              child: _ChoiceTile(
                icon: icon,
                title: label,
                selected: selected == product,
                onTap: () => onChanged(product),
              ),
            ),
          ),
      ],
    );
  }
}

class _ChoiceTile extends StatelessWidget {
  const _ChoiceTile({
    required this.icon,
    required this.title,
    required this.selected,
    required this.onTap,
  });

  final IconData icon;
  final String title;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final color = selected
        ? Theme.of(context).colorScheme.primary
        : Theme.of(context).colorScheme.onSurfaceVariant;
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(14),
      child: Container(
        constraints: const BoxConstraints(minHeight: 75),
        padding: const EdgeInsets.symmetric(vertical: 11, horizontal: 5),
        decoration: BoxDecoration(
          color: selected ? const Color(0xFFEAF2EF) : Colors.white,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(
            color: selected
                ? Theme.of(context).colorScheme.primary
                : Theme.of(context).colorScheme.outlineVariant,
          ),
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, size: 20, color: color),
            const SizedBox(height: 5),
            Text(
              title,
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.labelSmall?.copyWith(
                    color: color,
                    fontWeight: FontWeight.w800,
                  ),
            ),
          ],
        ),
      ),
    );
  }
}

class _AddressRow extends StatelessWidget {
  const _AddressRow({
    required this.icon,
    required this.label,
    required this.hint,
    required this.controller,
    this.destination = false,
  });
  final IconData icon;
  final String label;
  final String hint;
  final TextEditingController controller;
  final bool destination;

  @override
  Widget build(BuildContext context) {
    final color = destination ? const Color(0xFFC44D42) : AppTheme.emerald;
    return TextField(
      controller: controller,
      textInputAction: TextInputAction.next,
      decoration: InputDecoration(
        labelText: label,
        hintText: hint,
        prefixIcon: Icon(icon, color: color),
        border: const OutlineInputBorder(),
      ),
    );
  }
}

class _FareRow extends StatelessWidget {
  const _FareRow({
    required this.label,
    required this.amount,
    this.suffix,
    this.emphasize = false,
  });

  final String label;
  final int amount;
  final String? suffix;
  final bool emphasize;

  @override
  Widget build(BuildContext context) {
    final style = emphasize
        ? Theme.of(context).textTheme.titleSmall?.copyWith(
              fontWeight: FontWeight.w900,
            )
        : Theme.of(context).textTheme.bodySmall?.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            );
    return Row(
      children: [
        Expanded(child: Text(label, style: style)),
        PriceText(amount, suffix: suffix),
      ],
    );
  }
}
