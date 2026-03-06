import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';

import '../theme/app_theme.dart';
import 'dashboard_page.dart';
import 'marketplace_page.dart';
import 'peers_page.dart';
import 'revenue_page.dart';
import 'workloads_page.dart';
import 'settings_page.dart';

/// Root shell with a [NavigationBar] at the bottom.
/// Maintains page state via [IndexedStack].
class HomePage extends StatefulWidget {
  const HomePage({super.key});

  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  int _selectedIndex = 0;

  static const _pages = <Widget>[
    DashboardPage(),
    PeersPage(),
    RevenuePage(),
    WorkloadsPage(),
    MarketplacePage(),
    SettingsPage(),
  ];

  static const _destinations = <NavigationDestination>[
    NavigationDestination(
      icon:         Icon(Icons.dashboard_outlined),
      selectedIcon: Icon(Icons.dashboard_rounded),
      label: 'Dashboard',
    ),
    NavigationDestination(
      icon:         Icon(Icons.hub_outlined),
      selectedIcon: Icon(Icons.hub_rounded),
      label: 'Peers',
    ),
    NavigationDestination(
      icon:         Icon(Icons.bolt_outlined),
      selectedIcon: Icon(Icons.bolt_rounded),
      label: 'Revenue',
    ),
    NavigationDestination(
      icon:         Icon(Icons.memory_outlined),
      selectedIcon: Icon(Icons.memory_rounded),
      label: 'Workloads',
    ),
    NavigationDestination(
      icon:         Icon(Icons.storefront_outlined),
      selectedIcon: Icon(Icons.storefront_rounded),
      label: 'Market',
    ),
    NavigationDestination(
      icon:         Icon(Icons.settings_outlined),
      selectedIcon: Icon(Icons.settings_rounded),
      label: 'Settings',
    ),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: SLColors.canvas,
      appBar: AppBar(
        backgroundColor: SLColors.surface,
        titleSpacing: 16,
        title: Row(
          children: [
            Container(
              width: 30, height: 30,
              decoration: BoxDecoration(
                color: SLColors.cyan.withOpacity(0.12),
                borderRadius: BorderRadius.circular(6),
              ),
              child: const Icon(Icons.router_rounded,
                  color: SLColors.cyan, size: 16),
            ),
            const SizedBox(width: 10),
            Text('SoHoLINK',
                style: GoogleFonts.rajdhani(
                  fontSize: 18, fontWeight: FontWeight.w700,
                  color: SLColors.textPrimary, letterSpacing: 1.2,
                )),
          ],
        ),
        actions: [
          Padding(
            padding: const EdgeInsets.only(right: 12),
            child: IconButton(
              icon: const Icon(Icons.refresh_rounded),
              tooltip: 'Refresh',
              onPressed: () {
                // Broadcast a refresh by rebuilding the current page.
                // Each page listens to this via a ValueNotifier.
                refreshNotifier.value = !refreshNotifier.value;
              },
            ),
          ),
        ],
      ),
      body: IndexedStack(index: _selectedIndex, children: _pages),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _selectedIndex,
        onDestinationSelected: (i) => setState(() => _selectedIndex = i),
        destinations: _destinations,
        labelBehavior: NavigationDestinationLabelBehavior.onlyShowSelected,
        animationDuration: const Duration(milliseconds: 300),
      ),
    );
  }
}

/// Simple global refresh signal. Pages listen to this to re-fetch data
/// when the user taps the AppBar refresh button.
final refreshNotifier = ValueNotifier<bool>(false);
