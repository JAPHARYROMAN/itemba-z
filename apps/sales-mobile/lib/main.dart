import 'package:flutter/material.dart';

import 'application/mobile_runtime.dart';
import 'data/mobile_credential_store.dart';
import 'data/sqlcipher_local_store.dart';
import 'live_mobile_app.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();

  try {
    final store = await SqlCipherLocalStore.production();
    await store.initialize();
    final runtime = MobileRuntimeController(
      store: store,
      credentials: AndroidKeystoreCredentialStore(),
    );
    runApp(LiveMobileApp(runtime: runtime));
  } catch (_) {
    // Fail closed: never fall back to plaintext or memory persistence when the
    // encrypted database/Keystore is unavailable. Do not log storage errors,
    // because native messages can contain sensitive implementation details.
    runApp(const _SecureStorageUnavailableApp());
  }
}

class _SecureStorageUnavailableApp extends StatelessWidget {
  const _SecureStorageUnavailableApp();

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      home: Scaffold(
        body: SafeArea(
          child: Center(
            child: Padding(
              padding: const EdgeInsets.all(32),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(Icons.phonelink_lock, size: 64),
                  const SizedBox(height: 20),
                  Text(
                    'Secure storage unavailable',
                    style: Theme.of(context).textTheme.headlineSmall,
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: 10),
                  const Text(
                    'Hifadhi salama haipatikani. Close and reopen ITEMBA-Z, '
                    'or ask your administrator to re-authorize this device.',
                    textAlign: TextAlign.center,
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
