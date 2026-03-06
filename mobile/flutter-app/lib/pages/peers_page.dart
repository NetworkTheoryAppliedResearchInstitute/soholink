import 'package:flutter/material.dart';

import '../api/soholink_client.dart';
import '../models/peer_info.dart';
import '../theme/app_theme.dart';
import '../widgets/section_header.dart';
import '../widgets/status_dot.dart';
import 'home_page.dart';

/// Lists all LAN mesh and mobile peers discovered by the node.
class PeersPage extends StatefulWidget {
  const PeersPage({super.key});

  @override
  State<PeersPage> createState() => _PeersPageState();
}

class _PeersPageState extends State<PeersPage> {
  PeersResponse? _resp;
  bool           _loading = true;
  String?        _error;

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
      final r = await SoHoLinkClient.instance.getPeers();
      if (mounted) setState(() { _resp = r; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator(color: SLColors.cyan));
    }
    if (_error != null) {
      return _ErrorPeers(error: _error!, onRetry: _fetch);
    }

    final peers = _resp!.peers;

    return RefreshIndicator(
      color: SLColors.cyan,
      backgroundColor: SLColors.surface,
      onRefresh: _fetch,
      child: CustomScrollView(
        slivers: [
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(16, 16, 16, 0),
              child: SectionHeader(
                title: 'Federation Peers',
                trailing: _PeerCountBadge(count: _resp!.count),
              ),
            ),
          ),

          if (peers.isEmpty)
            const SliverFillRemaining(
              child: _EmptyPeers(),
            )
          else
            SliverList(
              delegate: SliverChildBuilderDelegate(
                (ctx, i) {
                  final p = peers[i];
                  return Padding(
                    padding: const EdgeInsets.fromLTRB(16, 0, 16, 10),
                    child: _PeerCard(peer: p),
                  );
                },
                childCount: peers.length,
              ),
            ),

          const SliverPadding(padding: EdgeInsets.only(bottom: 32)),
        ],
      ),
    );
  }
}

// ── Peer card ───────────────────────────────────────────────────────────────

class _PeerCard extends StatelessWidget {
  final PeerInfo peer;
  const _PeerCard({required this.peer});

  Color _statusColor() {
    switch (peer.status) {
      case 'connected':   return SLColors.green;
      case 'degraded':    return SLColors.amber;
      default:            return SLColors.red;
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: SLColors.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: SLColors.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              StatusDot(healthy: peer.isConnected, size: 10),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  peer.id.isNotEmpty ? peer.id : 'Unknown peer',
                  style: theme.textTheme.titleMedium,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                decoration: BoxDecoration(
                  color: _statusColor().withOpacity(0.12),
                  borderRadius: BorderRadius.circular(20),
                ),
                child: Text(peer.status,
                    style: TextStyle(
                        color: _statusColor(), fontSize: 11,
                        fontWeight: FontWeight.w600)),
              ),
            ],
          ),

          if (peer.address.isNotEmpty) ...[
            const SizedBox(height: 8),
            _Row(icon: Icons.lan_rounded, text: peer.address),
          ],
          if (peer.region.isNotEmpty)
            _Row(icon: Icons.location_on_outlined, text: peer.region),
          if (peer.did.isNotEmpty)
            _Row(
              icon: Icons.fingerprint_rounded,
              text: _shortDid(peer.did),
            ),
          if (peer.latencyMs > 0)
            _Row(
              icon: Icons.network_ping_rounded,
              text: '${peer.latencyMs} ms',
              color: peer.latencyMs < 50
                  ? SLColors.green
                  : peer.latencyMs < 200
                      ? SLColors.amber
                      : SLColors.red,
            ),
        ],
      ),
    );
  }

  static String _shortDid(String did) =>
      did.length > 20 ? '${did.substring(0, 10)}…${did.substring(did.length - 6)}' : did;
}

class _Row extends StatelessWidget {
  final IconData icon;
  final String   text;
  final Color    color;

  const _Row({
    required this.icon,
    required this.text,
    this.color = SLColors.textSecondary,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 6),
      child: Row(
        children: [
          Icon(icon, size: 14, color: SLColors.textMuted),
          const SizedBox(width: 6),
          Expanded(
            child: Text(text,
                style: TextStyle(fontSize: 12, color: color),
                overflow: TextOverflow.ellipsis),
          ),
        ],
      ),
    );
  }
}

// ── Count badge ─────────────────────────────────────────────────────────────

class _PeerCountBadge extends StatelessWidget {
  final int count;
  const _PeerCountBadge({required this.count});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: SLColors.cyan.withOpacity(0.12),
        borderRadius: BorderRadius.circular(20),
        border: Border.all(color: SLColors.cyan.withOpacity(0.3)),
      ),
      child: Text('$count online',
          style: const TextStyle(
              color: SLColors.cyan, fontSize: 12, fontWeight: FontWeight.w600)),
    );
  }
}

// ── Empty state ─────────────────────────────────────────────────────────────

class _EmptyPeers extends StatelessWidget {
  const _EmptyPeers();

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(40),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.hub_outlined, color: SLColors.textMuted, size: 48),
            const SizedBox(height: 16),
            Text('No peers discovered',
                style: Theme.of(context)
                    .textTheme.titleMedium
                    ?.copyWith(color: SLColors.textSecondary)),
            const SizedBox(height: 8),
            Text(
              'Peers appear when other SoHoLINK nodes\ncome online on the same network.',
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.bodyMedium,
            ),
          ],
        ),
      ),
    );
  }
}

// ── Error state ─────────────────────────────────────────────────────────────

class _ErrorPeers extends StatelessWidget {
  final String       error;
  final VoidCallback onRetry;
  const _ErrorPeers({required this.error, required this.onRetry});

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
            Text('Failed to load peers',
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
