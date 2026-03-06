/// Represents one active workload/rental from GET /api/workloads
class Workload {
  final String id;
  final String name;
  final String tenantDid;
  final String status;     // "running" | "pending" | "stopped"
  final int    cpuMillis;  // e.g. 500 = 0.5 vCPU
  final int    ramMb;
  final int    storageMb;
  final int    startedUnix;
  final int    earnedSats;

  const Workload({
    required this.id,
    required this.name,
    required this.tenantDid,
    required this.status,
    required this.cpuMillis,
    required this.ramMb,
    required this.storageMb,
    required this.startedUnix,
    required this.earnedSats,
  });

  factory Workload.fromJson(Map<String, dynamic> j) => Workload(
    id:          (j['id']           as String?) ?? '',
    name:        (j['name']         as String?) ?? '',
    tenantDid:   (j['tenant_did']   as String?) ?? '',
    status:      (j['status']       as String?) ?? 'unknown',
    cpuMillis:   (j['cpu_millis']   as num?)?.toInt() ?? 0,
    ramMb:       (j['ram_mb']       as num?)?.toInt() ?? 0,
    storageMb:   (j['storage_mb']   as num?)?.toInt() ?? 0,
    startedUnix: (j['started_unix'] as num?)?.toInt() ?? 0,
    earnedSats:  (j['earned_sats']  as num?)?.toInt() ?? 0,
  );

  bool get isRunning => status == 'running';

  /// e.g. "0.5 vCPU / 512 MB / 10 GB"
  String get resourceSummary {
    final cpu = (cpuMillis / 1000).toStringAsFixed(1);
    final ram = ramMb >= 1024
        ? '${(ramMb / 1024).toStringAsFixed(1)} GB'
        : '$ramMb MB';
    final disk = storageMb >= 1024
        ? '${(storageMb / 1024).toStringAsFixed(1)} GB'
        : '$storageMb MB';
    return '$cpu vCPU · $ram RAM · $disk disk';
  }
}

class WorkloadsResponse {
  final int             count;
  final List<Workload>  workloads;
  /// Live BTC/USD rate supplied by the node (0.0 if unavailable).
  final double          btcUsdRate;

  const WorkloadsResponse({
    required this.count,
    required this.workloads,
    required this.btcUsdRate,
  });

  factory WorkloadsResponse.fromJson(Map<String, dynamic> j) =>
      WorkloadsResponse(
        count:      (j['count']        as num?)?.toInt()    ?? 0,
        btcUsdRate: (j['btc_usd_rate'] as num?)?.toDouble() ?? 0.0,
        workloads: (j['workloads'] as List<dynamic>? ?? [])
            .map((e) => Workload.fromJson(e as Map<String, dynamic>))
            .toList(),
      );
}
