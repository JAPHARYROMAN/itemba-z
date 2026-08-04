import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:path/path.dart' as path_utils;
import 'package:sales_mobile/data/database_key_provider.dart';
import 'package:sales_mobile/data/demo_data.dart';
import 'package:sales_mobile/data/encrypted_database_opener.dart';
import 'package:sales_mobile/data/local_database_schema.dart';
import 'package:sales_mobile/data/sqlcipher_local_store.dart';
import 'package:sales_mobile/domain/connection_models.dart';
import 'package:sales_mobile/domain/models.dart';
import 'package:sqflite_common_ffi/sqflite_ffi.dart';

void main() {
  setUpAll(sqfliteFfiInit);

  test('database key is random, persisted once, and reused', () async {
    final vault = _MemorySecretVault();
    final firstProvider = DatabaseKeyProvider(vault: vault);
    final first = await firstProvider.getOrCreate();
    final second = await firstProvider.getOrCreate();
    final afterRestart = await DatabaseKeyProvider(vault: vault).getOrCreate();

    expect(base64Url.decode(first), hasLength(32));
    expect(second, first);
    expect(afterRestart, first);
    expect(vault.writeCount, 1);
  });

  test(
    'encrypted repository round-trips cache, draft, sale, and queue',
    () async {
      final fixture = await _StoreFixture.create();
      addTearDown(fixture.dispose);
      final store = fixture.store;
      await store.initialize();

      final connection = MobileConnection(
        baseUrl: Uri.parse('https://api.example.test'),
        identityMode: MobileIdentityMode.bearer,
      );
      final enrollment = DeviceEnrollment(
        deviceId: '00000000-0000-4000-8000-000000000006',
        deviceName: 'Counter 1',
        status: 'ACTIVE',
        actorId: '00000000-0000-4000-8000-000000000001',
        scope: const EnrollmentScope(
          tenantId: '00000000-0000-4000-8000-000000000002',
          companyId: '00000000-0000-4000-8000-000000000003',
          branchId: '00000000-0000-4000-8000-000000000004',
          warehouseId: '00000000-0000-4000-8000-000000000005',
        ),
        appVersion: '1.0.0+1',
        catalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
        masterDataVersion: 4,
        priceVersion: 8,
        availableMasterDataVersion: 4,
        availablePriceVersion: 8,
        availableCatalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
        timezone: 'Africa/Dar_es_Salaam',
        offlineEnabled: false,
        transactionValueLimitMinor: 0,
        dailyValueLimitMinor: 0,
        remainingDailyValueMinor: 0,
        offlineSalesValidUntil: DateTime.utc(2026, 8, 4, 12),
        stockAllocations: const [],
        enrolledAt: DateTime.utc(2026, 8, 4, 8),
        lastSeenAt: DateTime.utc(2026, 8, 4, 8),
      );
      final allocation = DeviceAllocation.failClosed(
        serverOfflineEnabled: false,
      );
      await store.saveConnection(connection);
      await store.saveCandidateDeviceId(enrollment.deviceId);
      await store.saveEnrollment(enrollment);
      await store.saveAllocation(allocation);
      expect((await store.readConnection())?.baseUrl, connection.baseUrl);
      expect(await store.readCandidateDeviceId(), enrollment.deviceId);
      expect((await store.readEnrollment())?.deviceId, enrollment.deviceId);
      expect((await store.readEnrollment())?.availableMasterDataVersion, 4);
      expect((await store.readAllocation())?.offlineEnabled, isFalse);

      await store.replaceCachedCustomers(
        demoCustomers,
        masterDataVersion: demoDevice.masterDataVersion,
      );
      await store.replaceCachedProducts(
        demoProducts,
        masterDataVersion: demoDevice.masterDataVersion,
        priceVersion: demoDevice.priceVersion,
      );

      final customers = await store.readCachedCustomers();
      final products = await store.readCachedProducts();
      expect(customers, hasLength(demoCustomers.length));
      expect(
        customers
            .firstWhere((customer) => customer.id == 'customer-kijiji')
            .credit,
        isNotNull,
      );
      expect(products, hasLength(demoProducts.length));
      expect(
        products
            .firstWhere((product) => product.id == 'product-cement')
            .sellingPrice,
        1850000,
      );
      expect(
        products
            .firstWhere((product) => product.id == 'product-cement')
            .taxBasisPoints,
        0,
      );

      final draft = _draft();
      await store.saveDraft(draft);
      final restoredDraft = (await store.readDrafts()).single;
      expect(restoredDraft.clientTransactionId, draft.clientTransactionId);
      expect(restoredDraft.customer.id, draft.customer.id);
      expect(restoredDraft.lines.single.product.id, demoProducts.first.id);
      expect(
        restoredDraft.lines.single.unitPrice,
        demoProducts.first.sellingPrice,
      );

      final sale = _sale(draft);
      await store.saveSale(sale);
      final restoredSale = (await store.readSales()).single;
      expect(restoredSale.clientTransactionId, sale.clientTransactionId);
      expect(restoredSale.lines.single.quantity, 2);

      final command = _command(sale);
      await store.enqueueSync(command);
      final queued = (await store.readSyncQueue()).single;
      expect(queued.idempotencyKey, command.idempotencyKey);
      expect(queued.sale.total, sale.total);
      await store.recordSyncFailure(
        sale: sale.copyWith(syncStatus: SyncStatus.reconciliationRequired),
        idempotencyKey: command.idempotencyKey,
        authoritativeRejection: false,
      );
      expect(
        (await store.readSales()).single.syncStatus,
        SyncStatus.reconciliationRequired,
      );
      expect(
        await store.readSyncQueue(),
        hasLength(1),
        reason: 'reconciliation must retain the exact command',
      );

      const result = SyncResult(
        serverSaleId: 'sale-2401',
        receiptReference: 'sale-2401',
        fiscalStatus: FiscalStatus.notConfigured,
        wasDuplicate: false,
      );
      await store.saveSyncResult(command.idempotencyKey, result);
      final restoredResult = await store.readSyncResult(command.idempotencyKey);
      expect(restoredResult?.serverSaleId, result.serverSaleId);
      await store.removeSyncCommand(command.idempotencyKey);
      expect(await store.readSyncQueue(), isEmpty);

      await store.deleteDraft(draft.clientTransactionId);
      expect(await store.readDrafts(), isEmpty);
    },
  );

  test('version 1 database migrates through draft and sync schemas', () async {
    final directory = await Directory.systemTemp.createTemp(
      'itemba-z-migration-',
    );
    addTearDown(() async {
      if (directory.existsSync()) await directory.delete(recursive: true);
    });
    final databasePath = path_utils.join(directory.path, 'migration.db');
    final v1 = await databaseFactoryFfi.openDatabase(
      databasePath,
      options: OpenDatabaseOptions(
        version: 1,
        onConfigure: LocalDatabaseSchema.configure,
        onCreate: LocalDatabaseSchema.create,
      ),
    );
    expect(await v1.getVersion(), 1);
    await v1.close();

    final store = SqlCipherLocalStore(
      databasePath: databasePath,
      keyProvider: DatabaseKeyProvider(vault: _MemorySecretVault()),
      opener: const _FfiTestDatabaseOpener(),
    );
    addTearDown(store.close);
    await store.initialize();

    final draft = _draft();
    final sale = _sale(draft);
    await store.saveDraft(draft);
    await store.saveSale(sale);
    await store.enqueueSync(_command(sale));

    expect(await store.readDrafts(), hasLength(1));
    expect(await store.readSales(), hasLength(1));
    expect(await store.readSyncQueue(), hasLength(1));
  });

  test('version 9 allocation migrates to an expired offline lease', () async {
    final directory = await Directory.systemTemp.createTemp(
      'itemba-z-lease-migration-',
    );
    addTearDown(() async {
      if (directory.existsSync()) await directory.delete(recursive: true);
    });
    final databasePath = path_utils.join(directory.path, 'migration.db');
    final v9 = await databaseFactoryFfi.openDatabase(
      databasePath,
      options: OpenDatabaseOptions(
        version: 9,
        onConfigure: LocalDatabaseSchema.configure,
        onCreate: LocalDatabaseSchema.create,
      ),
    );
    await v9.insert('device_allocation', {
      'singleton_id': 1,
      'offline_enabled': 1,
      'transaction_limit_minor': 100000,
      'daily_value_limit_minor': 200000,
      'remaining_daily_minor': 200000,
      'product_quantities_json': '{}',
      'updated_at_epoch': DateTime.utc(2026, 8, 4).millisecondsSinceEpoch,
    });
    await v9.close();

    final store = SqlCipherLocalStore(
      databasePath: databasePath,
      keyProvider: DatabaseKeyProvider(vault: _MemorySecretVault()),
      opener: const _FfiTestDatabaseOpener(),
    );
    addTearDown(store.close);
    await store.initialize();

    final migrated = await store.readAllocation();
    expect(
      migrated?.offlineSalesValidUntil,
      DateTime.fromMillisecondsSinceEpoch(0, isUtc: true),
    );
    expect(
      migrated?.offlineSalesValidUntil.isBefore(DateTime.now().toUtc()),
      isTrue,
    );
  });

  test(
    'populated v3 database migrates money and quantity to v4 integers',
    () async {
      final fixture = await _StoreFixture.create();
      addTearDown(fixture.dispose);
      await _createPopulatedV3(fixture.store.databasePath);

      await fixture.store.initialize();
      final customer = (await fixture.store.readCachedCustomers()).single;
      final product = (await fixture.store.readCachedProducts()).single;
      final draft = (await fixture.store.readDrafts()).single;
      final sale = (await fixture.store.readSales()).single;
      final queued = (await fixture.store.readSyncQueue()).single;

      expect(customer.credit?.limit, 5000000);
      expect(product.sellingPrice, 18500);
      expect(product.availableQuantity, 148);
      expect(draft.lines.single.unitPrice, 18500);
      expect(draft.lines.single.quantity, 2);
      expect(sale.total, 37000);
      expect(queued.sale.total, 37000);

      await fixture.store.close();
      final migrated = await databaseFactoryFfi.openDatabase(
        fixture.store.databasePath,
      );
      expect(await migrated.getVersion(), LocalDatabaseSchema.version);
      await _expectIntegerColumns(migrated, 'cached_customers', [
        'credit_limit',
        'current_exposure',
        'overdue_amount',
      ]);
      await _expectIntegerColumns(migrated, 'cached_products', [
        'selling_price',
        'available_quantity',
      ]);
      await _expectIntegerColumns(migrated, 'sale_draft_lines', [
        'unit_price',
        'quantity',
      ]);
      await migrated.close();
    },
  );

  test('v3 to v4 migration rolls back on a fractional legacy value', () async {
    final fixture = await _StoreFixture.create();
    addTearDown(fixture.dispose);
    await _createPopulatedV3(fixture.store.databasePath, productPrice: 18500.5);

    await expectLater(fixture.store.initialize(), throwsA(anything));
    await fixture.store.close();

    final unchanged = await databaseFactoryFfi.openDatabase(
      fixture.store.databasePath,
    );
    expect(await unchanged.getVersion(), 3);
    final productRows = await unchanged.query('cached_products');
    expect(productRows.single['selling_price'], 18500.5);
    final lineRows = await unchanged.query('sale_draft_lines');
    final productJson =
        jsonDecode(lineRows.single['product_json']! as String)
            as Map<String, Object?>;
    expect(productJson['sellingPrice'], isA<double>());
    final renamedTables = await unchanged.rawQuery('''
      SELECT name FROM sqlite_master WHERE name LIKE '%_v3'
    ''');
    expect(renamedTables, isEmpty);
    await unchanged.close();
  });
}

