import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'api/soholink_client.dart';
import 'pages/home_page.dart';
import 'pages/setup_page.dart';
import 'theme/app_theme.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Force portrait orientation; relax if tablet landscape is desired.
  await SystemChrome.setPreferredOrientations([
    DeviceOrientation.portraitUp,
    DeviceOrientation.portraitDown,
  ]);

  // Status bar appearance — dark icons on transparent bar.
  SystemChrome.setSystemUIOverlayStyle(const SystemUiOverlayStyle(
    statusBarColor: Colors.transparent,
    statusBarIconBrightness: Brightness.light,
    systemNavigationBarColor: SLColors.surface,
    systemNavigationBarIconBrightness: Brightness.light,
  ));

  // Initialise shared preferences once; all pages use the singleton.
  await SoHoLinkClient.instance.init();

  runApp(const SoHoLinkApp());
}

class SoHoLinkApp extends StatelessWidget {
  const SoHoLinkApp({super.key});

  @override
  Widget build(BuildContext context) {
    // Decide landing page: if the user has already saved a node URL, go
    // straight to the dashboard; otherwise show the setup wizard.
    final configured = SoHoLinkClient.instance.hasConfiguredUrl;

    return MaterialApp(
      title: 'SoHoLINK',
      debugShowCheckedModeBanner: false,
      theme: AppTheme.dark,
      home: configured ? const HomePage() : const SetupPage(),
    );
  }
}
