import 'package:flutter/material.dart';

import '../theme/app_theme.dart';

/// Animated pulsing dot indicating node / peer status.
class StatusDot extends StatefulWidget {
  final bool   healthy;
  final double size;

  const StatusDot({super.key, required this.healthy, this.size = 10});

  @override
  State<StatusDot> createState() => _StatusDotState();
}

class _StatusDotState extends State<StatusDot>
    with SingleTickerProviderStateMixin {
  late AnimationController _ctrl;
  late Animation<double>   _scale;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1200),
    )..repeat(reverse: true);
    _scale = Tween<double>(begin: 0.85, end: 1.15).animate(
      CurvedAnimation(parent: _ctrl, curve: Curves.easeInOut),
    );
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final color = widget.healthy ? SLColors.green : SLColors.red;
    return ScaleTransition(
      scale: widget.healthy ? _scale : const AlwaysStoppedAnimation(1.0),
      child: Container(
        width: widget.size,
        height: widget.size,
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          color: color,
          boxShadow: widget.healthy
              ? [BoxShadow(color: color.withOpacity(0.45), blurRadius: 6)]
              : null,
        ),
      ),
    );
  }
}
