import 'dart:convert';
import 'dart:math';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';

abstract interface class DatabaseSecretVault {
  Future<String?> read(String key);
  Future<void> write(String key, String value);
  Future<void> delete(String key);
}

/// Stores the SQLCipher secret using Android Keystore-backed authenticated
/// encryption. The secure-storage namespace isolates ITEMBA-Z aliases and data.
class AndroidKeystoreSecretVault implements DatabaseSecretVault {
  AndroidKeystoreSecretVault({FlutterSecureStorage? storage})
    : _storage =
          storage ??
          const FlutterSecureStorage(
            aOptions: AndroidOptions(
              storageNamespace: 'itemba_z_sales_database',
            ),
          );

  final FlutterSecureStorage _storage;

  @override
  Future<String?> read(String key) => _storage.read(key: key);

  @override
  Future<void> write(String key, String value) =>
      _storage.write(key: key, value: value);

  @override
  Future<void> delete(String key) => _storage.delete(key: key);
}

/// Generates a 256-bit SQLCipher passphrase once and retrieves it from the
/// platform vault thereafter. The secret is never hardcoded or logged.
class DatabaseKeyProvider {
  DatabaseKeyProvider({
    required DatabaseSecretVault vault,
    Random? secureRandom,
  }) : _vault = vault,
       _secureRandom = secureRandom ?? Random.secure();

  static const secretKey = 'sqlcipher_key_v1';

  final DatabaseSecretVault _vault;
  final Random _secureRandom;
  Future<String>? _keyFuture;

  Future<String> getOrCreate() => _keyFuture ??= _loadOrCreate();

  Future<String> _loadOrCreate() async {
    final existing = await _vault.read(secretKey);
    if (existing != null && existing.isNotEmpty) return existing;

    final bytes = List<int>.generate(32, (_) => _secureRandom.nextInt(256));
    final generated = base64UrlEncode(bytes);
    await _vault.write(secretKey, generated);
    return generated;
  }
}
