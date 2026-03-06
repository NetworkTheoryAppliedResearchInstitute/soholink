import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';

import '../api/soholink_client.dart';
import '../theme/app_theme.dart';
import 'home_page.dart';

/// First-run setup screen.
///
/// The user enters:
///   1. The node URL  (pre-filled with [kNodeUrl])
///   2. The owner private key  (64-char hex shown once at node first-start)
///
/// On Connect the app performs the Ed25519 challenge-response handshake
/// defined in [SoHoLinkClient.connectWithKey] and, if successful, stores
/// the device token and navigates to the dashboard.
class SetupPage extends StatefulWidget {
  const SetupPage({super.key});

  @override
  State<SetupPage> createState() => _SetupPageState();
}

class _SetupPageState extends State<SetupPage> {
  final _formKey    = GlobalKey<FormState>();
  final _urlCtrl    = TextEditingController(text: kNodeUrl);
  final _keyCtrl    = TextEditingController();
  bool  _showKey    = false;
  bool  _checking   = false;
  bool  _tosAccepted = false;   // must accept AUP before connecting
  String? _error;

  @override
  void dispose() {
    _urlCtrl.dispose();
    _keyCtrl.dispose();
    super.dispose();
  }

  Future<void> _connect() async {
    if (!_formKey.currentState!.validate()) return;
    if (!_tosAccepted) {
      setState(() => _error = 'You must accept the Terms of Service and AUP to continue.');
      return;
    }
    setState(() { _checking = true; _error = null; });

    final url   = _urlCtrl.text.trim();
    final pkHex = _keyCtrl.text.trim();

    // 1. Quick reachability check before attempting full auth.
    final reachable = await SoHoLinkClient.instance.checkHealth(url);
    if (!mounted) return;
    if (!reachable) {
      setState(() {
        _checking = false;
        _error = 'Could not reach the node. Check the URL and ensure the node is running.';
      });
      return;
    }

    // 2. Ed25519 challenge-response auth.
    try {
      await SoHoLinkClient.instance.connectWithKey(url, pkHex, 'SoHoLINK App');
      if (!mounted) return;
      Navigator.of(context).pushReplacement(
        MaterialPageRoute(builder: (_) => const HomePage()),
      );
    } on FormatException catch (e) {
      if (!mounted) return;
      setState(() { _checking = false; _error = e.message; });
    } catch (e) {
      if (!mounted) return;
      // Do not expose raw server errors or internal details to the user.
      setState(() {
        _checking = false;
        _error = 'Authentication failed. Check the URL and private key, then try again.';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      backgroundColor: SLColors.canvas,
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.symmetric(horizontal: 28, vertical: 48),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // ── Logo ─────────────────────────────────────────────────────
              Row(
                children: [
                  Container(
                    width: 42, height: 42,
                    decoration: BoxDecoration(
                      color: SLColors.cyan.withOpacity(0.12),
                      borderRadius: BorderRadius.circular(10),
                      border: Border.all(color: SLColors.cyan.withOpacity(0.4)),
                    ),
                    child: const Icon(Icons.router_rounded,
                        color: SLColors.cyan, size: 22),
                  ),
                  const SizedBox(width: 12),
                  Text('SoHoLINK',
                      style: GoogleFonts.rajdhani(
                        fontSize: 28, fontWeight: FontWeight.w700,
                        color: SLColors.textPrimary, letterSpacing: 1.5,
                      )),
                ],
              ),

              const SizedBox(height: 48),

              Text('Connect to your node',
                  style: theme.textTheme.displayLarge),
              const SizedBox(height: 8),
              Text(
                'Enter your node\'s address and the owner private key shown '
                'in the node console the first time it started.',
                style: theme.textTheme.bodyMedium,
              ),

              const SizedBox(height: 40),

              Form(
                key: _formKey,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    // ── URL ─────────────────────────────────────────────────
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
                      validator: (v) {
                        if (v == null || v.trim().isEmpty) return 'Please enter a URL';
                        final uri = Uri.tryParse(v.trim());
                        if (uri == null || !uri.hasScheme) {
                          return 'Must be a valid URL, e.g. http://192.168.1.x:8080';
                        }
                        return null;
                      },
                    ),

                    const SizedBox(height: 16),

                    // ── Owner private key ────────────────────────────────────
                    TextFormField(
                      controller: _keyCtrl,
                      obscureText: !_showKey,
                      autocorrect: false,
                      enableSuggestions: false,
                      style: theme.textTheme.bodyLarge
                          ?.copyWith(fontFamily: 'monospace', fontSize: 12),
                      decoration: InputDecoration(
                        labelText: 'Owner private key',
                        hintText: '64-character hex (shown once at node startup)',
                        prefixIcon: const Icon(Icons.vpn_key_rounded,
                            color: SLColors.textMuted, size: 18),
                        suffixIcon: IconButton(
                          icon: Icon(
                            _showKey
                                ? Icons.visibility_off_rounded
                                : Icons.visibility_rounded,
                            color: SLColors.textMuted, size: 18,
                          ),
                          onPressed: () => setState(() => _showKey = !_showKey),
                        ),
                      ),
                      validator: (v) {
                        final s = v?.trim() ?? '';
                        if (s.isEmpty) return 'Please enter the owner private key';
                        if (s.length != 64) {
                          return 'Key must be exactly 64 hex characters';
                        }
                        if (!RegExp(r'^[0-9a-fA-F]+$').hasMatch(s)) {
                          return 'Key must contain only hex characters (0-9, a-f)';
                        }
                        return null;
                      },
                    ),

                    // ── Error banner ─────────────────────────────────────────
                    if (_error != null) ...[
                      const SizedBox(height: 12),
                      Container(
                        padding: const EdgeInsets.all(12),
                        decoration: BoxDecoration(
                          color: SLColors.red.withOpacity(0.08),
                          borderRadius: BorderRadius.circular(8),
                          border: Border.all(color: SLColors.red.withOpacity(0.3)),
                        ),
                        child: Row(
                          children: [
                            const Icon(Icons.error_outline_rounded,
                                color: SLColors.red, size: 16),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Text(_error!,
                                  style: theme.textTheme.bodyMedium
                                      ?.copyWith(color: SLColors.red)),
                            ),
                          ],
                        ),
                      ),
                    ],

                    const SizedBox(height: 16),

                    // ── Terms of Service acceptance ──────────────────────────
                    Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(
                        color: SLColors.surface,
                        borderRadius: BorderRadius.circular(8),
                        border: Border.all(color: SLColors.border),
                      ),
                      child: Row(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Checkbox(
                            value: _tosAccepted,
                            activeColor: SLColors.cyan,
                            onChanged: _checking
                                ? null
                                : (v) => setState(() => _tosAccepted = v ?? false),
                          ),
                          Expanded(
                            child: Padding(
                              padding: const EdgeInsets.only(top: 12),
                              child: RichText(
                                text: TextSpan(
                                  style: theme.textTheme.bodySmall
                                      ?.copyWith(color: SLColors.textSecondary),
                                  children: const [
                                    TextSpan(text: 'I agree to the '),
                                    TextSpan(
                                      text: 'Acceptable Use Policy',
                                      style: TextStyle(
                                          color: SLColors.cyan,
                                          decoration: TextDecoration.underline),
                                    ),
                                    TextSpan(text: ' (ntari.org/aup). I understand that '
                                        'prohibited content (CSAM, malware, botnet tools) '
                                        'is blocked and reported to authorities automatically.'),
                                  ],
                                ),
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),

                    const SizedBox(height: 16),

                    // ── Connect button ───────────────────────────────────────
                    SizedBox(
                      height: 50,
                      child: ElevatedButton.icon(
                        onPressed: (_checking || !_tosAccepted) ? null : _connect,
                        icon: _checking
                            ? const SizedBox(
                                width: 18, height: 18,
                                child: CircularProgressIndicator(
                                    strokeWidth: 2, color: SLColors.canvas),
                              )
                            : const Icon(Icons.wifi_tethering_rounded, size: 18),
                        label: Text(_checking ? 'Authenticating…' : 'Connect'),
                      ),
                    ),
                  ],
                ),
              ),

              const SizedBox(height: 40),

              // ── Tip box ──────────────────────────────────────────────────
              Container(
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
                        const Icon(Icons.info_outline_rounded,
                            color: SLColors.cyan, size: 16),
                        const SizedBox(width: 8),
                        Text('Quick tips',
                            style: theme.textTheme.labelLarge
                                ?.copyWith(color: SLColors.cyan)),
                      ],
                    ),
                    const SizedBox(height: 10),
                    _tip('Private key', 'Printed once in the node console on first start'),
                    _tip('Android / iOS emulator', 'Use http://10.0.2.2:8080'),
                    _tip('Same Wi-Fi', 'Use the node machine\'s LAN IP, e.g. 192.168.x.x'),
                    _tip('Real node port', '8080  •  Preview mock: 4000'),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _tip(String label, String value) => Padding(
    padding: const EdgeInsets.only(top: 4),
    child: RichText(
      text: TextSpan(
        children: [
          TextSpan(
            text: '$label: ',
            style: const TextStyle(
                color: SLColors.textSecondary, fontSize: 12,
                fontWeight: FontWeight.w600),
          ),
          TextSpan(
            text: value,
            style: const TextStyle(
                color: SLColors.textMuted, fontSize: 12),
          ),
        ],
      ),
    ),
  );
}
