/// Represents one entry in the GET /api/peers response
class PeerInfo {
  final String id;
  final String did;
  final String address;
  final String region;
  final int    latencyMs;
  final String status; // "connected" | "degraded" | "unreachable"
  final int    lastSeenUnix;

  const PeerInfo({
    required this.id,
    required this.did,
    required this.address,
    required this.region,
    required this.latencyMs,
    required this.status,
    required this.lastSeenUnix,
  });

  factory PeerInfo.fromJson(Map<String, dynamic> j) => PeerInfo(
    id:           (j['id']            as String?) ?? '',
    did:          (j['did']           as String?) ?? '',
    address:      (j['address']       as String?) ?? '',
    region:       (j['region']        as String?) ?? '',
    latencyMs:    (j['latency_ms']    as num?)?.toInt()    ?? 0,
    status:       (j['status']        as String?) ?? 'unknown',
    lastSeenUnix: (j['last_seen_unix']as num?)?.toInt()    ?? 0,
  );

  bool get isConnected => status == 'connected';
}

class PeersResponse {
  final int            count;
  final List<PeerInfo> peers;

  const PeersResponse({required this.count, required this.peers});

  factory PeersResponse.fromJson(Map<String, dynamic> j) => PeersResponse(
    count: (j['count'] as num?)?.toInt() ?? 0,
    peers: (j['peers'] as List<dynamic>? ?? [])
        .map((e) => PeerInfo.fromJson(e as Map<String, dynamic>))
        .toList(),
  );
}
