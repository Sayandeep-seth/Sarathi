import 'package:flutter/material.dart';

class VehicleSpec {
  const VehicleSpec({
    required this.name,
    required this.widthMeters,
    required this.lengthMeters,
    required this.heightMeters,
    required this.electric,
    required this.seats,
    this.rangePerLiter = 16,
    this.rangePerChargeKm = 320,
    this.fuelTankLiters = 50,
  });

  final String name;
  final double widthMeters;
  final double lengthMeters;
  final double heightMeters;
  final bool electric;
  final int seats;
  final double rangePerLiter;
  final int rangePerChargeKm;
  final double fuelTankLiters;

  String get energyLabel => electric ? 'Battery' : 'Fuel';
}

class SampleRoute {
  const SampleRoute({
    required this.name,
    required this.roadType,
    required this.widthMeters,
    required this.distanceKm,
    required this.minutes,
    required this.oneWay,
    required this.color,
  });

  final String name;
  final String roadType;
  final double widthMeters;
  final double distanceKm;
  final int minutes;
  final bool oneWay;
  final Color color;

  bool canPass(VehicleSpec vehicle) {
    final opposingVehicleAndClearance = oneWay ? 0.6 : 2.4;
    return widthMeters >= vehicle.widthMeters + opposingVehicleAndClearance;
  }
}

class StationSample {
  const StationSample({
    required this.name,
    required this.type,
    required this.distanceKm,
    required this.rating,
    required this.trafficCount,
    required this.price,
    required this.icon,
  });

  final String name;
  final String type;
  final double distanceKm;
  final double rating;
  final int trafficCount;
  final String price;
  final IconData icon;
}

class SavedRoute {
  const SavedRoute({
    required this.origin,
    required this.destination,
  });

  final String origin;
  final String destination;
}

class ShopProduct {
  const ShopProduct({
    required this.name,
    required this.description,
    required this.priceRupees,
    required this.category,
    required this.icon,
    required this.color,
  });

  final String name;
  final String description;
  final int priceRupees;
  final String category;
  final IconData icon;
  final Color color;
}

class MobilityCatalog {
  const MobilityCatalog._();

  static const vehicles = [
    VehicleSpec(
      name: 'Maruti Suzuki Swift',
      widthMeters: 1.735,
      lengthMeters: 3.860,
      heightMeters: 1.520,
      electric: false,
      seats: 5,
      rangePerLiter: 20,
      fuelTankLiters: 37,
    ),
    VehicleSpec(
      name: 'Hyundai Creta',
      widthMeters: 1.790,
      lengthMeters: 4.330,
      heightMeters: 1.635,
      electric: false,
      seats: 5,
      rangePerLiter: 17,
    ),
    VehicleSpec(
      name: 'Toyota Innova Crysta',
      widthMeters: 1.830,
      lengthMeters: 4.735,
      heightMeters: 1.795,
      electric: false,
      seats: 7,
      rangePerLiter: 12,
      fuelTankLiters: 55,
    ),
    VehicleSpec(
      name: 'Tata Nexon EV',
      widthMeters: 1.802,
      lengthMeters: 3.995,
      heightMeters: 1.625,
      electric: true,
      seats: 5,
      rangePerChargeKm: 320,
    ),
  ];

  static const routes = [
    SampleRoute(
      name: 'Ring Road',
      roadType: 'Two-way · wide road',
      widthMeters: 7.2,
      distanceKm: 12.4,
      minutes: 28,
      oneWay: false,
      color: Color(0xFF16805E),
    ),
    SampleRoute(
      name: 'Market Street',
      roadType: 'One-way · city road',
      widthMeters: 3.2,
      distanceKm: 9.8,
      minutes: 34,
      oneWay: true,
      color: Color(0xFFDB9A3D),
    ),
    SampleRoute(
      name: 'Riverside Road',
      roadType: 'Two-way · local road',
      widthMeters: 4.8,
      distanceKm: 14.1,
      minutes: 39,
      oneWay: false,
      color: Color(0xFF687CC2),
    ),
    SampleRoute(
      name: 'Old Town Lane',
      roadType: 'One-way · narrow lane',
      widthMeters: 2.0,
      distanceKm: 8.6,
      minutes: 31,
      oneWay: true,
      color: Color(0xFFC44D42),
    ),
  ];

  static const petrolStations = [
    StationSample(
      name: 'Greenway Fuel Point',
      type: 'Petrol · sample listing',
      distanceKm: 2.1,
      rating: 4.8,
      trafficCount: 126,
      price: '₹102.4 / L',
      icon: Icons.local_gas_station_rounded,
    ),
    StationSample(
      name: 'City Centre Petroleum',
      type: 'Petrol · sample listing',
      distanceKm: 4.6,
      rating: 4.5,
      trafficCount: 84,
      price: '₹101.9 / L',
      icon: Icons.local_gas_station_rounded,
    ),
  ];

  static const evStations = [
    StationSample(
      name: 'ChargeGrid Hub',
      type: 'DC fast charging · sample listing',
      distanceKm: 3.4,
      rating: 4.7,
      trafficCount: 38,
      price: '₹18 / kWh',
      icon: Icons.ev_station_rounded,
    ),
    StationSample(
      name: 'Park & Charge',
      type: 'AC charging · sample listing',
      distanceKm: 6.2,
      rating: 4.4,
      trafficCount: 22,
      price: '₹14 / kWh',
      icon: Icons.ev_station_rounded,
    ),
  ];

  static const products = [
    ShopProduct(
      name: 'Weekend getaway kit',
      description: 'Travel organiser, neck pillow & bottle',
      priceRupees: 1499,
      category: 'Tour',
      icon: Icons.luggage_rounded,
      color: Color(0xFFE7F1EB),
    ),
    ShopProduct(
      name: 'Explorer essentials',
      description: 'Daypack, cap & reusable flask',
      priceRupees: 2199,
      category: 'Tour',
      icon: Icons.hiking_rounded,
      color: Color(0xFFF4EEDD),
    ),
    ShopProduct(
      name: 'Celebration gift box',
      description: 'Curated festive treats & keepsakes',
      priceRupees: 1799,
      category: 'Celebration',
      icon: Icons.card_giftcard_rounded,
      color: Color(0xFFF5E7E4),
    ),
    ShopProduct(
      name: 'Wedding day combo',
      description: 'Ceremony-ready gifts for the whole family',
      priceRupees: 3499,
      category: 'Celebration',
      icon: Icons.celebration_rounded,
      color: Color(0xFFEDE8F5),
    ),
  ];
}
