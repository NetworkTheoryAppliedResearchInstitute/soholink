import 'package:intl/intl.dart';

/// Revenue data from GET /api/revenue
class RevenueSummary {
  final int    earnedSatsTotal;
  final int    earnedSatsToday;
  final int    earnedSats7d;
  final int    earnedSats30d;
  final double feePct;
  final int    netSatsToday;
  final List<DailyRevenue> history;
  /// Live BTC/USD rate supplied by the node (0.0 if unavailable).
  final double btcUsdRate;

  const RevenueSummary({
    required this.earnedSatsTotal,
    required this.earnedSatsToday,
    required this.earnedSats7d,
    required this.earnedSats30d,
    required this.feePct,
    required this.netSatsToday,
    required this.history,
    required this.btcUsdRate,
  });

  factory RevenueSummary.fromJson(Map<String, dynamic> j) => RevenueSummary(
    earnedSatsTotal: (j['earned_sats_total'] as num?)?.toInt() ?? 0,
    earnedSatsToday: (j['earned_sats_today'] as num?)?.toInt() ?? 0,
    earnedSats7d:    (j['earned_sats_7d']    as num?)?.toInt() ?? 0,
    earnedSats30d:   (j['earned_sats_30d']   as num?)?.toInt() ?? 0,
    feePct:          (j['fee_pct']           as num?)?.toDouble() ?? 1.0,
    netSatsToday:    (j['net_sats_today']    as num?)?.toInt() ?? 0,
    btcUsdRate:      (j['btc_usd_rate']      as num?)?.toDouble() ?? 0.0,
    history: (j['history'] as List<dynamic>? ?? [])
        .map((e) => DailyRevenue.fromJson(e as Map<String, dynamic>))
        .toList(),
  );

  /// Satoshi → BTC string with 8 decimal places
  static String satsToBtc(int sats) =>
      (sats / 100000000).toStringAsFixed(8);

  /// Satoshi → formatted USD string, e.g. "$45.84"
  /// Returns empty string if rate is unavailable.
  String satsToUsd(int sats) {
    if (btcUsdRate <= 0) return '';
    final usd = sats * btcUsdRate / 100000000.0;
    return NumberFormat.currency(symbol: '\$', decimalDigits: 2).format(usd);
  }
}

class DailyRevenue {
  final String date;   // "2026-03-04"
  final int    sats;

  const DailyRevenue({required this.date, required this.sats});

  factory DailyRevenue.fromJson(Map<String, dynamic> j) => DailyRevenue(
    date: (j['date'] as String?) ?? '',
    sats: (j['sats'] as num?)?.toInt() ?? 0,
  );
}
