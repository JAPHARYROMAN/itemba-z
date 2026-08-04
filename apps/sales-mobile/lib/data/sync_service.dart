import '../domain/models.dart';
import 'local_store.dart';

abstract interface class AuthoritativeSyncGateway {
  Future<SyncResult> submit(SyncCommand command);
}

/// Development gateway that models the server's mandatory unique constraint:
/// `(device_id, client_transaction_id)`. A retry returns the original server
/// result and can never create a second business transaction.
class InMemoryAuthoritativeSyncGateway implements AuthoritativeSyncGateway {
  final Map<String, SyncResult> _posted = {};
  int _sequence = 2400;

  int get postedTransactionCount => _posted.length;

  @override
  Future<SyncResult> submit(SyncCommand command) async {
    final previous = _posted[command.idempotencyKey];
    if (previous != null) {
      return SyncResult(
        serverSaleId: previous.serverSaleId,
        receiptNumber: previous.receiptNumber,
        wasDuplicate: true,
      );
    }
    _sequence += 1;
    final result = SyncResult(
      serverSaleId: 'sale-$_sequence',
      receiptNumber: 'ITZ-DAR-${_sequence.toString().padLeft(6, '0')}',
      wasDuplicate: false,
    );
    _posted[command.idempotencyKey] = result;
    return result;
  }
}

class SalesSyncService {
  SalesSyncService({required this.store, required this.gateway});

  final EncryptedLocalStore store;
  final AuthoritativeSyncGateway gateway;

  Future<SyncResult> synchronize(SyncCommand command) async {
    final localResult = await store.readSyncResult(command.idempotencyKey);
    if (localResult != null) {
      return SyncResult(
        serverSaleId: localResult.serverSaleId,
        receiptNumber: localResult.receiptNumber,
        wasDuplicate: true,
      );
    }
    final result = await gateway.submit(command);
    await store.saveSyncResult(command.idempotencyKey, result);
    return result;
  }
}
