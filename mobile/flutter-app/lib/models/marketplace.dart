/// Marketplace domain models for the buyer-side purchase flow.
///
/// These classes mirror the JSON shapes produced by:
///   GET  /api/marketplace/nodes
///   POST /api/marketplace/estimate
///   GET  /api/wallet/balance
///   GET  /api/orders

// ─────────────────────────────────────────────────────────────────────────────
// Provider node
// ─────────────────────────────────────────────────────────────────────────────

class MarketplaceNode {
  final String nodeDid;
  final String address;
  final String region;
  final double availableCpu;
  final int    availableMemoryMb;
  final int    availableDiskGb;
  final bool   hasGpu;
  final String gpuModel;
  final int    pricePerCpuHourSats;
  final int    reputationScore;     // 0–100
  final double uptimePct;           // 0.0–100.0
  final String status;              // "online" | "offline"

  const MarketplaceNode({
    required this.nodeDid,
    required this.address,
    required this.region,
    required this.availableCpu,
    required this.availableMemoryMb,
    required this.availableDiskGb,
    required this.hasGpu,
    required this.gpuModel,
    required this.pricePerCpuHourSats,
    required this.reputationScore,
    required this.uptimePct,
    required this.status,
  });

  factory MarketplaceNode.fromJson(Map<String, dynamic> j) => MarketplaceNode(
    nodeDid:             j['node_did']               as String? ?? '',
    address:             j['address']                as String? ?? '',
    region:              j['region']                 as String? ?? '',
    availableCpu:        (j['available_cpu']         as num?)?.toDouble() ?? 0,
    availableMemoryMb:   (j['available_memory_mb']   as num?)?.toInt()    ?? 0,
    availableDiskGb:     (j['available_disk_gb']     as num?)?.toInt()    ?? 0,
    hasGpu:              j['has_gpu']                as bool?   ?? false,
    gpuModel:            j['gpu_model']              as String? ?? '',
    pricePerCpuHourSats: (j['price_per_cpu_hour_sats'] as num?)?.toInt() ?? 0,
    reputationScore:     (j['reputation_score']      as num?)?.toInt()    ?? 0,
    uptimePct:           (j['uptime_pct']            as num?)?.toDouble() ?? 0,
    status:              j['status']                 as String? ?? 'unknown',
  );

  /// Short DID label for display.
  String get shortDid => nodeDid.length > 20
      ? '${nodeDid.substring(0, 10)}…${nodeDid.substring(nodeDid.length - 6)}'
      : nodeDid;
}

// ─────────────────────────────────────────────────────────────────────────────
// Cost estimate
// ─────────────────────────────────────────────────────────────────────────────

class CostEstimate {
  final int    cpuCostSats;
  final int    memoryCostSats;
  final int    diskCostSats;
  final int    platformFeeSats;
  final int    totalSats;
  final double totalUsd;
  final double btcUsdRate;
  final int    durationHours;

  const CostEstimate({
    required this.cpuCostSats,
    required this.memoryCostSats,
    required this.diskCostSats,
    required this.platformFeeSats,
    required this.totalSats,
    required this.totalUsd,
    required this.btcUsdRate,
    required this.durationHours,
  });

  factory CostEstimate.fromJson(Map<String, dynamic> j) => CostEstimate(
    cpuCostSats:     (j['cpu_cost_sats']     as num?)?.toInt()    ?? 0,
    memoryCostSats:  (j['memory_cost_sats']  as num?)?.toInt()    ?? 0,
    diskCostSats:    (j['disk_cost_sats']    as num?)?.toInt()    ?? 0,
    platformFeeSats: (j['platform_fee_sats'] as num?)?.toInt()    ?? 0,
    totalSats:       (j['total_sats']        as num?)?.toInt()    ?? 0,
    totalUsd:        (j['total_usd']         as num?)?.toDouble() ?? 0,
    btcUsdRate:      (j['btc_usd_rate']      as num?)?.toDouble() ?? 0,
    durationHours:   (j['duration_hours']    as num?)?.toInt()    ?? 1,
  );

  static const CostEstimate zero = CostEstimate(
    cpuCostSats: 0, memoryCostSats: 0, diskCostSats: 0,
    platformFeeSats: 0, totalSats: 0, totalUsd: 0,
    btcUsdRate: 0, durationHours: 1,
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Wallet balance
// ─────────────────────────────────────────────────────────────────────────────

class WalletBalance {
  final int    balanceSats;
  final double balanceBtc;
  final double balanceUsd;
  final double btcUsdRate;

  const WalletBalance({
    required this.balanceSats,
    required this.balanceBtc,
    required this.balanceUsd,
    required this.btcUsdRate,
  });

  factory WalletBalance.fromJson(Map<String, dynamic> j) => WalletBalance(
    balanceSats: (j['balance_sats'] as num?)?.toInt()    ?? 0,
    balanceBtc:  (j['balance_btc']  as num?)?.toDouble() ?? 0,
    balanceUsd:  (j['balance_usd']  as num?)?.toDouble() ?? 0,
    btcUsdRate:  (j['btc_usd_rate'] as num?)?.toDouble() ?? 0,
  );

  static const WalletBalance zero = WalletBalance(
    balanceSats: 0, balanceBtc: 0, balanceUsd: 0, btcUsdRate: 0,
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Order
// ─────────────────────────────────────────────────────────────────────────────

class Order {
  final String   orderId;
  final String   orderType;       // "workload" | "service"
  final String   resourceRefId;
  final String   description;
  final double   cpuCores;
  final int      memoryMb;
  final int      diskGb;
  final int      durationHours;
  final int      estimatedSats;
  final int      chargedSats;
  final String   status;
  final DateTime createdAt;
  final DateTime updatedAt;

  const Order({
    required this.orderId,
    required this.orderType,
    required this.resourceRefId,
    required this.description,
    required this.cpuCores,
    required this.memoryMb,
    required this.diskGb,
    required this.durationHours,
    required this.estimatedSats,
    required this.chargedSats,
    required this.status,
    required this.createdAt,
    required this.updatedAt,
  });

  factory Order.fromJson(Map<String, dynamic> j) => Order(
    orderId:       j['order_id']       as String? ?? '',
    orderType:     j['order_type']     as String? ?? 'workload',
    resourceRefId: j['resource_ref_id'] as String? ?? '',
    description:   j['description']    as String? ?? '',
    cpuCores:      (j['cpu_cores']     as num?)?.toDouble() ?? 0,
    memoryMb:      (j['memory_mb']     as num?)?.toInt()    ?? 0,
    diskGb:        (j['disk_gb']       as num?)?.toInt()    ?? 0,
    durationHours: (j['duration_hours'] as num?)?.toInt()   ?? 0,
    estimatedSats: (j['estimated_sats'] as num?)?.toInt()   ?? 0,
    chargedSats:   (j['charged_sats']  as num?)?.toInt()    ?? 0,
    status:        j['status']         as String? ?? 'pending',
    createdAt:     _parseDate(j['created_at']),
    updatedAt:     _parseDate(j['updated_at']),
  );

  static DateTime _parseDate(dynamic v) {
    if (v is String) return DateTime.tryParse(v) ?? DateTime.now();
    return DateTime.now();
  }
}
