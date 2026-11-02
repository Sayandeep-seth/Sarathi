import 'package:flutter/material.dart';

import '../../../core/theme/app_theme.dart';
import '../data/mobility_catalog.dart';
import 'mobility_widgets.dart';

class ExploreScreen extends StatefulWidget {
  const ExploreScreen({required this.onSaveDailyRoute, super.key});

  final ValueChanged<SavedRoute> onSaveDailyRoute;

  @override
  State<ExploreScreen> createState() => _ExploreScreenState();
}

class _ExploreScreenState extends State<ExploreScreen> {
  final _origin = TextEditingController(text: 'Indiranagar, Bengaluru');
  final _destination = TextEditingController(text: 'Cubbon Park, Bengaluru');
  final _energy = TextEditingController(text: '20');
  VehicleSpec _vehicle = MobilityCatalog.vehicles.first;
  bool _showResults = false;

  @override
  void dispose() {
    _origin.dispose();
    _destination.dispose();
    _energy.dispose();
    super.dispose();
  }

  double get _energyAmount =>
      double.tryParse(_energy.text.trim())?.clamp(
        0,
        _vehicle.electric ? 100 : _vehicle.fuelTankLiters,
      ) ??
      0;

  void _updateEnergy(String value) {
    final amount = double.tryParse(value);
    final capacity = _vehicle.electric ? 100.0 : _vehicle.fuelTankLiters;
    if (amount != null && (amount < 0 || amount > capacity)) {
      final normalized = amount.clamp(0, capacity).toStringAsFixed(0);
      _energy.value = TextEditingValue(
        text: normalized,
        selection: TextSelection.collapsed(offset: normalized.length),
      );
    }
    setState(() {});
  }

  String get _rangeEstimate {
    final km = _estimatedRangeKm;
    return '${km.round()} km estimated range';
  }

  double get _estimatedRangeKm => _vehicle.electric
      ? _vehicle.rangePerChargeKm * _energyAmount / 100
      : _vehicle.rangePerLiter * _energyAmount;

