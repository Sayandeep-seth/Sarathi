import 'package:flutter/material.dart';

import '../data/mobility_catalog.dart';
import 'commute_screen.dart';
import 'explore_screen.dart';
import 'marketplace_screen.dart';
import 'trips_screen.dart';
import 'wifi_screen.dart';

class MobilityAppScreen extends StatefulWidget {
  const MobilityAppScreen({
    required this.phoneNumber,
    required this.onSignOut,
    super.key,
  });

  final String phoneNumber;
  final VoidCallback onSignOut;

  @override
  State<MobilityAppScreen> createState() => _MobilityAppScreenState();
}

class _MobilityAppScreenState extends State<MobilityAppScreen> {
  int _selectedIndex = 0;
  SavedRoute? _savedRoute;
  int _rideCount = 12;

  void _saveRoute(SavedRoute route) {
    setState(() => _savedRoute = route);
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('Route saved to your daily commute.')),
    );
  }

  void _addRide() {
    setState(() => _rideCount++);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final pages = [
      ExploreScreen(onSaveDailyRoute: _saveRoute),
      TripsScreen(onRideBooked: _addRide),
      CommuteScreen(
        savedRoute: _savedRoute,
      ),
      MarketplaceScreen(rideCount: _rideCount),
      const WifiScreen(),
    ];

    return Scaffold(
      appBar: AppBar(
        toolbarHeight: 48,
        automaticallyImplyLeading: false,
        title: const SizedBox.shrink(),
        backgroundColor: theme.scaffoldBackgroundColor,
        actions: [
          PopupMenuButton<String>(
            tooltip: 'Account',
            onSelected: (value) {
              if (value == 'signOut') widget.onSignOut();
            },
            itemBuilder: (context) => [
              PopupMenuItem<String>(
                enabled: false,
                child: Text('+91 ${widget.phoneNumber}'),
              ),
              const PopupMenuDivider(),
              const PopupMenuItem<String>(
                value: 'signOut',
                child: Row(
                  children: [
                    Icon(Icons.logout_rounded, size: 19),
                    SizedBox(width: 10),
                    Text('Sign out'),
                  ],
                ),
              ),
            ],
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 18),
              child: CircleAvatar(
                radius: 17,
                backgroundColor: theme.colorScheme.primary,
                child: const Icon(
                  Icons.person_outline_rounded,
                  color: Colors.white,
                  size: 19,
                ),
              ),
            ),
          ),
        ],
      ),
      body: SafeArea(
        bottom: false,
        child: IndexedStack(index: _selectedIndex, children: pages),
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _selectedIndex,
        onDestinationSelected: (index) {
          setState(() => _selectedIndex = index);
        },
        labelBehavior: NavigationDestinationLabelBehavior.alwaysShow,
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.map_outlined),
            selectedIcon: Icon(Icons.map_rounded),
            label: 'Explore',
          ),
          NavigationDestination(
            icon: Icon(Icons.local_taxi_outlined),
            selectedIcon: Icon(Icons.local_taxi_rounded),
            label: 'Trips',
          ),
          NavigationDestination(
            icon: Icon(Icons.qr_code_2_outlined),
            selectedIcon: Icon(Icons.qr_code_2_rounded),
            label: 'Daily',
          ),
          NavigationDestination(
            icon: Icon(Icons.shopping_bag_outlined),
            selectedIcon: Icon(Icons.shopping_bag_rounded),
            label: 'Shop',
          ),
          NavigationDestination(
            icon: Icon(Icons.wifi_outlined),
            selectedIcon: Icon(Icons.wifi_rounded),
            label: 'Wi-Fi',
          ),
        ],
      ),
      floatingActionButton: _selectedIndex == 0
          ? FloatingActionButton.small(
              tooltip: 'Current location',
              onPressed: () {
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(
                    content: Text(
                      'Live location is unavailable in this demo.',
                    ),
                  ),
                );
              },
              child: const Icon(Icons.my_location_rounded),
            )
          : null,
      floatingActionButtonLocation: FloatingActionButtonLocation.endFloat,
      backgroundColor: theme.scaffoldBackgroundColor,
    );
  }
}
