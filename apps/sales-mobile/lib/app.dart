import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';

import 'application/sales_controller.dart';
import 'core/app_strings.dart';
import 'core/theme.dart';
import 'domain/models.dart';
import 'presentation/sales_shell.dart';

class ItembaSalesApp extends StatefulWidget {
  const ItembaSalesApp({super.key, this.controller});

  final SalesController? controller;

  @override
  State<ItembaSalesApp> createState() => _ItembaSalesAppState();
}

class _ItembaSalesAppState extends State<ItembaSalesApp> {
  late final SalesController controller;
  late final bool ownsController;

  @override
  void initState() {
    super.initState();
    ownsController = widget.controller == null;
    controller = widget.controller ?? SalesController();
  }

  @override
  void dispose() {
    if (ownsController) controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: controller,
      builder: (context, _) {
        final locale =
            controller.language == AppLanguage.swahili
                ? const Locale('sw')
                : const Locale('en');
        return MaterialApp(
          debugShowCheckedModeBanner: false,
          title: 'ITEMBA-Z Sales',
          theme: AppTheme.light,
          locale: locale,
          supportedLocales: const [Locale('en'), Locale('sw')],
          localizationsDelegates: const [
            AppStrings.delegate,
            GlobalMaterialLocalizations.delegate,
            GlobalWidgetsLocalizations.delegate,
            GlobalCupertinoLocalizations.delegate,
          ],
          home: SalesShell(controller: controller),
        );
      },
    );
  }
}
