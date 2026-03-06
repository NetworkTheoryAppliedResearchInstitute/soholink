import 'package:flutter/material.dart';
import 'package:intl/intl.dart';

import '../api/soholink_client.dart';
import '../models/marketplace.dart';
import '../theme/app_theme.dart';

/// Order configuration page — configure workload resources, see live cost
/// estimate, check wallet balance, and submit for payment.
class OrderPage extends StatefulWidget {
  final MarketplaceNode node;
  const OrderPage({super.key, required this.node});

  @override
  State<OrderPage> createState() => _OrderPageState();
}

class _OrderPageState extends State<OrderPage> {
  // ── resource sliders ───────────────────────────────────────────────────
  double _cpu        = 1.0;
  double _ramGb      = 1.0; // display in GB, send as MB
  double _diskGb     = 10.0;
  int    _durationH  = 4;

  static const _durations = [1, 4, 8, 24, 72];

  // ── remote state ───────────────────────────────────────────────────────
  CostEstimate  _estimate = CostEstimate.zero;
  WalletBalance _wallet   = WalletBalance.zero;
  bool          _estimating  = false;
  bool          _loadingWallet = false;
  bool          _purchasing    = false;
  String?       _error;

  @override
  void initState() {
    super.initState();
    // Clamp sliders to what the selected node can offer.
    _cpu    = _cpu.clamp(0.5, widget.node.availableCpu.clamp(0.5, 64));
    _ramGb  = _ramGb.clamp(0.5, (widget.node.availableMemoryMb / 1024).clamp(0.5, 512));
    _diskGb = _diskGb.clamp(1, widget.node.availableDiskGb.clamp(1, 10000).toDouble());
    _refreshEstimate();
    _loadWallet();
  }

  // ── data fetching ───────────────────────────────────────────────────────

  Future<void> _refreshEstimate() async {
    setState(() => _estimating = true);
    try {
      final est = await SoHoLinkClient.instance.estimateCost(
        cpuCores:      _cpu,
        memoryMb:      (_ramGb * 1024).round(),
        diskGb:        _diskGb.round(),
        durationHours: _durationH,
      );
      if (mounted) setState(() { _estimate = est; _estimating = false; });
    } catch (_) {
      if (mounted) setState(() => _estimating = false);
    }
  }

  Future<void> _loadWallet() async {
    setState(() => _loadingWallet = true);
    try {
      final bal = await SoHoLinkClient.instance.getWalletBalance();
      if (mounted) setState(() { _wallet = bal; _loadingWallet = false; });
    } catch (_) {
      if (mounted) setState(() => _loadingWallet = false);
    }
  }

  // ── purchase ─────────────────────────────────────────────────────────────

  Future<void> _purchase() async {
    if (_estimate.totalSats > _wallet.balanceSats) {
      _showTopupDialog();
      return;
    }
    // Require explicit confirmation before debiting the wallet.
    final confirmed = await _showConfirmationDialog();
    if (!confirmed || !mounted) return;

    setState(() { _purchasing = true; _error = null; });
    try {
      final result = await SoHoLinkClient.instance.purchaseWorkload(
        cpuCores:      _cpu,
        memoryMb:      (_ramGb * 1024).round(),
        diskGb:        _diskGb.round(),
        durationHours: _durationH,
      );
      if (!mounted) return;
      setState(() => _purchasing = false);
      _showSuccessDialog(result);
    } catch (e) {
      if (mounted) setState(() {
        // Do not expose raw API error to the user.
        _error = 'Purchase failed. Please try again.';
        _purchasing = false;
      });
    }
  }