Future<void> _createPopulatedV3(
  String databasePath, {
  double productPrice = 18500.0,
}) async {
  final database = await databaseFactoryFfi.openDatabase(
    databasePath,
    options: OpenDatabaseOptions(
      version: 3,
      onConfigure: LocalDatabaseSchema.configure,
      onCreate: LocalDatabaseSchema.create,
    ),
  );
  final now = DateTime(2026, 8, 4, 14, 30).millisecondsSinceEpoch;
  final customer = <String, Object?>{
    'id': 'customer-kijiji',
    'name': 'Kijiji Hardware Ltd',
    'code': 'CUS-0018',
    'isGeneral': false,
    'isActive': true,
    'creditEnabled': true,
    'inAttendantScope': true,
    'phone': '+255 712 880 114',
    'credit': <String, Object?>{
      'limit': 5000000.0,
      'currentExposure': 1840000.0,
      'overdueAmount': 0.0,
      'dueDate': DateTime(2026, 8, 25).millisecondsSinceEpoch,
    },
  };
  final product = <String, Object?>{
    'id': 'product-cement',
    'code': 'CEM-OPC-50',
    'name': 'OPC Cement 50 kg',
    'commonDescription': 'Cement bag',
    'brand': 'Twiga',
    'category': 'Cement',
    'unit': 'Bag',
    'sellingPrice': 18500.0,
    'availableQuantity': 148.0,
  };
  final device = <String, Object?>{
    'deviceId': demoDevice.deviceId,
    'userId': demoDevice.userId,
    'attendantName': demoDevice.attendantName,
    'companyId': demoDevice.companyId,
    'companyName': demoDevice.companyName,
    'branchId': demoDevice.branchId,
    'branchName': demoDevice.branchName,
    'warehouseId': demoDevice.warehouseId,
    'warehouseName': demoDevice.warehouseName,
    'appVersion': demoDevice.appVersion,
    'masterDataVersion': demoDevice.masterDataVersion,
    'priceVersion': demoDevice.priceVersion,
    'approved': true,
  };
  final sale = <String, Object?>{
    'serverSaleId': '',
    'receiptNumber': 'Offline pending',
    'clientTransactionId': 'legacy-client-1',
    'deviceId': demoDevice.deviceId,
    'customer': customer,
    'saleType': 'cash',
    'paymentMethod': 'cash',
    'lines': <Object?>[
      <String, Object?>{
        'product': product,
        'quantity': 2.0,
        'unitPrice': 18500.0,
      },
    ],
    'total': 37000.0,
    'createdAt': now,
    'paymentStatus': 'Paid',
    'syncStatus': 'pendingSync',
    'syncMessage': null,
  };
  final command = <String, Object?>{
    'deviceId': demoDevice.deviceId,
    'userId': demoDevice.userId,
    'clientTransactionId': 'legacy-client-1',
    'clientTimestamp': now,
    'companyId': demoDevice.companyId,
    'branchId': demoDevice.branchId,
    'warehouseId': demoDevice.warehouseId,
    'appVersion': demoDevice.appVersion,
    'masterDataVersion': demoDevice.masterDataVersion,
    'priceVersion': demoDevice.priceVersion,
    'syncAttemptNumber': 1,
    'sale': sale,
  };
  await database.insert('cached_customers', {
    'id': 'customer-kijiji',
    'name': 'Kijiji Hardware Ltd',
    'code': 'CUS-0018',
    'is_general': 0,
    'is_active': 1,
    'credit_enabled': 1,
    'in_attendant_scope': 1,
    'phone': '+255 712 880 114',
    'credit_limit': 5000000.0,
    'current_exposure': 1840000.0,
    'overdue_amount': 0.0,
    'due_date_epoch': DateTime(2026, 8, 25).millisecondsSinceEpoch,
    'master_data_version': demoDevice.masterDataVersion,
    'updated_at_epoch': now,
  });
  await database.insert('cached_products', {
    'id': 'product-cement',
    'code': 'CEM-OPC-50',
    'name': 'OPC Cement 50 kg',
    'common_description': 'Cement bag',
    'brand': 'Twiga',
    'category': 'Cement',
    'unit': 'Bag',
    'selling_price': productPrice,
    'available_quantity': 148.0,
    'master_data_version': demoDevice.masterDataVersion,
    'price_version': demoDevice.priceVersion,
    'updated_at_epoch': now,
  });
  await database.insert('sale_drafts', {
    'client_transaction_id': 'legacy-client-1',
    'created_at_epoch': now,
    'device_json': jsonEncode(device),
    'sale_type': 'cash',
    'customer_json': jsonEncode(customer),
    'payment_method': 'cash',
    'updated_at_epoch': now,
  });
  await database.insert('sale_draft_lines', {
    'client_transaction_id': 'legacy-client-1',
    'line_number': 0,
    'product_json': jsonEncode(product),
    'unit_price': 18500.0,
    'quantity': 2.0,
  });
  await database.insert('local_sales', {
    'client_transaction_id': 'legacy-client-1',
    'device_id': demoDevice.deviceId,
    'created_at_epoch': now,
    'sync_status': 'pendingSync',
    'sale_json': jsonEncode(sale),
    'updated_at_epoch': now,
  });
  await database.insert('sync_queue', {
    'idempotency_key': '${demoDevice.deviceId}::legacy-client-1',
    'device_id': demoDevice.deviceId,
    'client_transaction_id': 'legacy-client-1',
    'user_id': demoDevice.userId,
    'client_timestamp_epoch': now,
    'company_id': demoDevice.companyId,
    'branch_id': demoDevice.branchId,
    'warehouse_id': demoDevice.warehouseId,
    'app_version': demoDevice.appVersion,
    'master_data_version': demoDevice.masterDataVersion,
    'price_version': demoDevice.priceVersion,
    'sync_attempt_number': 1,
    'command_json': jsonEncode(command),
    'queued_at_epoch': now,
  });
  await database.close();
}

