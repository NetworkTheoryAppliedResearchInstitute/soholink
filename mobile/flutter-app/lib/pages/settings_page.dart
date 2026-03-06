import 'package:flutter/material.dart';

import '../api/soholink_client.dart';
import '../theme/app_theme.dart';
import '../widgets/section_header.dart';
import 'setup_page.dart';

/// App settings: node URL, connection test, appearance info.
class SettingsPage extends StatefulWidget {
  const SettingsPage({super.key});

  @override
  State<SettingsPage> createState() => _SettingsPageState();
}

class _SettingsPageState extends State<SettingsPage> {
  final _urlCtrl      = TextEditingController();
  bool  _testing      = false;
  bool? _lastTestOk;

  @override
  void initState() {
    super.initState();
    _urlCtrl.text = SoHoLinkClient.instance.baseUrl;
  }

  @override
  void dispose() {
    _urlCtrl.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    final url = _urlCtrl.text.trim();
    if (url.isEmpty) return;
    await SoHoLinkClient.instance.setBaseUrl(url);
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('Node URL saved')),
    );
  }

  Future<void> _test() async {
    final url = _urlCtrl.text.trim();
    if (url.isEmpty) return;
    await SoHoLinkClient.instance.setBaseUrl(url);
    setState(() { _testing = true; _lastTestOk = null; });
    final ok = await SoHoLinkClient.instance.checkHealth(url);
    if (!mounted) return;
    setState(() { _testing = false; _lastTestOk = ok; });
  }

  void _changeNode() {
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => const SetupPage()),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
      children: [
        // ── Node connection ──────────────────────────────────────────────
        const SectionHeader(title: 'Node Connection'),

        Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: SLColors.surface,
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: SLColors.border),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TextFormField(
                controller: _urlCtrl,
                keyboardType: TextInputType.url,
                autocorrect: false,
                style: theme.textTheme.bodyLarge,
                decoration: const InputDecoration(
                  labelText: 'Node URL',
                  hintText: 'http://192.168.1.x:8080',
                  prefixIcon: Icon(Icons.lan_rounded,
                      color: SLColors.textMuted, size: 18),
                ),
              ),

              if (_lastTestOk != null) ...[
                const SizedBox(height: 10),
                Row(
                  children: [
                    Icon(
                      _lastTestOk!
                          ? Icons.check_circle_rounded
                          : Icons.error_rounded,
                      color: _lastTestOk! ? SLColors.green : SLColors.red,
                      size: 16,
                    ),
                    const SizedBox(width: 8),
                    Text(
                      _lastTestOk!
                          ? 'Connection successful'
                          : 'Could not reach node',
                      style: TextStyle(
                        color: _lastTestOk! ? SLColors.green : SLColors.red,
                        fontSize: 13,
                      ),
                    ),
                  ],
                ),
              ],

              const SizedBox(height: 14),

              Row(
                children: [
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: _testing ? null : _test,
                      icon: _testing
                          ? const SizedBox(
                              width: 14, height: 14,
                              child: CircularProgressIndicator(
                                  strokeWidth: 2, color: SLColors.cyan))
                          : const Icon(Icons.wifi_tethering_rounded, size: 16),
                      label: Text(_testing ? 'Testing…' : 'Test'),
                    ),
                  ),
                  const SizedBox(width: 10),
                  Expanded(
                    child: ElevatedButton.icon(
                      onPressed: _save,
                      icon: const Icon(Icons.save_rounded, size: 16),
                      label: const Text('Save'),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),

        // ── Actions ────────────────────────────────────────────────────
        const SectionHeader(title: 'Actions'),

        _ActionTile(
          icon:  Icons.swap_horiz_rounded,
          label: 'Change Node',
          subtitle: 'Connect to a different SoHoLINK node',
          onTap: _changeNode,
          accent: SLColors.cyan,
        ),

        const SizedBox(height: 10),

        _ActionTile(
          icon:  Icons.open_in_browser_rounded,
          label: 'API Reference',
          subtitle: 'View REST endpoint documentation',
          onTap: () => ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('See docs/ in the SoHoLINK repo')),
          ),
          accent: SLColors.purple,
        ),

        // ── About ──────────────────────────────────────────────────────
        const SectionHeader(title: 'About'),

        Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: SLColors.surface,
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: SLColors.border),
          ),
          child: Column(
            children: [
              _AboutRow('App', 'SoHoLINK Node Dashboard'),
              _AboutRow('Version', '0.1.0'),
              _AboutRow('API base', SoHoLinkClient.instance.baseUrl),
              _AboutRow('License', 'MIT'),
              _AboutRow('Org',
                  'Network Theory Applied Research Institute'),
            ],
          ),
        ),
      ],
    );
  }
}

// ── Helpers ────────────────────────────────────────────────────────────────────

class _ActionTile extends StatelessWidget {
  final IconData   icon;
  final String     label;
  final String     subtitle;
  final VoidCallback onTap;
  final Color      accent;

  const _ActionTile({
    required this.icon,
    required this.label,
    required this.subtitle,
    required this.onTap,
    this.accent = SLColors.cyan,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: SLColors.surface,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: SLColors.border),
        ),
        child: Row(
          children: [
            Container(
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: accent.withOpacity(0.12),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Icon(icon, color: accent, size: 18),
            ),
            const SizedBox(width: 14),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(label, style: Theme.of(context).textTheme.titleMedium),
                  Text(subtitle, style: Theme.of(context).textTheme.bodyMedium),
                ],
              ),
            ),
            const Icon(Icons.chevron_right_rounded,
                color: SLColors.textMuted, size: 18),
          ],
        ),
      ),
    );
  }
}

class _AboutRow extends StatelessWidget {
  final String label;
  final String value;
  const _AboutRow(this.label, this.value);

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: Theme.of(context).textTheme.bodyMedium),
          Flexible(
            child: Text(
              value,
              textAlign: TextAlign.right,
              style: Theme.of(context)
                  .textTheme.bodyMedium
                  ?.copyWith(color: SLColors.textPrimary),
            ),
          ),
        ],
      ),
    );
  }
}
