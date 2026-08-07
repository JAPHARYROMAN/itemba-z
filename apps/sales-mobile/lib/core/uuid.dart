import 'dart:convert';
import 'dart:math';

import 'package:crypto/crypto.dart';

/// RFC 9562 UUID utilities used for device and client transaction identities.
/// New identities use a cryptographically secure UUIDv4. Legacy, non-UUID
/// queue keys are mapped deterministically to UUIDv5 before they reach the API.
class UuidGenerator {
  UuidGenerator({Random? random}) : _random = random ?? Random.secure();

  static final RegExp _uuid = RegExp(
    r'^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$',
    caseSensitive: false,
  );

  static const String legacyTransactionNamespace =
      '65d66a6d-dc43-4fc7-906d-a3a3433ca38d';

  final Random _random;

  String v4() {
    final bytes = List<int>.generate(16, (_) => _random.nextInt(256));
    bytes[6] = (bytes[6] & 0x0f) | 0x40;
    bytes[8] = (bytes[8] & 0x3f) | 0x80;
    return _format(bytes);
  }

  static bool isValid(String value) => _uuid.hasMatch(value);

  static String normalizeLegacyTransactionId(String value) {
    final normalized = value.trim().toLowerCase();
    if (isValid(normalized)) return normalized;
    return v5(legacyTransactionNamespace, value);
  }

  static String v5(String namespace, String name) {
    final namespaceBytes = _parse(namespace);
    final digest =
        sha1.convert([...namespaceBytes, ...utf8.encode(name)]).bytes;
    final bytes = digest.take(16).toList(growable: false);
    bytes[6] = (bytes[6] & 0x0f) | 0x50;
    bytes[8] = (bytes[8] & 0x3f) | 0x80;
    return _format(bytes);
  }

  static List<int> _parse(String value) {
    if (!isValid(value)) throw FormatException('Invalid UUID: $value');
    final hex = value.replaceAll('-', '');
    return List<int>.generate(
      16,
      (index) => int.parse(hex.substring(index * 2, index * 2 + 2), radix: 16),
    );
  }

  static String _format(List<int> bytes) {
    final hex =
        bytes.map((byte) => byte.toRadixString(16).padLeft(2, '0')).join();
    return '${hex.substring(0, 8)}-'
        '${hex.substring(8, 12)}-'
        '${hex.substring(12, 16)}-'
        '${hex.substring(16, 20)}-'
        '${hex.substring(20)}';
  }
}
