import 'package:flutter_test/flutter_test.dart';
import 'package:sales_mobile/application/sales_controller.dart';
import 'package:sales_mobile/data/demo_data.dart';
import 'package:sales_mobile/data/local_store.dart';
import 'package:sales_mobile/data/sync_service.dart';
import 'package:sales_mobile/domain/connection_models.dart';
import 'package:sales_mobile/domain/models.dart';

void main() {
  test('ambiguous success parsing retains the exact durable command', () async {
    final store = InMemoryEncryptedLocalStore();
    final gateway = _FailingGateway(SyncFailureKind.ambiguous);
    final completedAt = DateTime.utc(2026, 8, 4, 9, 5);
    final controller = SalesController(
      store: store,
      gateway: gateway,
      clock: () => completedAt,
    );
    final draft = controller.createNewSale(now: DateTime.utc(2026, 8, 4, 9))
      ..addProduct(demoProducts.first);

    final sale = await controller.completeSale(draft);

    expect(sale.syncStatus, SyncStatus.requiresReview);
    expect(controller.isOnline, isTrue);
    final queued = (await store.readSyncQueue()).single;
    expect(queued.clientTransactionId, draft.clientTransactionId);
    expect(queued.clientTimestamp, completedAt);
    expect(queued.sale.createdAt, completedAt);
    expect(queued.masterDataVersion, draft.device.masterDataVersion);
    expect(queued.priceVersion, draft.device.priceVersion);
    expect(queued.sale.createdOffline, isFalse);
    expect(queued.sale.total, draft.total);

    final restarted = SalesController(store: store, gateway: gateway);
    await restarted.initializeLocalData();
    expect(restarted.sales.single.syncStatus, SyncStatus.pendingSync);
    expect(
      (await store.readSyncQueue()).single.clientTransactionId,
      draft.clientTransactionId,
    );
  });

  test(
    'exhausted transport failure moves later sales onto offline gates',
    () async {
      final store = InMemoryEncryptedLocalStore();
      final gateway = _FailingGateway(SyncFailureKind.retryable);
      final controller = SalesController(store: store, gateway: gateway);
      final first = controller.createNewSale(now: DateTime.utc(2026, 8, 4, 9))
        ..addProduct(demoProducts.first);

      final failed = await controller.completeSale(first);

      expect(failed.syncStatus, SyncStatus.pendingSync);
      expect(controller.isOnline, isFalse);
      expect(controller.liveConnectionState, LiveConnectionState.offlineCache);
      expect(gateway.calls, 1);
      final firstCommand = (await store.readSyncQueue()).single;
      expect(firstCommand.sale.createdOffline, isFalse);

      final second = controller.createNewSale(now: DateTime.utc(2026, 8, 4, 10))
        ..addProduct(demoProducts.first);
      final offline = await controller.completeSale(second);

      expect(offline.createdOffline, isTrue);
      expect(offline.paymentMethod, PaymentMethod.cash);
      expect(
        gateway.calls,
        1,
        reason: 'offline completion must not call the API',
      );
      expect(await store.readSyncQueue(), hasLength(2));
    },
  );

  test('only authoritative terminal rejection removes the command', () async {
    final store = InMemoryEncryptedLocalStore();
    final controller = SalesController(
      store: store,
      gateway: _FailingGateway(SyncFailureKind.terminal),
    );
    final draft = controller.createNewSale()..addProduct(demoProducts.first);

    final rejected = await controller.completeSale(draft);

    expect(rejected.syncStatus, SyncStatus.rejected);
    expect(await store.readSyncQueue(), isEmpty);
  });
}

class _FailingGateway implements AuthoritativeSyncGateway {
  _FailingGateway(this.kind);

  final SyncFailureKind kind;
  int calls = 0;

  @override
  Future<SyncResult> submit(SyncCommand command) async {
    calls += 1;
    throw SyncFailure(kind: kind, message: 'simulated failure');
  }
}