Future<void> _expectIntegerColumns(
  Database database,
  String table,
  List<String> columns,
) async {
  final metadata = await database.rawQuery('PRAGMA table_info($table)');
  for (final column in columns) {
    final row = metadata.firstWhere((item) => item['name'] == column);
    expect(row['type'], 'INTEGER');
  }
}

SaleDraft _draft() {
  final draft = SaleDraft(
    clientTransactionId: 'ITZ-DAR-0017-client-9001',
    createdAt: DateTime(2026, 8, 4, 14, 30),
    device: demoDevice,
    saleType: SaleType.cash,
    customer: demoCustomers.first,
  );
  draft.lines.add(CartLine(product: demoProducts.first, quantity: 2));
  return draft;
}

CompletedSale _sale(SaleDraft draft) => CompletedSale(
  serverSaleId: '',
  receiptReference: 'Offline pending',
  clientTransactionId: draft.clientTransactionId,
  deviceId: demoDevice.deviceId,
  customer: draft.customer,
  saleType: draft.saleType,
  paymentMethod: PaymentMethod.cash,
  lines: List.unmodifiable(draft.lines),
  total: draft.total,
  createdAt: draft.createdAt,
  paymentStatus: 'Paid',
  syncStatus: SyncStatus.pendingSync,
);

