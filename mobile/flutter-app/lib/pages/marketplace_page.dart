import 'package:flutter/material.dart';
import 'package:intl/intl.dart';

import '../api/soholink_client.dart';
import '../models/marketplace.dart';
import '../theme/app_theme.dart';
import '../widgets/section_header.dart';
import 'home_page.dart';
import 'order_page.dart';

/// Marketplace page — browse available compute provider nodes.
/// Filter by CPU, price, region, and GPU; tap "Configure" to place an order.
class MarketplacePage extends StatefulWidget {
  const MarketplacePage({super.key});

  @override
  State<MarketplacePage> createState() => _MarketplacePageState();
}

class _MarketplacePageState extends State<MarketplacePage> {
  List<MarketplaceNode> _nodes   = [];
  bool                  _loading = true;
  String?               _error;

  // ── filter state ────────────────────────────────────────────────────────
  double  _minCpu       = 0;
  int?    _maxPrice;
  String  _region       = '';
  bool    _gpuOnly      = false;

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
      final nodes = await SoHoLinkClient.instance.getMarketplaceNodes(
        minCpu:       _minCpu > 0 ? _minCpu : null,
        maxPriceSats: _maxPrice,
        region:       _region.isNotEmpty ? _region : null,
        gpu:          _gpuOnly ? true : null,
      );
      if (mounted) setState(() { _nodes = nodes; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        _FilterBar(
          minCpu:    _minCpu,
          gpuOnly:   _gpuOnly,
          region:    _region,
          onChanged: (cpu, gpu, region) {
            _minCpu   = cpu;
            _gpuOnly  = gpu;
            _region   = region;
            _fetch();
          },
        ),
        Expanded(child: _buildBody()),
      ],
    );
  }

  Widget _buildBody() {
    if (_loading) {
      return const Center(child: CircularProgressIndicator(color: SLColors.cyan));
    }
    if (_error != null) {
      return _ErrorState(error: _error!, onRetry: _fetch);
    }
    if (_nodes.isEmpty) {
      return const _EmptyState();
    }
    return RefreshIndicator(
      color: SLColors.cyan,
      backgroundColor: SLColors.surface,
      onRefresh: _fetch,
      child: CustomScrollView(
        slivers: [
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 0),
              child: SectionHeader(
                title: 'Available Providers',
                trailing: _CountBadge(count: _nodes.length),
              ),
            ),
          ),
          SliverList(
            delegate: SliverChildBuilderDelegate(
              (ctx, i) => Padding(
                padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
                child: _ProviderCard(
                  node: _nodes[i],
                  onConfigure: () => _openOrderPage(_nodes[i]),
                ),
              ),
              childCount: _nodes.length,
            ),
          ),
          const SliverPadding(padding: EdgeInsets.only(bottom: 32)),
        ],
      ),
    );
  }

  void _openOrderPage(MarketplaceNode node) {
    Navigator.of(context).push(MaterialPageRoute(
      builder: (_) => OrderPage(node: node),
    ));
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Filter bar
// ─────────────────────────────────────────────────────────────────────────────

class _FilterBar extends StatefulWidget {
  final double  minCpu;
  final bool    gpuOnly;
  final String  region;
  final void Function(double cpu, bool gpu, String region) onChanged;

  const _FilterBar({
    required this.minCpu,
    required this.gpuOnly,
    required this.region,
    required this.onChanged,
  });

  @override
  State<_FilterBar> createState() => _FilterBarState();
}

class _FilterBarState extends State<_FilterBar> {
  late double  _cpu;
  late bool    _gpu;
  late String  _region;

  static const _regions = ['', 'us-east-1', 'us-west-2', 'eu-west-1', 'ap-southeast-1'];

  @override
  void initState() {
    super.initState();
    _cpu    = widget.minCpu;
    _gpu    = widget.gpuOnly;
    _region = widget.region;
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      color: SLColors.surface,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      child: Column(
        children: [
          Row(
            children: [
              Text('Min CPU: ${_cpu.toStringAsFixed(1)} cores',
                  style: const TextStyle(color: SLColors.textSecondary, fontSize: 12)),
              Expanded(
                child: Slider(
                  value: _cpu,
                  min: 0, max: 16, divisions: 32,
                  activeColor: SLColors.cyan,
                  onChanged: (v) => setState(() => _cpu = v),
                  onChangeEnd: (_) => widget.onChanged(_cpu, _gpu, _region),
                ),
              ),
            ],
          ),
          Row(
            children: [
              // Region dropdown
              Expanded(
                child: DropdownButtonFormField<String>(
                  value: _region,
                  decoration: const InputDecoration(
                    labelText: 'Region',
                    contentPadding: EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                    isDense: true,
                  ),
                  dropdownColor: SLColors.surfaceAlt,
                  style: const TextStyle(color: SLColors.textPrimary, fontSize: 13),
                  items: _regions.map((r) => DropdownMenuItem(
                    value: r,
                    child: Text(r.isEmpty ? 'Any region' : r),
                  )).toList(),
                  onChanged: (v) {
                    setState(() => _region = v ?? '');
                    widget.onChanged(_cpu, _gpu, _region);
                  },
                ),
              ),
              const SizedBox(width: 12),
              // GPU toggle
              Row(
                children: [
                  const Text('GPU', style: TextStyle(color: SLColors.textSecondary, fontSize: 12)),
                  Switch(
                    value: _gpu,
                    activeColor: SLColors.cyan,
                    onChanged: (v) {
                      setState(() => _gpu = v);
                      widget.onChanged(_cpu, _gpu, _region);
                    },
                  ),
                ],
              ),
            ],
          ),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Provider card
// ─────────────────────────────────────────────────────────────────────────────

class _ProviderCard extends StatelessWidget {
  final MarketplaceNode node;
  final VoidCallback    onConfigure;
  const _ProviderCard({required this.node, required this.onConfigure});

  Color _statusColor() => node.status == 'online' ? SLColors.green : SLColors.red;

  @override
  Widget build(BuildContext context) {
    final fmt = NumberFormat('#,###');

    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: SLColors.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: SLColors.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // ── header ───────────────────────────────────────────────────────
          Row(
            children: [
              Container(
                width: 8, height: 8,
                decoration: BoxDecoration(
                  color: _statusColor(),
                  shape: BoxShape.circle,
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(node.shortDid,
                    style: const TextStyle(
                        color: SLColors.textPrimary,
                        fontSize: 13,
                        fontWeight: FontWeight.w600),
                    overflow: TextOverflow.ellipsis),
              ),
              _RegionBadge(region: node.region),
            ],
          ),

          const SizedBox(height: 10),

          // ── resource grid ─────────────────────────────────────────────────
          Row(
            children: [
              _ResChip(icon: Icons.memory_outlined, label: '${node.availableCpu.toStringAsFixed(1)} vCPU'),
              const SizedBox(width: 8),
              _ResChip(icon: Icons.storage_outlined, label: '${(node.availableMemoryMb / 1024).toStringAsFixed(0)} GB RAM'),
              const SizedBox(width: 8),
              _ResChip(icon: Icons.sd_card_outlined, label: '${node.availableDiskGb} GB disk'),
            ],
          ),

          if (node.hasGpu && node.gpuModel.isNotEmpty) ...[
            const SizedBox(height: 6),
            _ResChip(icon: Icons.videocam_outlined, label: node.gpuModel),
          ],

          const SizedBox(height: 10),
          const Divider(height: 1),
          const SizedBox(height: 10),

          // ── pricing + reputation ─────────────────────────────────────────
          Row(
            children: [
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('${fmt.format(node.pricePerCpuHourSats)} sats/CPU/hr',
                      style: const TextStyle(
                          color: SLColors.amber,
                          fontSize: 13,
                          fontWeight: FontWeight.w700)),
                  const SizedBox(height: 2),
                  _ReputationBar(score: node.reputationScore),
                ],
              ),
              const Spacer(),
              ElevatedButton(
                onPressed: onConfigure,
                style: ElevatedButton.styleFrom(
                  padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                ),
                child: const Text('Configure'),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Small widgets
// ─────────────────────────────────────────────────────────────────────────────

class _RegionBadge extends StatelessWidget {
  final String region;
  const _RegionBadge({required this.region});

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
    decoration: BoxDecoration(
      color: SLColors.purple.withOpacity(0.15),
      borderRadius: BorderRadius.circular(20),
    ),
    child: Text(region.isEmpty ? 'global' : region,
        style: const TextStyle(
            color: SLColors.purple, fontSize: 10, fontWeight: FontWeight.w600)),
  );
}

class _ResChip extends StatelessWidget {
  final IconData icon;
  final String   label;
  const _ResChip({required this.icon, required this.label});

  @override
  Widget build(BuildContext context) => Row(
    mainAxisSize: MainAxisSize.min,
    children: [
      Icon(icon, size: 13, color: SLColors.textMuted),
      const SizedBox(width: 4),
      Text(label, style: const TextStyle(color: SLColors.textSecondary, fontSize: 12)),
    ],
  );
}

class _ReputationBar extends StatelessWidget {
  final int score; // 0–100 LBTAS score: 80+ = trusted, 50–79 = caution, <50 = new/risky
  const _ReputationBar({required this.score});

  Color _color() {
    if (score >= 80) return SLColors.green;
    if (score >= 50) return SLColors.amber;
    return SLColors.red;
  }

  String _label() {
    if (score >= 80) return 'Trusted';
    if (score >= 50) return 'Caution';
    return 'New/Risky';
  }

  @override
  Widget build(BuildContext context) => Tooltip(
    // Tooltip provides an accessible legend on long-press.
    message: 'LBTAS reputation score: ${_label()}\n'
             '80–100 % Trusted provider  ·  50–79 % Use caution  ·  0–49 % New or low-reputation\n'
             'Score reflects uptime, SLA adherence, and community feedback.',
    preferBelow: false,
    child: Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        SizedBox(
          width: 60, height: 4,
          child: ClipRRect(
            borderRadius: BorderRadius.circular(2),
            child: LinearProgressIndicator(
              value: score / 100.0,
              backgroundColor: SLColors.border,
              valueColor: AlwaysStoppedAnimation<Color>(_color()),
            ),
          ),
        ),
        const SizedBox(width: 6),
        Text('$score%',
            style: TextStyle(color: _color(), fontSize: 11, fontWeight: FontWeight.w600)),
        const SizedBox(width: 4),
        Icon(Icons.info_outline_rounded, size: 10, color: _color().withOpacity(0.6)),
      ],
    ),
  );
}

class _CountBadge extends StatelessWidget {
  final int count;
  const _CountBadge({required this.count});

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
    decoration: BoxDecoration(
      color: SLColors.cyan.withOpacity(0.12),
      borderRadius: BorderRadius.circular(20),
      border: Border.all(color: SLColors.cyan.withOpacity(0.3)),
    ),
    child: Text('$count nodes',
        style: const TextStyle(
            color: SLColors.cyan, fontSize: 12, fontWeight: FontWeight.w600)),
  );
}

class _EmptyState extends StatelessWidget {
  const _EmptyState();

  @override
  Widget build(BuildContext context) => Center(
    child: Padding(
      padding: const EdgeInsets.all(40),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.storefront_outlined, color: SLColors.textMuted, size: 48),
          const SizedBox(height: 16),
          Text('No providers found',
              style: Theme.of(context)
                  .textTheme.titleMedium
                  ?.copyWith(color: SLColors.textSecondary)),
          const SizedBox(height: 8),
          Text(
            'Adjust filters or wait for providers\nto join the federation.',
            textAlign: TextAlign.center,
            style: Theme.of(context).textTheme.bodyMedium,
          ),
        ],
      ),
    ),
  );
}

class _ErrorState extends StatelessWidget {
  final String       error;
  final VoidCallback onRetry;
  const _ErrorState({required this.error, required this.onRetry});

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