  void _findRoutes() {
    if (_origin.text.trim().isEmpty || _destination.text.trim().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
            content: Text('Enter both a starting point and destination.')),
      );
      return;
    }
    FocusScope.of(context).unfocus();
    setState(() => _showResults = true);
  }

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    final stations = _vehicle.electric
        ? MobilityCatalog.evStations
        : MobilityCatalog.petrolStations;

    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(20, 16, 20, 32),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const MobilityHeader(
            eyebrow: 'Plan your journey',
            title: 'Find your way.',
            subtitle:
                'Compare sample routes, road widths and nearby energy stops.',
          ),
          const SizedBox(height: 20),
          const _RouteMapPreview(),
          const SizedBox(height: 18),
          AppSurface(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SectionHeading('Journey details'),
                const SizedBox(height: 14),
                _PlaceField(
                  label: 'STARTING LOCATION',
                  controller: _origin,
                  icon: Icons.trip_origin_rounded,
                ),
                const SizedBox(height: 10),
                _PlaceField(
                  label: 'DESTINATION',
                  controller: _destination,
                  icon: Icons.location_on_rounded,
                ),
                const SizedBox(height: 13),
                Text(
                  'CAR MODEL',
                  style: text.labelSmall?.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                    letterSpacing: 0.9,
                    fontWeight: FontWeight.w800,
                  ),
                ),
                const SizedBox(height: 7),
                DropdownButtonFormField<VehicleSpec>(
                  value: _vehicle,
                  isExpanded: true,
                  decoration: const InputDecoration(
                    prefixIcon: Icon(Icons.directions_car_filled_outlined),
                    border: OutlineInputBorder(),
                    contentPadding: EdgeInsets.symmetric(
                      horizontal: 12,
                      vertical: 13,
                    ),
                  ),
                  items: MobilityCatalog.vehicles
                      .map(
                        (vehicle) => DropdownMenuItem(
                          value: vehicle,
                          child: Text(
                            vehicle.name,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                      )
                      .toList(),
                  onChanged: (vehicle) {
                    if (vehicle == null) return;
                    setState(() {
                      _vehicle = vehicle;
                      _energy.text = vehicle.electric ? '80' : '20';
                    });
                  },
                ),
                const SizedBox(height: 10),
                Container(
                  width: double.infinity,
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: const Color(0xFFF4F7F5),
                    borderRadius: BorderRadius.circular(14),
                  ),
                  child: Row(
                    children: [
                      const Icon(
                        Icons.straighten_rounded,
                        color: AppTheme.emerald,
                      ),
                      const SizedBox(width: 9),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              'ESTIMATED EXTERIOR · L × W × H',
                              style: text.labelSmall?.copyWith(
                                letterSpacing: 0.4,
                                fontWeight: FontWeight.w800,
                              ),
                            ),
                            const SizedBox(height: 3),
                            Text(
                              '${_vehicle.lengthMeters.toStringAsFixed(3)} × '
                              '${_vehicle.widthMeters.toStringAsFixed(3)} × '
                              '${_vehicle.heightMeters.toStringAsFixed(3)} m',
                              style: text.bodySmall?.copyWith(
                                color: Theme.of(context)
                                    .colorScheme
                                    .onSurfaceVariant,
                                fontWeight: FontWeight.w700,
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 13),
                LayoutBuilder(
                  builder: (context, constraints) {
                    final label = _vehicle.electric
                        ? 'BATTERY CHARGE (%)'
                        : 'FUEL ON BOARD · MAX ${_vehicle.fuelTankLiters.round()} L';
                    final range = Text(
                      _rangeEstimate,
                      style: text.labelSmall?.copyWith(
                        color: AppTheme.emerald,
                        fontWeight: FontWeight.w800,
                      ),
                    );
                    final fieldLabel = Text(
                      label,
                      style: text.labelSmall?.copyWith(
                        color: Theme.of(context).colorScheme.onSurfaceVariant,
                        letterSpacing: 0.9,
                        fontWeight: FontWeight.w800,
                      ),
                    );
                    if (constraints.maxWidth < 300) {
                      return Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          fieldLabel,
                          const SizedBox(height: 3),
                          Align(alignment: Alignment.centerRight, child: range),
                        ],
                      );
                    }
                    return Row(
                      children: [
                        Expanded(child: fieldLabel),
                        range,
                      ],
                    );
                  },
                ),
                const SizedBox(height: 7),
                TextField(
                  controller: _energy,
                  keyboardType: const TextInputType.numberWithOptions(
                    decimal: true,
                  ),
                  decoration: InputDecoration(
                    prefixIcon: Icon(
                      _vehicle.electric
                          ? Icons.battery_charging_full_rounded
                          : Icons.local_gas_station_rounded,
                    ),
                    suffixText: _vehicle.electric ? '%' : 'L',
                    border: const OutlineInputBorder(),
                    contentPadding: const EdgeInsets.symmetric(
                      horizontal: 12,
                      vertical: 13,
                    ),
                  ),
                  onChanged: _updateEnergy,
                ),
                const SizedBox(height: 14),
                SizedBox(
                  width: double.infinity,
                  child: FilledButton.icon(
                    onPressed: _findRoutes,
                    icon: const Icon(Icons.alt_route_rounded),
                    label: const Text('Show route options'),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),
          SampleNotice(
            message:
                'Map, traffic, road widths, vehicle specs (model years may differ) and station activity below are sample data. '
                'They are not live navigation or GPS measurements.',
          ),
          if (_showResults) ...[
            const SizedBox(height: 24),
            SectionHeading(
              'Road options',
              trailing: Text(
                '${MobilityCatalog.routes.length} routes',
                style: text.labelMedium?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
              ),
            ),
            const SizedBox(height: 12),
            for (var index = 0;
                index < MobilityCatalog.routes.length;
                index++) ...[
              _RoadOptionCard(
                route: MobilityCatalog.routes[index],
                vehicle: _vehicle,
                energyRangeKm: _estimatedRangeKm,
                recommended: index == 0,
              ),
              const SizedBox(height: 10),
            ],
            OutlinedButton.icon(
              onPressed: () => widget.onSaveDailyRoute(
                SavedRoute(
                  origin: _origin.text.trim(),
                  destination: _destination.text.trim(),
                ),
              ),
              icon: const Icon(Icons.bookmark_add_outlined),
              label: const Text('Save as daily commute'),
            ),
            const SizedBox(height: 24),
            SectionHeading(
              _vehicle.electric
                  ? 'EV charging on the way'
                  : 'Petrol stops nearby',
              trailing: TextButton(
                onPressed: () => _showStationSort(context),
                child: const Text('Top rated'),
              ),
            ),
            const SizedBox(height: 11),
            for (final station in stations) ...[
              _StationCard(station: station),
              const SizedBox(height: 10),
            ],
          ],
        ],
      ),
    );
  }

  void _showStationSort(BuildContext context) {
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        content: Text(
          'Demo ratings use sample visit counts, not live GPS or verified reviews.',
        ),
      ),
    );
  }
}

