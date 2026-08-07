import '../domain/models.dart';
import 'local_store.dart';

abstract interface class AuthoritativeSyncGateway {
  Future<SyncResult> submit(SyncCommand command);
}

enum SyncFailureKind {
  retryable,
  ambiguous,
  terminal,
  reconciliation,
  authentication,
  suspended,
}

class SyncFailure implements Exception {
  const SyncFailure({required this.kind, required this.message, this.code});

  final SyncFailureKind kind;
  final String message;
  final String? code;

  @override
  String toString() => message;
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
        receiptReference: previous.receiptReference,
        fiscalStatus: previous.fiscalStatus,
        serverTotalMinor: previous.serverTotalMinor,
        serverSubtotalMinor: previous.serverSubtotalMinor,
        serverTaxMinor: previous.serverTaxMinor,
        serverCogsMinor: previous.serverCogsMinor,
        serverLines: previous.serverLines,
        wasDuplicate: true,
      );
    }
    _sequence += 1;
    final result = SyncResult(
      serverSaleId: 'sale-$_sequence',
      receiptReference: 'sale-$_sequence',
      fiscalStatus: FiscalStatus.notConfigured,
      serverTotalMinor: command.sale.total,
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

  Future<SyncResult> resolve(SyncCommand command) async {
    final localResult = await store.readSyncResult(command.idempotencyKey);
    if (localResult != null) {
      return SyncResult(
        serverSaleId: localResult.serverSaleId,
        receiptReference: localResult.receiptReference,
        fiscalStatus: localResult.fiscalStatus,
        serverTotalMinor: localResult.serverTotalMinor,
        serverSubtotalMinor: localResult.serverSubtotalMinor,
        serverTaxMinor: localResult.serverTaxMinor,
        serverCogsMinor: localResult.serverCogsMinor,
        serverLines: localResult.serverLines,
        wasDuplicate: true,
      );
    }
    return gateway.submit(command);
  }

  Future<SyncResult> synchronize(SyncCommand command) async {
    final result = await resolve(command);
    await store.saveSyncResult(command.idempotencyKey, result);
    return result;
  }
}
