import 'package:flutter_test/flutter_test.dart';
import 'package:sales_mobile/application/sales_controller.dart';
import 'package:sales_mobile/data/local_store.dart';
import 'package:sales_mobile/data/sync_service.dart';
import 'package:sales_mobile/domain/models.dart';

void main() {
  test(
    'restart replays exact durable command and persists server sale',
    () async {
      final store = InMemoryEncryptedLocalStore();
      final originalDevice = _device(appVersion: '1.0.0', version: 1);
      final currentDevice = _device(appVersion: '2.0.0', version: 9);
      final sale = _sale(syncStatus: SyncStatus.syncing);
      final command = _command(sale, originalDevice);
      await store.persistPendingSync(sale: sale, command: command);
      final gateway = _CapturingGateway();
      final controller = SalesController(
        device: currentDevice,
        customers: const [_customer],
        products: const [_product],
        store: store,
        gateway: gateway,
      );

      await controller.initializeLocalData(seedWhenEmpty: false);
      expect(controller.sales.single.syncStatus, SyncStatus.pendingSync);

      await controller.synchronizePending();

      final replayed = gateway.commands.single;
      expect(replayed.deviceId, command.deviceId);
      expect(replayed.appVersion, '1.0.0');
      expect(replayed.masterDataVersion, 1);
      expect(replayed.priceVersion, 1);
      expect(replayed.clientTimestamp, command.clientTimestamp);
      expect(replayed.syncAttemptNumber, 1);
      expect(replayed.sale.lines.single.unitPrice, 18500);
      final synced = controller.sales.single;
      expect(synced.syncStatus, SyncStatus.synced);
      expect(synced.total, 23600);
      expect(synced.subtotalMinor, 20000);
      expect(synced.taxMinor, 3600);
      expect(synced.lines.single.unitPrice, 20000);
      expect(synced.lines.single.tax, 3600);
      expect(synced.receiptReference, 'SALE-REF-1');
      expect(await store.readSyncQueue(), isEmpty);
      expect((await store.readSales()).single.total, 23600);
    },
  );

  test(
    'legacy interrupted syncing sale without a command requires review',
    () async {
      final store = InMemoryEncryptedLocalStore();
      await store.saveSale(_sale(syncStatus: SyncStatus.syncing));
      final controller = SalesController(
        device: _device(appVersion: '2.0.0', version: 9),
        customers: const [_customer],
        products: const [_product],
        store: store,
      );

      await controller.initializeLocalData(seedWhenEmpty: false);

      expect(controller.sales.single.syncStatus, SyncStatus.requiresReview);
      expect(controller.sales.single.syncMessage, contains('Do not repost'));
    },
  );
}

class _CapturingGateway implements AuthoritativeSyncGateway {
  final List<SyncCommand> commands = [];

  @override
  Future<SyncResult> submit(SyncCommand command) async {
    commands.add(command);
    return const SyncResult(
      serverSaleId: '00000000-0000-4000-8000-000000000020',
      receiptReference: 'SALE-REF-1',
      fiscalStatus: FiscalStatus.pending,
      serverSubtotalMinor: 20000,
      serverTaxMinor: 3600,
      serverTotalMinor: 23600,
      serverCogsMinor: 12000,
      serverLines: [
        AuthoritativeSaleLine(
          productId: '00000000-0000-4000-8000-000000000008',
          quantity: 1,
          unitPriceMinor: 20000,
          subtotalMinor: 20000,
          taxMinor: 3600,
          totalMinor: 23600,
        ),
      ],
      wasDuplicate: false,
    );
  }
}

const _customer = Customer(
  id: '00000000-0000-4000-8000-000000000007',
  name: 'General Customer',
  code: 'GENERAL',
  isGeneral: true,
  isActive: true,
  creditEnabled: false,
  inAttendantScope: true,
);

const _product = Product(
  id: '00000000-0000-4000-8000-000000000008',
  code: 'P-1',
  name: 'Cement',
  commonDescription: '',
  brand: '',
  category: '',
  unit: 'Bag',
  sellingPrice: 18500,
  availableQuantity: 10,
  taxBasisPoints: 0,
);

DeviceContext _device({required String appVersion, required int version}) =>
    DeviceContext(
      deviceId: '00000000-0000-4000-8000-000000000006',
      userId: '00000000-0000-4000-8000-000000000001',
      tenantId: '00000000-0000-4000-8000-000000000002',
      attendantName: 'Attendant',
      companyId: '00000000-0000-4000-8000-000000000003',
      companyName: 'Company',
      branchId: '00000000-0000-4000-8000-000000000004',
      branchName: 'Branch',
      warehouseId: '00000000-0000-4000-8000-000000000005',
      warehouseName: 'Warehouse',
      appVersion: appVersion,
      masterDataVersion: version,
      priceVersion: version,
      approved: true,
    );

CompletedSale _sale({required SyncStatus syncStatus}) => CompletedSale(
  serverSaleId: '',
  receiptReference: 'Pending',
  clientTransactionId: '00000000-0000-4000-8000-000000000009',
  deviceId: '00000000-0000-4000-8000-000000000006',
  customer: _customer,
  saleType: SaleType.cash,
  paymentMethod: PaymentMethod.cash,
  lines: [CartLine(product: _product, quantity: 1)],
  total: 18500,
  createdAt: DateTime.utc(2026, 8, 4, 8),
  paymentStatus: 'Paid',
  syncStatus: syncStatus,
);

SyncCommand _command(CompletedSale sale, DeviceContext device) => SyncCommand(
  deviceId: device.deviceId,
  userId: device.userId,
  clientTransactionId: sale.clientTransactionId,
  clientTimestamp: sale.createdAt,
  companyId: device.companyId,
  branchId: device.branchId,
  warehouseId: device.warehouseId,
  appVersion: device.appVersion,
  masterDataVersion: device.masterDataVersion,
  priceVersion: device.priceVersion,
  syncAttemptNumber: 1,
  sale: sale,
);