class _PlaceField extends StatelessWidget {
  const _PlaceField({
    required this.label,
    required this.controller,
    required this.icon,
  });

  final String label;
  final TextEditingController controller;
  final IconData icon;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: Theme.of(context).textTheme.labelSmall?.copyWith(
                color: Theme.of(context).colorScheme.onSurfaceVariant,
                letterSpacing: 0.9,
                fontWeight: FontWeight.w800,
              ),
        ),
        const SizedBox(height: 7),
        TextField(
          controller: controller,
          textInputAction: TextInputAction.next,
          decoration: InputDecoration(
            prefixIcon: Icon(icon),
            border: const OutlineInputBorder(),
            contentPadding: const EdgeInsets.symmetric(
              horizontal: 12,
              vertical: 13,
            ),
          ),
        ),
      ],
    );
  }
}

class _RouteMapPreview extends StatelessWidget {
  const _RouteMapPreview();

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 190,
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: const Color(0xFFEAF1E9),
        borderRadius: BorderRadius.circular(22),
        border: Border.all(color: Theme.of(context).colorScheme.outlineVariant),
      ),
      child: Stack(
        fit: StackFit.expand,
        children: [
          const CustomPaint(painter: _SampleMapPainter()),
          Positioned(
            left: 18,
            top: 16,
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 7),
              decoration: BoxDecoration(
                color: Colors.white,
                borderRadius: BorderRadius.circular(20),
                boxShadow: const [
                  BoxShadow(color: Color(0x18000000), blurRadius: 10),
                ],
              ),
              child: const Row(
                children: [
                  Icon(Icons.near_me_rounded,
                      size: 15, color: AppTheme.emerald),
                  SizedBox(width: 6),
                  Text(
                    'ROUTE PREVIEW',
                    style: TextStyle(
                      fontSize: 10,
                      letterSpacing: 0.8,
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                ],
              ),
            ),
          ),
          const Positioned(
            left: 44,
            bottom: 31,
            child: _MapPin(label: 'START', color: AppTheme.emerald),
          ),
          const Positioned(
            right: 41,
            top: 43,
            child: _MapPin(label: 'DESTINATION', color: Color(0xFFC44D42)),
          ),
        ],
      ),
    );
  }
}

class _MapPin extends StatelessWidget {
  const _MapPin({required this.label, required this.color});

  final String label;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(Icons.location_on_rounded, color: color, size: 20),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 4),
          decoration: BoxDecoration(
            color: Colors.white,
            borderRadius: BorderRadius.circular(6),
          ),
          child: Text(
            label,
            style: const TextStyle(fontSize: 8, fontWeight: FontWeight.w800),
          ),
        ),
      ],
    );
  }
}

class _SampleMapPainter extends CustomPainter {
  const _SampleMapPainter();