  /// Shows a modal summary and returns true only when the user taps "Pay & Launch".
  Future<bool> _showConfirmationDialog() async {
    final fmt = NumberFormat('#,###');
    final result = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: SLColors.surfaceAlt,
        title: const Text('Confirm Purchase',
            style: TextStyle(color: SLColors.textPrimary)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Your wallet will be debited ${fmt.format(_estimate.totalSats)} sats'
              '${_estimate.totalUsd > 0 ? '  (≈ \$${_estimate.totalUsd.toStringAsFixed(4)})' : ''}.',
              style: const TextStyle(color: SLColors.textSecondary),
            ),
            const SizedBox(height: 10),
            Text(
              '${_cpu.toStringAsFixed(1)} vCPU · ${_ramGb.toStringAsFixed(1)} GB RAM'
              ' · ${_diskGb.round()} GB disk · ${_durationH}h',
              style: const TextStyle(color: SLColors.textMuted, fontSize: 12),
            ),
            const SizedBox(height: 10),
            const Text(
              'By confirming, you certify this workload complies with '
              'the Acceptable Use Policy. Prohibited workloads are '
              'blocked automatically and may result in account suspension.',
              style: TextStyle(color: SLColors.textMuted, fontSize: 11),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Cancel'),
          ),
          ElevatedButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Pay & Launch'),
          ),
        ],
      ),
    );
    return result ?? false;
  }

  void _showTopupDialog() {
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: SLColors.surfaceAlt,
        title: const Text('Insufficient balance',
            style: TextStyle(color: SLColors.textPrimary)),
        content: Text(
          'Your wallet has ${_wallet.balanceSats.toStringAsFixed(0)} sats '
          'but this workload costs ${_estimate.totalSats} sats.\n\n'
          'Top up your wallet to continue.',
          style: const TextStyle(color: SLColors.textSecondary),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Later'),
          ),
          ElevatedButton(
            onPressed: () {
              Navigator.pop(ctx);
              _showTopupAmountDialog();
            },
            child: const Text('Top Up'),
          ),
        ],
      ),
    );
  }

  void _showTopupAmountDialog() {
    final ctrl = TextEditingController(text: _estimate.totalSats.toString());
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: SLColors.surfaceAlt,
        title: const Text('Top Up Wallet',
            style: TextStyle(color: SLColors.textPrimary)),
        content: TextField(
          controller: ctrl,
          keyboardType: TextInputType.number,
          decoration: const InputDecoration(labelText: 'Amount (sats)'),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Cancel')),
          ElevatedButton(
            onPressed: () async {
              Navigator.pop(ctx);
              final sats = int.tryParse(ctrl.text);
              if (sats == null || sats <= 0) return;
              try {
                final r = await SoHoLinkClient.instance.topupWallet(amountSats: sats);
                if (!mounted) return;
                _showInvoiceDialog(r['invoice'] as String? ?? '', r['topup_id'] as String? ?? '');
              } catch (e) {
                if (!mounted) return;
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('Top-up failed: $e')));
              }
            },
            child: const Text('Create Invoice'),
          ),
        ],
      ),
    );
  }

  void _showInvoiceDialog(String invoice, String topupId) {
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: SLColors.surfaceAlt,
        title: const Text('Lightning Invoice', style: TextStyle(color: SLColors.textPrimary)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('Pay this invoice to credit your wallet:',
                style: TextStyle(color: SLColors.textSecondary, fontSize: 12)),
            const SizedBox(height: 8),
            if (invoice.isNotEmpty)
              SelectableText(invoice,
                  style: const TextStyle(
                      color: SLColors.cyan, fontSize: 10,
                      fontFamily: 'monospace'))
            else
              const Text('No invoice — confirm manually in test mode.',
                  style: TextStyle(color: SLColors.textMuted, fontSize: 12)),
          ],
        ),
        actions: [
          // Dev: manual confirm
          TextButton(
            onPressed: () async {
              Navigator.pop(ctx);
              try {
                await SoHoLinkClient.instance.confirmTopup(topupId);
                await _loadWallet();
              } catch (e) {
                if (!mounted) return;
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('Confirm failed: $e')));
              }
            },
            child: const Text('Confirm (dev)'),
          ),
          ElevatedButton(
            onPressed: () {
              Navigator.pop(ctx);
              _loadWallet(); // Refresh balance — user may have paid
            },
            child: const Text('Done'),
          ),
        ],
      ),
    );
  }

  void _showSuccessDialog(Map<String, dynamic> result) {
    final orderId    = result['order_id']    as String? ?? '';
    final charged    = result['charged_sats'] as int?   ?? 0;
    final fmt        = NumberFormat('#,###');
    showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (ctx) => AlertDialog(
        backgroundColor: SLColors.surfaceAlt,
        title: Row(
          children: [
            const Icon(Icons.check_circle_rounded, color: SLColors.green),
            const SizedBox(width: 8),
            const Text('Workload Launched!',
                style: TextStyle(color: SLColors.textPrimary)),
          ],
        ),
        content: Text(
          'Order: $orderId\nCharged: ${fmt.format(charged)} sats',
          style: const TextStyle(color: SLColors.textSecondary),
        ),
        actions: [
          ElevatedButton(
            onPressed: () {
              Navigator.pop(ctx);           // close dialog
              Navigator.pop(context);       // pop order page → back to market
            },
            child: const Text('Done'),
          ),
        ],
      ),
    );
  }

  // ── build ─────────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    final fmt = NumberFormat('#,###');

    return Scaffold(
      backgroundColor: SLColors.canvas,
      appBar: AppBar(
        title: const Text('Configure Workload'),
        backgroundColor: SLColors.surface,
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [

            // ── Provider summary ────────────────────────────────────────────
            _SectionCard(
              title: 'Provider',
              child: Row(
                children: [
                  const Icon(Icons.dns_outlined, color: SLColors.cyan, size: 16),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(widget.node.shortDid,
                            style: const TextStyle(
                                color: SLColors.textPrimary,
                                fontWeight: FontWeight.w600,
                                fontSize: 13),
                            overflow: TextOverflow.ellipsis),
                        Text(
                          '${widget.node.region} · '
                          '${fmt.format(widget.node.pricePerCpuHourSats)} sats/CPU/hr',
                          style: const TextStyle(
                              color: SLColors.textMuted, fontSize: 11),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),

            const SizedBox(height: 12),

            // ── Resource sliders ─────────────────────────────────────────────
            _SectionCard(
              title: 'Resources',
              child: Column(
                children: [
                  _SliderRow(
                    label: 'CPU cores',
                    value: _cpu,
                    min: 0.5,
                    max: widget.node.availableCpu.clamp(0.5, 64),
                    divisions: ((widget.node.availableCpu.clamp(0.5, 64) - 0.5) / 0.5).round().clamp(1, 127),
                    displayValue: '${_cpu.toStringAsFixed(1)} vCPU',
                    onChanged: (v) => setState(() => _cpu = v),
                    onChangeEnd: (_) => _refreshEstimate(),
                  ),
                  _SliderRow(
                    label: 'RAM',
                    value: _ramGb,
                    min: 0.5,
                    max: (widget.node.availableMemoryMb / 1024).clamp(0.5, 512),
                    divisions: (((widget.node.availableMemoryMb / 1024).clamp(0.5, 512) - 0.5) / 0.5).round().clamp(1, 511),
                    displayValue: '${_ramGb.toStringAsFixed(1)} GB',
                    onChanged: (v) => setState(() => _ramGb = v),
                    onChangeEnd: (_) => _refreshEstimate(),
                  ),
                  _SliderRow(
                    label: 'Disk',
                    value: _diskGb,
                    min: 1,
                    max: widget.node.availableDiskGb.clamp(1, 10000).toDouble(),
                    divisions: widget.node.availableDiskGb.clamp(1, 999),
                    displayValue: '${_diskGb.round()} GB',
                    onChanged: (v) => setState(() => _diskGb = v),
                    onChangeEnd: (_) => _refreshEstimate(),
                  ),
                  // Duration picker
                  Row(
                    children: [
                      const Text('Duration',
                          style: TextStyle(color: SLColors.textSecondary, fontSize: 12)),
                      const SizedBox(width: 12),
                      Expanded(
                        child: SingleChildScrollView(
                          scrollDirection: Axis.horizontal,
                          child: Row(
                            children: _durations.map((h) => Padding(
                              padding: const EdgeInsets.only(right: 8),
                              child: ChoiceChip(
                                label: Text('${h}h'),
                                selected: _durationH == h,
                                selectedColor: SLColors.cyan.withOpacity(0.2),
                                labelStyle: TextStyle(
                                  color: _durationH == h ? SLColors.cyan : SLColors.textMuted,
                                  fontSize: 12,
                                ),
                                onSelected: (_) {
                                  setState(() => _durationH = h);
                                  _refreshEstimate();
                                },
                              ),
                            )).toList(),
                          ),
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),

            const SizedBox(height: 12),

            // ── Cost estimate ────────────────────────────────────────────────
            _SectionCard(
              title: 'Cost Estimate',
              child: _estimating
                  ? const Center(
                      child: Padding(
                        padding: EdgeInsets.symmetric(vertical: 8),
                        child: SizedBox(
                          height: 20, width: 20,
                          child: CircularProgressIndicator(
                              strokeWidth: 2, color: SLColors.cyan),
                        ),
                      ),
                    )
                  : Column(
                      children: [
                        _CostRow('CPU',           _estimate.cpuCostSats),
                        _CostRow('Memory',        _estimate.memoryCostSats),
                        _CostRow('Disk',          _estimate.diskCostSats),
                        _CostRow('Platform fee',  _estimate.platformFeeSats),
                        const Divider(height: 16),
                        Row(
                          mainAxisAlignment: MainAxisAlignment.spaceBetween,
                          children: [
                            const Text('Total',
                                style: TextStyle(
                                    color: SLColors.textPrimary,
                                    fontWeight: FontWeight.w700,
                                    fontSize: 14)),
                            Text(
                              '${fmt.format(_estimate.totalSats)} sats'
                              '${_estimate.totalUsd > 0 ? '  ≈ \$${_estimate.totalUsd.toStringAsFixed(4)}' : ''}',
                              style: const TextStyle(
                                  color: SLColors.amber,
                                  fontWeight: FontWeight.w700,
                                  fontSize: 14),
                            ),
                          ],
                        ),
                      ],
                    ),
            ),

            const SizedBox(height: 12),

            // ── Wallet balance ────────────────────────────────────────────────
            _SectionCard(
              title: 'Wallet Balance',
              child: _loadingWallet
                  ? const Center(
                      child: SizedBox(
                        height: 20, width: 20,
                        child: CircularProgressIndicator(
                            strokeWidth: 2, color: SLColors.cyan),
                      ),
                    )
                  : Row(
                      children: [
                        const Icon(Icons.account_balance_wallet_outlined,
                            color: SLColors.cyan, size: 18),
                        const SizedBox(width: 8),
                        Text(
                          '${NumberFormat('#,###').format(_wallet.balanceSats)} sats',
                          style: TextStyle(
                            color: _wallet.balanceSats >= _estimate.totalSats
                                ? SLColors.green
                                : SLColors.red,
                            fontWeight: FontWeight.w700,
                            fontSize: 15,
                          ),
                        ),
                        if (_wallet.balanceUsd > 0) ...[
                          const SizedBox(width: 6),
                          Text(
                            '≈ \$${_wallet.balanceUsd.toStringAsFixed(2)}',
                            style: const TextStyle(
                                color: SLColors.textMuted, fontSize: 11),
                          ),
                        ],
                        const Spacer(),
                        TextButton(
                          onPressed: _loadWallet,
                          child: const Text('Refresh',
                              style: TextStyle(fontSize: 12)),
                        ),
                      ],
                    ),
            ),

            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(_error!,
                  style: const TextStyle(color: SLColors.red, fontSize: 12)),
            ],

            const SizedBox(height: 20),

            // ── Action button ─────────────────────────────────────────────────
            SizedBox(
              height: 50,
              child: ElevatedButton(
                onPressed: (_purchasing || _estimating) ? null : _purchase,
                child: _purchasing
                    ? const SizedBox(
                        height: 20, width: 20,
                        child: CircularProgressIndicator(
                            strokeWidth: 2, color: SLColors.canvas))
                    : Text(
                        _wallet.balanceSats >= _estimate.totalSats
                            ? 'Pay & Launch Workload'
                            : 'Add Funds to Continue',
                        style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w700),
                      ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Small widgets
// ─────────────────────────────────────────────────────────────────────────────

class _SectionCard extends StatelessWidget {
  final String title;
  final Widget child;
  const _SectionCard({required this.title, required this.child});

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(14),
    decoration: BoxDecoration(
      color: SLColors.surface,
      borderRadius: BorderRadius.circular(12),
      border: Border.all(color: SLColors.border),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(title.toUpperCase(),
            style: const TextStyle(
                color: SLColors.textMuted,
                fontSize: 10,
                fontWeight: FontWeight.w700,
                letterSpacing: 1.2)),
        const SizedBox(height: 10),
        child,
      ],
    ),
  );
}

class _SliderRow extends StatelessWidget {
  final String   label;
  final double   value;
  final double   min;
  final double   max;
  final int      divisions;
  final String   displayValue;
  final ValueChanged<double> onChanged;
  final ValueChanged<double> onChangeEnd;
  const _SliderRow({
    required this.label,
    required this.value,
    required this.min,
    required this.max,
    required this.divisions,
    required this.displayValue,
    required this.onChanged,
    required this.onChangeEnd,
  });

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: 4),
    child: Row(
      children: [
        SizedBox(
          width: 44,
          child: Text(label,
              style: const TextStyle(
                  color: SLColors.textSecondary, fontSize: 11)),
        ),
        Expanded(
          child: Slider(
            value: value,
            min: min,
            max: max,
            divisions: divisions,
            activeColor: SLColors.cyan,
            onChanged: onChanged,
            onChangeEnd: onChangeEnd,
          ),
        ),
        SizedBox(
          width: 70,
          child: Text(displayValue,
              style: const TextStyle(
                  color: SLColors.textPrimary, fontSize: 12),
              textAlign: TextAlign.right),
        ),
      ],
    ),
  );
}

class _CostRow extends StatelessWidget {
  final String label;
  final int    sats;
  const _CostRow(this.label, this.sats, {super.key});

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(vertical: 2),
    child: Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(label,
            style: const TextStyle(
                color: SLColors.textSecondary, fontSize: 13)),
        Text('${NumberFormat('#,###').format(sats)} sats',
            style: const TextStyle(
                color: SLColors.textPrimary, fontSize: 13)),
      ],
    ),
  );
}
