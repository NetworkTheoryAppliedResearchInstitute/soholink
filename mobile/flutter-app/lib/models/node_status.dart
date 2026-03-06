/// Represents the JSON payload from GET /api/status
class NodeStatus {
  final int    uptimeSeconds;
  final String os;
  final int    activeRentals;
  final int    federationNodes;
  final int    mobileNodes;
  final int    earnedSatsToday;
  final int    cpuOfferedPct;
  final double cpuUsedPct;
  final double ramOfferedGb;
  final double ramUsedPct;
  final double storageOfferedGb;
  final double storageUsedPct;
  final int    netOfferedMbps;
  final double netUsedPct;
  /// Live BTC/USD rate supplied by the node (0.0 if unavailable).
  final double btcUsdRate;

  const NodeStatus({
    required this.uptimeSeconds,
    required this.os,
    required this.activeRentals,
    required this.federationNodes,
    required this.mobileNodes,
    required this.earnedSatsToday,
    required this.cpuOfferedPct,
    required this.cpuUsedPct,
    required this.ramOfferedGb,
    required this.ramUsedPct,
    required this.storageOfferedGb,
    required this.storageUsedPct,
    required this.netOfferedMbps,
    required this.netUsedPct,
    required this.btcUsdRate,
  });

  factory NodeStatus.fromJson(Map<String, dynamic> j) => NodeStatus(
    uptimeSeconds:    (j['uptime_seconds']    as num?)?.toInt()    ?? 0,
    os:               (j['os']                as String?)          ?? '',
    activeRentals:    (j['active_rentals']    as num?)?.toInt()    ?? 0,
    federationNodes:  (j['federation_nodes']  as num?)?.toInt()    ?? 0,
    mobileNodes:      (j['mobile_nodes']      as num?)?.toInt()    ?? 0,
    earnedSatsToday:  (j['earned_sats_today'] as num?)?.toInt()    ?? 0,
    cpuOfferedPct:    (j['cpu_offered_pct']   as num?)?.toInt()    ?? 0,
    cpuUsedPct:       (j['cpu_used_pct']      as num?)?.toDouble() ?? 0.0,
    ramOfferedGb:     (j['ram_offered_gb']    as num?)?.toDouble() ?? 0.0,
    ramUsedPct:       (j['ram_used_pct']      as num?)?.toDouble() ?? 0.0,
    storageOfferedGb: (j['storage_offered_gb']as num?)?.toDouble() ?? 0.0,
    storageUsedPct:   (j['storage_used_pct']  as num?)?.toDouble() ?? 0.0,
    netOfferedMbps:   (j['net_offered_mbps']  as num?)?.toInt()    ?? 0,
    netUsedPct:       (j['net_used_pct']      as num?)?.toDouble() ?? 0.0,
    btcUsdRate:       (j['btc_usd_rate']      as num?)?.toDouble() ?? 0.0,
  );

  /// Human-readable uptime string, e.g. "3d 4h 22m"
  String get uptimeFormatted {
    final d = Duration(seconds: uptimeSeconds);
    final days  = d.inDays;
    final hours = d.inHours.remainder(24);
    final mins  = d.inMinutes.remainder(60);
    if (days > 0) return '${days}d ${hours}h ${mins}m';
    if (hours > 0) return '${hours}h ${mins}m';
    return '${mins}m';
  }
}
