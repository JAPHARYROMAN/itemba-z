import 'package:flutter_test/flutter_test.dart';
import 'package:sales_mobile/data/demo_data.dart';
import 'package:sales_mobile/data/local_store.dart';
import 'package:sales_mobile/data/sync_service.dart';
import 'package:sales_mobile/domain/models.dart';

void main() {
  test(
    'duplicate device + client transaction returns original result',
    () async {
      final store = InMemoryEncryptedLocalStore();
      final gateway = InMemoryAuthoritativeSyncGateway();
      final service = SalesSyncService(store: store, gateway: gateway);
      final now = DateTime(2026, 8, 4, 10, 30);
      final sale = CompletedSale(
        serverSaleId: '',
        receiptNumber: 'Pending',
        clientTransactionId: 'client-001',
        deviceId: demoDevice.deviceId,
        customer: demoCustomers.first,
        saleType: SaleType.cash,
        paymentMethod: PaymentMethod.cash,
        lines: [CartLine(product: demoProducts.first, quantity: 1)],
        total: demoProducts.first.sellingPrice,
        createdAt: now,
        paymentStatus: 'Paid',
        syncStatus: SyncStatus.pendingSync,
      );
      final command = SyncCommand(
        deviceId: demoDevice.deviceId,
        userId: demoDevice.userId,
        clientTransactionId: sale.clientTransactionId,
        clientTimestamp: now,
        companyId: demoDevice.companyId,
        branchId: demoDevice.branchId,
        warehouseId: demoDevice.warehouseId,
        appVersion: demoDevice.appVersion,
        masterDataVersion: demoDevice.masterDataVersion,
        priceVersion: demoDevice.priceVersion,
        syncAttemptNumber: 1,
        sale: sale,
      );

      final first = await service.synchronize(command);
      final retry = await service.synchronize(command);

      expect(first.wasDuplicate, isFalse);
      expect(retry.wasDuplicate, isTrue);
      expect(retry.serverSaleId, first.serverSaleId);
      expect(retry.receiptNumber, first.receiptNumber);
      expect(gateway.postedTransactionCount, 1);
    },
  );
}
