import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:intl/intl.dart';

import '../api/soholink_client.dart';
import '../models/revenue.dart';
import '../theme/app_theme.dart';
import '../widgets/section_header.dart';
import '../widgets/stat_card.dart';
import 'home_page.dart';

/// Revenue page — shows daily earnings chart, totals, and fee breakdown.
class RevenuePage extends StatefulWidget {
  const RevenuePage({super.key});

  @override
  State<RevenuePage> createState() => _RevenuePageState();
}

class _RevenuePageState extends State<RevenuePage> {
  RevenueSummary? _rev;
  bool            _loading = true;
  String?         _error;

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
      final r = await SoHoLinkClient.instance.getRevenue();
      if (mounted) setState(() { _rev = r; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const Center(
        child: CircularProgressIndicator(color: SLColors.cyan));
    if (_error != null) return _RevenueError(error: _error!, onRetry: _fetch);

    final r    = _rev!;
    final fmts = NumberFormat('#,###');

    return RefreshIndicator(
      color: SLColors.cyan,
      backgroundColor: SLColors.surface,
      onRefresh: _fetch,
      child: ListView(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
        children: [
          // ── Summary cards ─────────────────────────────────────────────
          const SectionHeader(title: 'Earnings'),

          GridView.count(
            crossAxisCount: 2,
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            crossAxisSpacing: 12,
            mainAxisSpacing: 12,
            childAspectRatio: 1.25,
            children: [
              StatCard(
                icon: Icons.today_rounded,
                label: 'Today (gross)',
                value: r.satsToUsd(r.earnedSatsToday),
                subtitle: '${fmts.format(r.earnedSatsToday)} sats',
                accent: SLColors.amber,
              ),
              StatCard(
                icon: Icons.account_balance_wallet_outlined,
                label: 'Today (net)',
                value: r.satsToUsd(r.netSatsToday),
                subtitle: '${r.feePct.toStringAsFixed(0)}% fee deducted',
                accent: SLColors.green,
              ),
              StatCard(
                icon: Icons.date_range_rounded,
                label: 'Last 7 days',
                value: r.satsToUsd(r.earnedSats7d),
                subtitle: '${fmts.format(r.earnedSats7d)} sats',
                accent: SLColors.cyan,
              ),
              StatCard(
                icon: Icons.calendar_month_rounded,
                label: 'Last 30 days',
                value: r.satsToUsd(r.earnedSats30d),
                subtitle: '${fmts.format(r.earnedSats30d)} sats',
                accent: SLColors.purple,
              ),
            ],
          ),

          // ── 30-day chart ──────────────────────────────────────────────
          if (r.history.isNotEmpty) ...[
            const SectionHeader(title: '30-Day History'),
            Container(
              height: 200,
              padding: const EdgeInsets.fromLTRB(8, 16, 8, 8),
              decoration: BoxDecoration(
                color: SLColors.surface,
                borderRadius: BorderRadius.circular(12),
                border: Border.all(color: SLColors.border),
              ),
              child: _BarChart(history: r.history, btcUsdRate: r.btcUsdRate),
            ),
          ],

          // ── Fee breakdown ─────────────────────────────────────────────
          const SectionHeader(title: 'Fee Breakdown'),
          _FeeCard(rev: r),

          // ── All-time total ────────────────────────────────────────────
          const SectionHeader(title: 'All-Time Total'),
          Container(
            padding: const EdgeInsets.all(20),
            decoration: BoxDecoration(
              color: SLColors.surface,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: SLColors.border),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(r.satsToUsd(r.earnedSatsTotal),
                    style: Theme.of(context).textTheme.displayLarge
                        ?.copyWith(color: SLColors.amber)),
                const SizedBox(height: 4),
                Text(
                  '${fmts.format(r.earnedSatsTotal)} sats'
                  '  ·  ${RevenueSummary.satsToBtc(r.earnedSatsTotal)} BTC',
                  style: Theme.of(context).textTheme.bodyMedium,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// ── Bar chart ────────────────────────────────────────────────────────────────

class _BarChart extends StatelessWidget {
  final List<DailyRevenue> history;
  final double             btcUsdRate;
  const _BarChart({required this.history, required this.btcUsdRate});

  double _toUsdValue(int sats) =>
      btcUsdRate > 0 ? sats * btcUsdRate / 100000000.0 : sats.toDouble();

  @override
  Widget build(BuildContext context) {
    final values = history.map((e) => _toUsdValue(e.sats)).toList();
    final maxVal = values.fold<double>(0.01, (m, v) => v > m ? v : m);
    final useUsd = btcUsdRate > 0;

    return BarChart(
      BarChartData(
        maxY: maxVal * 1.2,
        gridData: FlGridData(
          show: true,
          drawVerticalLine: false,
          getDrawingHorizontalLine: (_) =>
              const FlLine(color: SLColors.border, strokeWidth: 1),
        ),
        borderData: FlBorderData(show: false),
        titlesData: FlTitlesData(
          bottomTitles: AxisTitles(
            sideTitles: SideTitles(
              showTitles: true,
              interval: 7,
              reservedSize: 22,
              getTitlesWidget: (v, _) {
                final idx = v.toInt();
                if (idx < 0 || idx >= history.length) return const SizedBox();
                final parts = history[idx].date.split('-');
                if (parts.length < 3) return const SizedBox();
                return Text('${parts[1]}/${parts[2]}',
                    style: const TextStyle(
                        fontSize: 9, color: SLColors.textMuted));
              },
            ),
          ),
          leftTitles: const AxisTitles(
              sideTitles: SideTitles(showTitles: false)),
          topTitles: const AxisTitles(
              sideTitles: SideTitles(showTitles: false)),
          rightTitles: const AxisTitles(
              sideTitles: SideTitles(showTitles: false)),
        ),
        barGroups: List.generate(history.length, (i) {
          return BarChartGroupData(
            x: i,
            barRods: [
              BarChartRodData(
                toY: values[i],
                color: SLColors.amber,
                width: 6,
                borderRadius: const BorderRadius.vertical(
                    top: Radius.circular(3)),
              ),
            ],
          );
        }),
        barTouchData: BarTouchData(
          touchTooltipData: BarTouchTooltipData(
            getTooltipColor: (_) => SLColors.surfaceAlt,
            tooltipRoundedRadius: 8,
            getTooltipItem: (group, _, rod, __) {
              final label = useUsd
                  ? NumberFormat.currency(symbol: '\$', decimalDigits: 2)
                      .format(rod.toY)
                  : '${NumberFormat('#,###').format(rod.toY.toInt())} sats';
              return BarTooltipItem(
                label,
                const TextStyle(color: SLColors.amber, fontSize: 12),
              );
            },
          ),
        ),
      ),
    );
  }
}

// ── Fee breakdown card ────────────────────────────────────────────────────────

class _FeeCard extends StatelessWidget {
  final RevenueSummary rev;
  const _FeeCard({required this.rev});

  @override
  Widget build(BuildContext context) {
    final gross = rev.earnedSatsToday;
    final fee   = (gross * rev.feePct / 100).round();
    final net   = gross - fee;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: SLColors.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: SLColors.border),
      ),
      child: Column(
        children: [
          _FeeRow('Gross earnings (today)', rev.satsToUsd(gross),
              SLColors.textPrimary),
          const SizedBox(height: 8),
          _FeeRow('Platform fee (${rev.feePct.toStringAsFixed(0)}%)',
              '− ${rev.satsToUsd(fee)}', SLColors.red),
          const Divider(height: 24),
          _FeeRow('Net to you', rev.satsToUsd(net), SLColors.green,
              bold: true),
        ],
      ),
    );
  }
}

class _FeeRow extends StatelessWidget {
  final String label;
  final String value;
  final Color  color;
  final bool   bold;

  const _FeeRow(this.label, this.value, this.color, {this.bold = false});

  @override
  Widget build(BuildContext context) {
    final style = Theme.of(context).textTheme.bodyMedium?.copyWith(
      color: color,
      fontWeight: bold ? FontWeight.w700 : FontWeight.normal,
    );
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(label, style: style),
        Text(value, style: style),
      ],
    );
  }
}

// ── Error ─────────────────────────────────────────────────────────────────────

class _RevenueError extends StatelessWidget {
  final String       error;
  final VoidCallback onRetry;
  const _RevenueError({required this.error, required this.onRetry});

  @override
  Widget build(BuildContext context) => Center(
    child: Padding(
      padding: const EdgeInsets.all(32),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.wifi_off_rounded, color: SLColors.red, size: 48),
          const SizedBox(height: 16),
          Text(error, textAlign: TextAlign.center,
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