SyncCommand _command(CompletedSale sale) => SyncCommand(
  deviceId: demoDevice.deviceId,
  userId: demoDevice.userId,
  clientTransactionId: sale.clientTransactionId,
  clientTimestamp: sale.createdAt,
  companyId: demoDevice.companyId,
  branchId: demoDevice.branchId,
  warehouseId: demoDevice.warehouseId,
  appVersion: demoDevice.appVersion,
  catalogSnapshotToken: demoDevice.catalogSnapshotToken,
  masterDataVersion: demoDevice.masterDataVersion,
  priceVersion: demoDevice.priceVersion,
  syncAttemptNumber: 1,
  sale: sale,
);

class _MemorySecretVault implements DatabaseSecretVault {
  final Map<String, String> values = {};
  int writeCount = 0;

  @override
  Future<String?> read(String key) async => values[key];

  @override
  Future<void> write(String key, String value) async {
    writeCount += 1;
    values[key] = value;
  }

  @override
  Future<void> delete(String key) async {
    values.remove(key);
  }
}

class _FfiTestDatabaseOpener implements EncryptedDatabaseOpener {
  const _FfiTestDatabaseOpener();

  @override
  Future<Database> open({
    required String path,
    required String password,
    required int version,
    required OnDatabaseConfigureFn onConfigure,
    required OnDatabaseCreateFn onCreate,
    required OnDatabaseVersionChangeFn onUpgrade,
  }) {
    if (password.isEmpty) throw StateError('A database secret is required.');
    return databaseFactoryFfi.openDatabase(
      path,
      options: OpenDatabaseOptions(
        version: version,
        onConfigure: onConfigure,
        onCreate: onCreate,
        onUpgrade: onUpgrade,
      ),
    );
  }
}

class _StoreFixture {
  _StoreFixture({required this.directory, required this.store});

  final Directory directory;
  final SqlCipherLocalStore store;

  static Future<_StoreFixture> create() async {
    final directory = await Directory.systemTemp.createTemp('itemba-z-store-');
    return _StoreFixture(
      directory: directory,
      store: SqlCipherLocalStore(
        databasePath: path_utils.join(directory.path, 'sales.db'),
        keyProvider: DatabaseKeyProvider(vault: _MemorySecretVault()),
        opener: const _FfiTestDatabaseOpener(),
      ),
    );
  }

  Future<void> dispose() async {
    await store.close();
    if (directory.existsSync()) await directory.delete(recursive: true);
  }
}