  @override
  void paint(Canvas canvas, Size size) {
    final green = Paint()..color = const Color(0xFFDCE9D9);
    final greenAlt = Paint()..color = const Color(0xFFE3ECD9);
    for (var row = 0; row < 3; row++) {
      for (var col = 0; col < 5; col++) {
        final x = 14.0 + col * 75;
        final y = 16.0 + row * 61;
        final rect = RRect.fromRectAndRadius(
          Rect.fromLTWH(x, y, 42 + (col % 2) * 13, 30),
          const Radius.circular(7),
        );
        canvas.drawRRect(rect, (row + col).isEven ? green : greenAlt);
      }
    }
    final minor = Paint()
      ..color = Colors.white
      ..strokeWidth = 6
      ..style = PaintingStyle.stroke
      ..strokeCap = StrokeCap.round;
    final minorRoads = [
      Path()
        ..moveTo(0, size.height * 0.36)
        ..cubicTo(80, 60, 170, 102, size.width, 68),
      Path()
        ..moveTo(size.width * 0.23, 0)
        ..cubicTo(62, 70, 111, 130, 94, size.height),
      Path()
        ..moveTo(size.width * 0.74, 0)
        ..cubicTo(176, 75, 235, 113, 226, size.height),
    ];
    for (final road in minorRoads) {
      canvas.drawPath(road, minor);
    }

    final route = Paint()
      ..color = AppTheme.emerald
      ..strokeWidth = 5
      ..style = PaintingStyle.stroke
      ..strokeCap = StrokeCap.round
      ..strokeJoin = StrokeJoin.round;
    final path = Path()
      ..moveTo(size.width * 0.14, size.height * 0.78)
      ..cubicTo(size.width * 0.25, size.height * 0.73, size.width * 0.29,
          size.height * 0.44, size.width * 0.42, size.height * 0.52)
      ..cubicTo(size.width * 0.58, size.height * 0.61, size.width * 0.62,
          size.height * 0.24, size.width * 0.82, size.height * 0.29);
    canvas.drawPath(path, route);
  }

  @override
  bool shouldRepaint(covariant _SampleMapPainter oldDelegate) => false;
}

class _RoadOptionCard extends StatelessWidget {
  const _RoadOptionCard({
    required this.route,
    required this.vehicle,
    required this.energyRangeKm,
    required this.recommended,
  });

  final SampleRoute route;
  final VehicleSpec vehicle;
  final double energyRangeKm;
  final bool recommended;

  @override
  Widget build(BuildContext context) {
    final fits = route.canPass(vehicle);
    final enoughEnergy = energyRangeKm >= route.distanceKm;
    return AppSurface(
      padding: const EdgeInsets.all(15),
      child: Column(
        children: [
          Row(
            children: [
              Container(
                width: 43,
                height: 43,
                decoration: BoxDecoration(
                  color: AppTheme.translucent(route.color, 0.12),
                  borderRadius: BorderRadius.circular(14),
                ),
                child: Icon(Icons.alt_route_rounded, color: route.color),
              ),
              const SizedBox(width: 11),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Flexible(
                          child: Text(
                            route.name,
                            overflow: TextOverflow.ellipsis,
                            style: Theme.of(context)
                                .textTheme
                                .titleSmall
                                ?.copyWith(fontWeight: FontWeight.w800),
                          ),
                        ),
                        if (recommended) ...[
                          const SizedBox(width: 7),
                          const _MiniTag(label: 'SUGGESTED'),
                        ],
                      ],
                    ),
                    const SizedBox(height: 3),
                    Text(
                      route.roadType,
                      style: Theme.of(context).textTheme.bodySmall?.copyWith(
                            color:
                                Theme.of(context).colorScheme.onSurfaceVariant,
                          ),
                    ),
                  ],
                ),
              ),
            ],
          ),
          const Divider(height: 24),
          Row(
            children: [
              _Metric(label: '${route.distanceKm} km', icon: Icons.straighten),
              const SizedBox(width: 14),
              _Metric(label: '${route.minutes} min', icon: Icons.schedule),
              const Spacer(),
              _Metric(
                  label: '${route.widthMeters} m road',
                  icon: Icons.width_normal),
            ],
          ),
          const SizedBox(height: 11),
          _ConditionLine(
            passed: fits,
            text: fits
                ? route.oneWay
                    ? 'One-way road: sample vehicle clearance fits'
                    : 'Two-way road: sample opposing-car clearance fits'
                : 'Narrow for this car plus opposing traffic',
          ),
          const SizedBox(height: 6),
          _ConditionLine(
            passed: enoughEnergy,
            text: enoughEnergy
                ? 'Estimated ${vehicle.energyLabel.toLowerCase()} range covers this route'
                : 'Plan an energy stop before this route',
          ),
        ],
      ),
    );
  }
}

