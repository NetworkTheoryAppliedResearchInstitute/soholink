import 'package:flutter/material.dart';

import '../theme/app_theme.dart';

/// A labeled percentage bar for CPU / RAM / Storage / Network.
///
/// ```dart
/// ResourceBar(label: 'CPU', usedPct: 42.5, offeredLabel: '50%')
/// ```
class ResourceBar extends StatelessWidget {
  final String label;
  final double usedPct;       // 0–100
  final String offeredLabel;  // e.g. "50%" or "8 GB" or "100 Mbps"
  final Color  barColor;

  const ResourceBar({
    super.key,
    required this.label,
    required this.usedPct,
    required this.offeredLabel,
    this.barColor = SLColors.cyan,
  });

  Color _barTint() {
    if (usedPct >= 90) return SLColors.red;
    if (usedPct >= 70) return SLColors.amber;
    return barColor;
  }

  @override
  Widget build(BuildContext context) {
    final theme  = Theme.of(context);
    final tint   = _barTint();
    final clamped = (usedPct / 100).clamp(0.0, 1.0);

    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(label, style: theme.textTheme.bodyMedium),
              RichText(
                text: TextSpan(
                  children: [
                    TextSpan(
                      text: '${usedPct.toStringAsFixed(1)}%',
                      style: theme.textTheme.labelLarge?.copyWith(color: tint),
                    ),
                    TextSpan(
                      text: '  of $offeredLabel',
                      style: theme.textTheme.labelSmall,
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: 6),
          ClipRRect(
            borderRadius: BorderRadius.circular(4),
            child: LinearProgressIndicator(
              value: clamped,
              minHeight: 6,
              backgroundColor: SLColors.surfaceAlt,
              valueColor: AlwaysStoppedAnimation<Color>(tint),
            ),
          ),
        ],
      ),
    );
  }
}
