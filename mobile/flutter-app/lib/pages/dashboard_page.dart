import 'package:flutter/material.dart';
import 'package:intl/intl.dart';

import '../api/soholink_client.dart';
import '../models/node_status.dart';
import '../theme/app_theme.dart';
import '../widgets/resource_bar.dart';
import '../widgets/section_header.dart';
import '../widgets/stat_card.dart';
import '../widgets/status_dot.dart';
import 'home_page.dart';

/// Main overview tab showing node health metrics and resource usage.
class DashboardPage extends StatefulWidget {
  const DashboardPage({super.key});

  @override
  State<DashboardPage> createState() => _DashboardPageState();
}

class _DashboardPageState extends State<DashboardPage> {
  NodeStatus? _status;
  bool        _loading = true;
  String?     _error;

  @override
  void initState() {
    super.initState();
    _fetch();
    refreshNotifier.addListener(_onRefresh);
  }

  @override
  void dispose() {
    refreshNotifier.removeListener(_onRefresh);
    super.dispose();
  }

  void _onRefresh() => _fetch();

  Future<void> _fetch() async {
    if (!mounted) return;
    setState(() { _loading = true; _error = null; });
    try {
      final s = await SoHoLinkClient.instance.getStatus();
      if (mounted) setState(() { _status = s; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  String _toUsd(int sats, double rate) {
    if (rate <= 0) return '${NumberFormat('#,###').format(sats)} sats';
    final usd = sats * rate / 100000000.0;
    return NumberFormat.currency(symbol: '\$', decimalDigits: 2).format(usd);
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Center(
        child: CircularProgressIndicator(color: SLColors.cyan),
      );
    }
    if (_error != null) {
      return _ErrorView(error: _error!, onRetry: _fetch);
    }

    final s = _status!;

    return RefreshIndicator(
      color: SLColors.cyan,
      backgroundColor: SLColors.surface,
      onRefresh: _fetch,
      child: ListView(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
        children: [
          // ── Node health header ──────────────────────────────────────────
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: SLColors.surface,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: SLColors.border),
            ),
            child: Row(
              children: [
                StatusDot(healthy: s.activeRentals >= 0, size: 12),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('Node Online',
                          style: Theme.of(context)
                              .textTheme.titleMedium
                              ?.copyWith(color: SLColors.green)),
                      Text('Uptime: ${s.uptimeFormatted}  ·  ${s.os}',
                          style: Theme.of(context).textTheme.bodyMedium),
                    ],
                  ),
                ),
              ],
            ),
          ),

          // ── Summary stat cards ──────────────────────────────────────────
          const SectionHeader(title: 'Snapshot'),

          GridView.count(
            crossAxisCount: 2,
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            crossAxisSpacing: 12,
            mainAxisSpacing: 12,
            childAspectRatio: 1.3,
            children: [
              StatCard(
                icon: Icons.attach_money_rounded,
                label: 'Earned Today',
                value: _toUsd(s.earnedSatsToday, s.btcUsdRate),
                subtitle: '${NumberFormat('#,###').format(s.earnedSatsToday)} sats',
                accent: SLColors.amber,
              ),
              StatCard(
                icon: Icons.memory_rounded,
                label: 'Active Rentals',
                value: '${s.activeRentals}',
                accent: SLColors.cyan,
              ),
              StatCard(
                icon: Icons.hub_rounded,
                label: 'Federation Peers',
                value: '${s.federationNodes}',
                subtitle: '${s.mobileNodes} mobile',
                accent: SLColors.purple,
              ),
              StatCard(
                icon: Icons.router_rounded,
                label: 'CPU Offered',
                value: '${s.cpuOfferedPct}%',
                accent: SLColors.green,
              ),
            ],
          ),

          // ── Resource usage ──────────────────────────────────────────────
          const SectionHeader(title: 'Resource Usage'),

          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: SLColors.surface,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: SLColors.border),
            ),
            child: Column(
              children: [
                ResourceBar(
                  label: 'CPU',
                  usedPct: s.cpuUsedPct,
                  offeredLabel: '${s.cpuOfferedPct}%',
                ),
                const Divider(height: 16),
                ResourceBar(
                  label: 'RAM',
                  usedPct: s.ramUsedPct,
                  offeredLabel: s.ramOfferedGb > 0
                      ? '${s.ramOfferedGb.toStringAsFixed(1)} GB'
                      : '—',
                  barColor: SLColors.purple,
                ),
                const Divider(height: 16),
                ResourceBar(
                  label: 'Storage',
                  usedPct: s.storageUsedPct,
                  offeredLabel: s.storageOfferedGb > 0
                      ? '${s.storageOfferedGb.toStringAsFixed(1)} GB'
                      : '—',
                  barColor: SLColors.amber,
                ),
                const Divider(height: 16),
                ResourceBar(
                  label: 'Network',
                  usedPct: s.netUsedPct,
                  offeredLabel: s.netOfferedMbps > 0
                      ? '${s.netOfferedMbps} Mbps'
                      : '—',
                  barColor: SLColors.green,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// ── Error View ─────────────────────────────────────────────────────────────

class _ErrorView extends StatelessWidget {
  final String      error;
  final VoidCallback onRetry;

  const _ErrorView({required this.error, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.wifi_off_rounded, color: SLColors.red, size: 48),
            const SizedBox(height: 16),
            Text('Could not reach node',
                style: Theme.of(context)
                    .textTheme.titleMedium
                    ?.copyWith(color: SLColors.red)),
            const SizedBox(height: 8),
            Text(error,
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.bodyMedium),
            const SizedBox(height: 24),
            OutlinedButton.icon(
              onPressed: onRetry,
              icon: const Icon(Icons.refresh_rounded),
              label: const Text('Retry'),
            ),
          ],
        ),
      ),
    );
  }
}