class _ConditionLine extends StatelessWidget {
  const _ConditionLine({required this.passed, required this.text});
  final bool passed;
  final String text;

  @override
  Widget build(BuildContext context) {
    final color = passed ? const Color(0xFF16805E) : const Color(0xFFC44D42);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 9),
      decoration: BoxDecoration(
        color: AppTheme.translucent(color, 0.09),
        borderRadius: BorderRadius.circular(11),
      ),
      child: Row(
        children: [
          Icon(
            passed ? Icons.check_circle_outline : Icons.warning_amber_rounded,
            color: color,
            size: 17,
          ),
          const SizedBox(width: 7),
          Expanded(
            child: Text(
              text,
              style: Theme.of(context).textTheme.labelSmall?.copyWith(
                    color: color,
                    fontWeight: FontWeight.w700,
                  ),
            ),
          ),
        ],
      ),
    );
  }
}

class _Metric extends StatelessWidget {
  const _Metric({required this.label, required this.icon});
  final String label;
  final IconData icon;

  @override
  Widget build(BuildContext context) => Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 14, color: Theme.of(context).colorScheme.primary),
          const SizedBox(width: 4),
          Text(label, style: Theme.of(context).textTheme.labelSmall),
        ],
      );
}

class _MiniTag extends StatelessWidget {
  const _MiniTag({required this.label});
  final String label;

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 4),
        decoration: BoxDecoration(
          color: const Color(0xFFE9F4EC),
          borderRadius: BorderRadius.circular(20),
        ),
        child: Text(
          label,
          style: const TextStyle(
            fontSize: 8,
            letterSpacing: 0.4,
            color: AppTheme.emerald,
            fontWeight: FontWeight.w800,
          ),
        ),
      );
}

class _StationCard extends StatelessWidget {
  const _StationCard({required this.station});
  final StationSample station;

  @override
  Widget build(BuildContext context) {
    return AppSurface(
      padding: const EdgeInsets.all(14),
      child: Row(
        children: [
          Container(
            width: 42,
            height: 42,
            decoration: BoxDecoration(
              color: const Color(0xFFE9F2ED),
              borderRadius: BorderRadius.circular(13),
            ),
            child: Icon(station.icon, color: AppTheme.emerald),
          ),
          const SizedBox(width: 11),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  station.name,
                  style: Theme.of(context)
                      .textTheme
                      .titleSmall
                      ?.copyWith(fontWeight: FontWeight.w800),
                ),
                const SizedBox(height: 3),
                Text(
                  '${station.type} · ${station.distanceKm} km',
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                        color: Theme.of(context).colorScheme.onSurfaceVariant,
                      ),
                ),
                const SizedBox(height: 5),
                Row(
                  children: [
                    const Icon(Icons.star_rounded,
                        size: 14, color: Color(0xFFE7A52E)),
                    const SizedBox(width: 3),
                    Text(
                      '${station.rating}  ·  ${station.trafficCount} sample cars observed',
                      style: Theme.of(context).textTheme.labelSmall,
                    ),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(width: 6),
          Text(
            station.price,
            textAlign: TextAlign.end,
            style: Theme.of(context).textTheme.labelMedium?.copyWith(
                  fontWeight: FontWeight.w800,
                ),
          ),
        ],
      ),
    );
  }
}
