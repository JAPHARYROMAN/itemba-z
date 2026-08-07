import 'package:flutter_secure_storage/flutter_secure_storage.dart';

abstract interface class MobileCredentialStore {
  Future<void> saveBearerToken(String token);
  Future<String?> readBearerToken();
  Future<void> deleteBearerToken();
}

class AndroidKeystoreCredentialStore implements MobileCredentialStore {
  AndroidKeystoreCredentialStore({FlutterSecureStorage? storage})
    : _storage =
          storage ??
          const FlutterSecureStorage(
            aOptions: AndroidOptions(
              storageNamespace: 'itemba_z_sales_credentials',
            ),
          );

  static const _tokenKey = 'api_bearer_token_v1';
  final FlutterSecureStorage _storage;

  @override
  Future<void> saveBearerToken(String token) async {
    final normalized = token.trim();
    if (normalized.isEmpty) throw ArgumentError.value(token, 'token');
    await _storage.write(key: _tokenKey, value: normalized);
  }

  @override
  Future<String?> readBearerToken() => _storage.read(key: _tokenKey);

  @override
  Future<void> deleteBearerToken() => _storage.delete(key: _tokenKey);
}

class InMemoryMobileCredentialStore implements MobileCredentialStore {
  String? _token;

  @override
  Future<void> saveBearerToken(String token) async {
    final normalized = token.trim();
    if (normalized.isEmpty) throw ArgumentError.value(token, 'token');
    _token = normalized;
  }

  @override
  Future<String?> readBearerToken() async => _token;

  @override
  Future<void> deleteBearerToken() async {
    _token = null;
  }
}
