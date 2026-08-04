import 'package:flutter_test/flutter_test.dart';
import 'package:sales_mobile/core/uuid.dart';

void main() {
  test('new identities are RFC UUIDv4 values', () {
    final generator = UuidGenerator();
    final first = generator.v4();
    final second = generator.v4();

    expect(UuidGenerator.isValid(first), isTrue);
    expect(first.substring(14, 15), '4');
    expect(first, isNot(second));
  });

  test('legacy queue identity maps to one stable UUIDv5', () {
    final first = UuidGenerator.normalizeLegacyTransactionId('legacy-sale-1');
    final retry = UuidGenerator.normalizeLegacyTransactionId('legacy-sale-1');

    expect(first, retry);
    expect(UuidGenerator.isValid(first), isTrue);
    expect(first.substring(14, 15), '5');
  });
}
