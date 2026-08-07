import 'dart:convert';

import 'package:path/path.dart' as path_utils;
import 'package:sqflite_sqlcipher/sqflite.dart' as sqlcipher;
import 'package:sqflite_sqlcipher/sqlite_api.dart';

import '../domain/models.dart';
import '../domain/connection_models.dart';
import 'database_key_provider.dart';
import 'encrypted_database_opener.dart';
import 'local_data_codec.dart';
import 'local_database_schema.dart';
import 'local_store.dart';

class SqlCipherLocalStore implements EncryptedLocalStore {
  SqlCipherLocalStore({
    required this.databasePath,
    required this.keyProvider,
    EncryptedDatabaseOpener opener = const SqlCipherDatabaseOpener(),
  }) : _opener = opener;

  final String databasePath;
  final DatabaseKeyProvider keyProvider;
  final EncryptedDatabaseOpener _opener;
  Future<Database>? _databaseFuture;

  static Future<SqlCipherLocalStore> production() async {
    final databasesPath = await sqlcipher.getDatabasesPath();
    return SqlCipherLocalStore(
      databasePath: path_utils.join(databasesPath, 'itemba_z_sales_v1.db'),
      keyProvider: DatabaseKeyProvider(vault: AndroidKeystoreSecretVault()),
    );
  }

  Future<Database> get _database => _databaseFuture ??= _open();

  Future<Database> _open() async {
    final password = await keyProvider.getOrCreate();
    return _opener.open(
      path: databasePath,
      password: password,
      version: LocalDatabaseSchema.version,
      onConfigure: LocalDatabaseSchema.configure,
      onCreate: LocalDatabaseSchema.create,
      onUpgrade: LocalDatabaseSchema.migrate,
    );
  }

  @override
  Future<void> initialize() async {
    await _database;
  }

  @override
  Future<void> saveConnection(MobileConnection connection) async {
    final database = await _database;
    final development = connection.developmentIdentity;
    await database.insert('mobile_connection', {
      'singleton_id': 1,
      'base_url': connection.baseUrl.toString(),
      'identity_mode': connection.identityMode.name,
      'dev_actor_id': development?.actorId,
      'dev_tenant_id': development?.tenantId,
      'dev_company_id': development?.companyId,
      'dev_branch_id': development?.branchId,
      'dev_warehouse_id': development?.warehouseId,
      'updated_at_epoch': DateTime.now().millisecondsSinceEpoch,
    }, conflictAlgorithm: ConflictAlgorithm.replace);
  }

  @override
  Future<MobileConnection?> readConnection() async {
    final database = await _database;
    final rows = await database.query(
      'mobile_connection',
      where: 'singleton_id = 1',
      limit: 1,
    );
    if (rows.isEmpty) return null;
    final row = rows.single;
    final mode = MobileIdentityMode.values.byName(
      row['identity_mode']! as String,
    );
    return MobileConnection(
      baseUrl: Uri.parse(row['base_url']! as String),
      identityMode: mode,
      developmentIdentity:
          mode == MobileIdentityMode.developmentHeaders
              ? DevelopmentIdentity(
                actorId: row['dev_actor_id']! as String,
                tenantId: row['dev_tenant_id']! as String,
                companyId: row['dev_company_id']! as String,
                branchId: row['dev_branch_id']! as String,
                warehouseId: row['dev_warehouse_id']! as String,
              )
              : null,
    );
  }

  @override
  Future<void> saveCandidateDeviceId(String deviceId) async {
    final database = await _database;
    await database.insert('mobile_device_identity', {
      'singleton_id': 1,
      'candidate_device_id': deviceId,
      'updated_at_epoch': DateTime.now().millisecondsSinceEpoch,
    }, conflictAlgorithm: ConflictAlgorithm.replace);
  }

  @override
  Future<String?> readCandidateDeviceId() async {
    final database = await _database;
    final rows = await database.query(
      'mobile_device_identity',
      columns: ['candidate_device_id'],
      where: 'singleton_id = 1',
      limit: 1,
    );
    return rows.isEmpty ? null : rows.single['candidate_device_id'] as String;
  }

  @override
  Future<void> saveEnrollment(DeviceEnrollment enrollment) async {
    final database = await _database;
    await database.insert(
      'device_enrollment',
      _enrollmentRow(enrollment),
      conflictAlgorithm: ConflictAlgorithm.replace,
    );
  }

  @override
  Future<DeviceEnrollment?> readEnrollment() async {
    final database = await _database;
    final rows = await database.query(
      'device_enrollment',
      where: 'singleton_id = 1',
      limit: 1,
    );
    if (rows.isEmpty) return null;
    final row = rows.single;
    final allocationValues =
        jsonDecode(row['stock_allocations_json']! as String) as List<Object?>;
    return DeviceEnrollment(
      deviceId: row['device_id']! as String,
      deviceName: row['device_name']! as String,
      status: row['status']! as String,
      actorId: row['actor_id']! as String,
      scope: EnrollmentScope(
        tenantId: row['tenant_id']! as String,
        companyId: row['company_id']! as String,
        branchId: row['branch_id']! as String,
        warehouseId: row['warehouse_id']! as String,
      ),
      appVersion: row['app_version']! as String,
      catalogSnapshotToken: row['catalog_snapshot_token']! as String,
      masterDataVersion: _exactInt(
        row['master_data_version'],
        'device_enrollment.master_data_version',
      ),
      priceVersion: _exactInt(
        row['price_version'],
        'device_enrollment.price_version',
      ),
      availableMasterDataVersion: _exactInt(
        row['available_master_data_version'],
        'device_enrollment.available_master_data_version',
      ),
      availablePriceVersion: _exactInt(
        row['available_price_version'],
        'device_enrollment.available_price_version',
      ),
      availableCatalogSnapshotToken:
          row['available_catalog_snapshot_token']! as String,
      timezone: row['timezone']! as String,
      offlineEnabled: row['offline_enabled'] == 1,
      transactionValueLimitMinor: _exactInt(
        row['transaction_value_limit_minor'],
        'device_enrollment.transaction_value_limit_minor',
      ),
      dailyValueLimitMinor: _exactInt(
        row['daily_value_limit_minor'],
        'device_enrollment.daily_value_limit_minor',
      ),
      remainingDailyValueMinor: _exactInt(
        row['remaining_daily_value_minor'],
        'device_enrollment.remaining_daily_value_minor',
      ),
      offlineSalesValidUntil: DateTime.fromMillisecondsSinceEpoch(
        row['offline_sales_valid_until_epoch']! as int,
        isUtc: true,
      ),
      stockAllocations: allocationValues
          .map((value) {
            final allocation = value! as Map<String, Object?>;
            return MobileStockAllocation(
              productId: allocation['product_id']! as String,
              allocatedQuantity: _exactInt(
                allocation['allocated_quantity'],
                'device_enrollment.stock_allocations.allocated_quantity',
              ),
              remainingQuantity: _exactInt(
                allocation['remaining_quantity'],
                'device_enrollment.stock_allocations.remaining_quantity',
              ),
            );
          })
          .toList(growable: false),
      enrolledAt: DateTime.fromMillisecondsSinceEpoch(
        row['enrolled_at_epoch']! as int,
        isUtc: true,
      ),
      lastSeenAt: DateTime.fromMillisecondsSinceEpoch(
        row['last_seen_at_epoch']! as int,
        isUtc: true,
      ),
    );
  }

  @override
  Future<void> saveAllocation(DeviceAllocation allocation) async {
    final database = await _database;
    await database.insert('device_allocation', {
      'singleton_id': 1,
      'offline_enabled': _bool(allocation.offlineEnabled),
      'transaction_limit_minor': allocation.transactionLimitMinor,
      'daily_value_limit_minor': allocation.dailyValueLimitMinor,
      'remaining_daily_minor': allocation.remainingDailyMinor,
      'offline_sales_valid_until_epoch':
          allocation.offlineSalesValidUntil.millisecondsSinceEpoch,
      'product_quantities_json': jsonEncode(allocation.productQuantities),
      'updated_at_epoch': allocation.updatedAt.millisecondsSinceEpoch,
    }, conflictAlgorithm: ConflictAlgorithm.replace);
  }

  @override
  Future<DeviceAllocation?> readAllocation() async {
    final database = await _database;
    final rows = await database.query(
      'device_allocation',
      where: 'singleton_id = 1',
      limit: 1,
    );
    if (rows.isEmpty) return null;
    final row = rows.single;
    final quantities = (jsonDecode(row['product_quantities_json']! as String)
            as Map<String, Object?>)
        .map(
          (key, value) => MapEntry(
            key,
            _exactInt(value, 'device_allocation.product_quantities.$key'),
          ),
        );
    return DeviceAllocation(
      offlineEnabled: row['offline_enabled'] == 1,
      transactionLimitMinor: _exactInt(
        row['transaction_limit_minor'],
        'device_allocation.transaction_limit_minor',
      ),
      dailyValueLimitMinor: _exactInt(
        row['daily_value_limit_minor'],
        'device_allocation.daily_value_limit_minor',
      ),
      remainingDailyMinor: _exactInt(
        row['remaining_daily_minor'],
        'device_allocation.remaining_daily_minor',
      ),
      offlineSalesValidUntil: DateTime.fromMillisecondsSinceEpoch(
        row['offline_sales_valid_until_epoch']! as int,
        isUtc: true,
      ),
      productQuantities: quantities,
      updatedAt: DateTime.fromMillisecondsSinceEpoch(
        row['updated_at_epoch']! as int,
        isUtc: true,
      ),
    );
  }

  @override
  Future<void> replaceCachedCustomers(
    List<Customer> customers, {
    required int masterDataVersion,
  }) async {
    final database = await _database;
    final updatedAt = DateTime.now().millisecondsSinceEpoch;
    await database.transaction((transaction) async {
      await transaction.delete('cached_customers');
      for (final customer in customers) {
        await transaction.insert(
          'cached_customers',
          _customerRow(customer, masterDataVersion, updatedAt),
        );
      }
    });
  }

  @override
  Future<List<Customer>> readCachedCustomers() async {
    final database = await _database;
    final rows = await database.query('cached_customers', orderBy: 'name ASC');
    return rows.map(_customerFromRow).toList(growable: false);
  }

  @override
  Future<void> replaceCachedProducts(
    List<Product> products, {
    required int masterDataVersion,
    required int priceVersion,
  }) async {
    final database = await _database;
    final updatedAt = DateTime.now().millisecondsSinceEpoch;
    await database.transaction((transaction) async {
      await transaction.delete('cached_products');
      for (final product in products) {
        await transaction.insert(
          'cached_products',
          _productRow(product, masterDataVersion, priceVersion, updatedAt),
        );
      }
    });
  }

  @override
  Future<List<Product>> readCachedProducts() async {
    final database = await _database;
    final rows = await database.query('cached_products', orderBy: 'name ASC');
    return rows.map(_productFromRow).toList(growable: false);
  }

  @override
  Future<InstalledMasterDataVersions?> readInstalledMasterDataVersions() async {
    final database = await _database;
    final installStates = await database.query(
      'catalog_install_state',
      where: 'singleton_id = 1',
      limit: 1,
    );
    final customerVersions = await database.rawQuery(
      'SELECT DISTINCT master_data_version FROM cached_customers',
    );
    final productVersions = await database.rawQuery(
      'SELECT DISTINCT master_data_version, price_version FROM cached_products',
    );
    if (installStates.length != 1 ||
        customerVersions.length != 1 ||
        productVersions.length != 1) {
      return null;
    }
    final customerMaster = _versionInt(
      customerVersions.single['master_data_version'],
    );
    final productMaster = _versionInt(
      productVersions.single['master_data_version'],
    );
    final installState = installStates.single;
    final installedMaster = _versionInt(installState['master_data_version']);
    final installedPrice = _versionInt(installState['price_version']);
    final productPrice = _versionInt(productVersions.single['price_version']);
    final catalogSnapshotToken =
        installState['catalog_snapshot_token']! as String;
    if (customerMaster != productMaster ||
        installedMaster != customerMaster ||
        installedPrice != productPrice ||
        catalogSnapshotToken == unacknowledgedCatalogSnapshotToken) {
      return null;
    }
    return InstalledMasterDataVersions(
      catalogSnapshotToken: catalogSnapshotToken,
      masterDataVersion: customerMaster,
      priceVersion: productPrice,
    );
  }

  @override
  Future<void> installMasterData({
    required List<Customer> customers,
    required List<Product> products,
    required int masterDataVersion,
    required int priceVersion,
    required String catalogSnapshotToken,
    DeviceEnrollment? enrollment,
  }) async {
    final database = await _database;
    final updatedAt = DateTime.now().millisecondsSinceEpoch;
    await database.transaction((transaction) async {
      await transaction.delete('cached_customers');
      for (final customer in customers) {
        await transaction.insert(
          'cached_customers',
          _customerRow(customer, masterDataVersion, updatedAt),
        );
      }
      await transaction.delete('cached_products');
      for (final product in products) {
        await transaction.insert(
          'cached_products',
          _productRow(product, masterDataVersion, priceVersion, updatedAt),
        );
      }
      await transaction.insert('catalog_install_state', {
        'singleton_id': 1,
        'catalog_snapshot_token': catalogSnapshotToken,
        'master_data_version': masterDataVersion,
        'price_version': priceVersion,
        'installed_at_epoch': updatedAt,
      }, conflictAlgorithm: ConflictAlgorithm.replace);
      if (enrollment != null) {
        await transaction.insert(
          'device_enrollment',
          _enrollmentRow(enrollment),
          conflictAlgorithm: ConflictAlgorithm.replace,
        );
      }
    });
  }

  @override
  Future<void> saveDraft(SaleDraft draft) async {
    final database = await _database;
    await database.transaction((transaction) async {
      await transaction.insert('sale_drafts', {
        'client_transaction_id': draft.clientTransactionId,
        'created_at_epoch': draft.createdAt.millisecondsSinceEpoch,
        'device_json': LocalDataCodec.encodeDevice(draft.device),
        'sale_type': draft.saleType.name,
        'customer_json': LocalDataCodec.encodeCustomer(draft.customer),
        'payment_method': draft.paymentMethod.name,
        'updated_at_epoch': DateTime.now().millisecondsSinceEpoch,
      }, conflictAlgorithm: ConflictAlgorithm.replace);
      await transaction.delete(
        'sale_draft_lines',
        where: 'client_transaction_id = ?',
        whereArgs: [draft.clientTransactionId],
      );
      for (var index = 0; index < draft.lines.length; index++) {
        final line = draft.lines[index];
        await transaction.insert('sale_draft_lines', {
          'client_transaction_id': draft.clientTransactionId,
          'line_number': index,
          'product_json': LocalDataCodec.encodeProduct(line.product),
          'unit_price': line.unitPrice,
          'quantity': line.quantity,
        });
      }
    });
  }

  @override
  Future<List<SaleDraft>> readDrafts() async {
    final database = await _database;
    final rows = await database.query(
      'sale_drafts',
      orderBy: 'created_at_epoch DESC',
    );
    final drafts = <SaleDraft>[];
    for (final row in rows) {
      final transactionId = row['client_transaction_id']! as String;
      final draft = SaleDraft(
          clientTransactionId: transactionId,
          createdAt: DateTime.fromMillisecondsSinceEpoch(
            row['created_at_epoch']! as int,
          ),
          device: LocalDataCodec.decodeDevice(row['device_json']! as String),
          saleType: SaleType.values.byName(row['sale_type']! as String),
          customer: LocalDataCodec.decodeCustomer(
            row['customer_json']! as String,
          ),
        )
        ..paymentMethod = PaymentMethod.values.byName(
          row['payment_method']! as String,
        );
      final lineRows = await database.query(
        'sale_draft_lines',
        where: 'client_transaction_id = ?',
        whereArgs: [transactionId],
        orderBy: 'line_number ASC',
      );
      for (final lineRow in lineRows) {
        final stored = LocalDataCodec.decodeProduct(
          lineRow['product_json']! as String,
        );
        final capturedPrice = _exactInt(
          lineRow['unit_price'],
          'sale_draft_lines.unit_price',
        );
        final product = Product(
          id: stored.id,
          code: stored.code,
          name: stored.name,
          commonDescription: stored.commonDescription,
          brand: stored.brand,
          category: stored.category,
          unit: stored.unit,
          sellingPrice: capturedPrice,
          availableQuantity: stored.availableQuantity,
          taxBasisPoints: stored.taxBasisPoints,
          masterDataVersion: stored.masterDataVersion,
          priceVersion: stored.priceVersion,
        );
        draft.lines.add(
          CartLine(
            product: product,
            quantity: _exactInt(
              lineRow['quantity'],
              'sale_draft_lines.quantity',
            ),
          ),
        );
      }
      drafts.add(draft);
    }
    return drafts;
  }

  @override
  Future<void> deleteDraft(String clientTransactionId) async {
    final database = await _database;
    await database.delete(
      'sale_drafts',
      where: 'client_transaction_id = ?',
      whereArgs: [clientTransactionId],
    );
  }

  @override
  Future<void> saveSale(CompletedSale sale) async {
    final database = await _database;
    await database.insert(
      'local_sales',
      _saleRow(sale),
      conflictAlgorithm: ConflictAlgorithm.ignore,
    );
  }

  @override
  Future<void> persistPendingSync({
    required CompletedSale sale,
    required SyncCommand command,
  }) async {
    final database = await _database;
    await database.transaction((transaction) async {
      await transaction.insert(
        'local_sales',
        _saleRow(sale),
        conflictAlgorithm: ConflictAlgorithm.ignore,
      );
      await transaction.insert(
        'sync_queue',
        _syncQueueRow(command),
        conflictAlgorithm: ConflictAlgorithm.replace,
      );
      await transaction.delete(
        'sale_drafts',
        where: 'client_transaction_id = ?',
        whereArgs: [sale.clientTransactionId],
      );
    });
  }

  @override
  Future<void> persistOfflineCompletion({
    required CompletedSale sale,
    required SyncCommand command,
    required DeviceAllocation allocation,
  }) async {
    final database = await _database;
    await database.transaction((transaction) async {
      await transaction.insert(
        'local_sales',
        _saleRow(sale),
        conflictAlgorithm: ConflictAlgorithm.ignore,
      );
      await transaction.insert(
        'sync_queue',
        _syncQueueRow(command),
        conflictAlgorithm: ConflictAlgorithm.replace,
      );
      await transaction.insert(
        'device_allocation',
        _allocationRow(allocation),
        conflictAlgorithm: ConflictAlgorithm.replace,
      );
      await transaction.delete(
        'sale_drafts',
        where: 'client_transaction_id = ?',
        whereArgs: [sale.clientTransactionId],
      );
    });
  }

  @override
  Future<List<CompletedSale>> readSales() async {
    final database = await _database;
    final rows = await database.query(
      'local_sales',
      orderBy: 'created_at_epoch DESC',
    );
    return rows
        .map((row) => LocalDataCodec.decodeSale(row['sale_json']! as String))
        .toList(growable: false);
  }

  @override
  Future<void> replaceSale(CompletedSale sale) async {
    final database = await _database;
    await database.insert(
      'local_sales',
      _saleRow(sale),
      conflictAlgorithm: ConflictAlgorithm.replace,
    );
  }

  @override
  Future<void> enqueueSync(SyncCommand command) async {
    final database = await _database;
    await database.insert(
      'sync_queue',
      _syncQueueRow(command),
      conflictAlgorithm: ConflictAlgorithm.replace,
    );
  }

  @override
  Future<List<SyncCommand>> readSyncQueue() async {
    final database = await _database;
    final rows = await database.query(
      'sync_queue',
      orderBy: 'queued_at_epoch ASC',
    );
    return rows
        .map(
          (row) =>
              LocalDataCodec.decodeSyncCommand(row['command_json']! as String),
        )
        .toList(growable: false);
  }

  @override
  Future<void> removeSyncCommand(String idempotencyKey) async {
    final database = await _database;
    await database.delete(
      'sync_queue',
      where: 'idempotency_key = ?',
      whereArgs: [idempotencyKey],
    );
  }

  @override
  Future<void> saveSyncResult(String idempotencyKey, SyncResult result) async {
    final database = await _database;
    await database.insert('sync_results', {
      'idempotency_key': idempotencyKey,
      'server_sale_id': result.serverSaleId,
      'receipt_reference': result.receiptReference,
      'fiscal_status': result.fiscalStatus.name,
      'server_total_minor': result.serverTotalMinor,
      'recorded_at_epoch': DateTime.now().millisecondsSinceEpoch,
    }, conflictAlgorithm: ConflictAlgorithm.ignore);
  }

  @override
  Future<SyncResult?> readSyncResult(String idempotencyKey) async {
    final database = await _database;
    final rows = await database.query(
      'sync_results',
      where: 'idempotency_key = ?',
      whereArgs: [idempotencyKey],
      limit: 1,
    );
    if (rows.isEmpty) return null;
    final row = rows.single;
    return SyncResult(
      serverSaleId: row['server_sale_id']! as String,
      receiptReference: row['receipt_reference']! as String,
      fiscalStatus: FiscalStatus.values.byName(row['fiscal_status']! as String),
      serverTotalMinor:
          row['server_total_minor'] == null
              ? null
              : _exactInt(
                row['server_total_minor'],
                'sync_results.server_total_minor',
              ),
      wasDuplicate: true,
    );
  }

  @override
  Future<void> finalizeSync({
    required CompletedSale sale,
    required SyncCommand command,
    required SyncResult result,
  }) async {
    final database = await _database;
    await database.transaction((transaction) async {
      await transaction.insert('sync_results', {
        'idempotency_key': command.idempotencyKey,
        'server_sale_id': result.serverSaleId,
        'receipt_reference': result.receiptReference,
        'fiscal_status': result.fiscalStatus.name,
        'server_total_minor': result.serverTotalMinor,
        'recorded_at_epoch': DateTime.now().millisecondsSinceEpoch,
      }, conflictAlgorithm: ConflictAlgorithm.ignore);
      await transaction.insert(
        'local_sales',
        _saleRow(sale),
        conflictAlgorithm: ConflictAlgorithm.replace,
      );
      await transaction.delete(
        'sync_queue',
        where: 'idempotency_key = ?',
        whereArgs: [command.idempotencyKey],
      );
    });
  }

  @override
  Future<void> recordSyncFailure({
    required CompletedSale sale,
    required String idempotencyKey,
    required bool authoritativeRejection,
  }) async {
    final database = await _database;
    await database.transaction((transaction) async {
      await transaction.insert(
        'local_sales',
        _saleRow(sale),
        conflictAlgorithm: ConflictAlgorithm.replace,
      );
      if (authoritativeRejection) {
        await transaction.delete(
          'sync_queue',
          where: 'idempotency_key = ?',
          whereArgs: [idempotencyKey],
        );
      }
    });
  }

  @override
  Future<void> clearSensitiveCache() async {
    final database = await _database;
    await database.transaction((transaction) async {
      await transaction.delete('device_allocation');
      await transaction.delete('device_enrollment');
      await transaction.delete('mobile_connection');
      await transaction.delete('mobile_device_identity');
      await transaction.delete('sync_queue');
      await transaction.delete('sync_results');
      await transaction.delete('local_sales');
      await transaction.delete('sale_drafts');
      await transaction.delete('cached_products');
      await transaction.delete('cached_customers');
      await transaction.delete('catalog_install_state');
    });
  }

  @override
  Future<void> close() async {
    final databaseFuture = _databaseFuture;
    if (databaseFuture == null) return;
    _databaseFuture = null;
    try {
      final database = await databaseFuture;
      await database.close();
    } catch (_) {
      // A failed open has no database handle to close. Preserve the original
      // initialization error at its call site and make cleanup idempotent.
    }
  }

  Map<String, Object?> _saleRow(CompletedSale sale) => {
    'client_transaction_id': sale.clientTransactionId,
    'device_id': sale.deviceId,
    'created_at_epoch': sale.createdAt.millisecondsSinceEpoch,
    'sync_status': sale.syncStatus.name,
    'sale_json': LocalDataCodec.encodeSale(sale),
    'updated_at_epoch': DateTime.now().millisecondsSinceEpoch,
  };

  Map<String, Object?> _syncQueueRow(SyncCommand command) => {
    'idempotency_key': command.idempotencyKey,
    'device_id': command.deviceId,
    'client_transaction_id': command.clientTransactionId,
    'user_id': command.userId,
    'client_timestamp_epoch': command.clientTimestamp.millisecondsSinceEpoch,
    'company_id': command.companyId,
    'branch_id': command.branchId,
    'warehouse_id': command.warehouseId,
    'app_version': command.appVersion,
    'master_data_version': command.masterDataVersion,
    'price_version': command.priceVersion,
    'catalog_snapshot_token': command.catalogSnapshotToken,
    'sync_attempt_number': command.syncAttemptNumber,
    'command_json': LocalDataCodec.encodeSyncCommand(command),
    'queued_at_epoch': DateTime.now().millisecondsSinceEpoch,
  };

  Map<String, Object?> _allocationRow(DeviceAllocation allocation) => {
    'singleton_id': 1,
    'offline_enabled': _bool(allocation.offlineEnabled),
    'transaction_limit_minor': allocation.transactionLimitMinor,
    'daily_value_limit_minor': allocation.dailyValueLimitMinor,
    'remaining_daily_minor': allocation.remainingDailyMinor,
    'offline_sales_valid_until_epoch':
        allocation.offlineSalesValidUntil.millisecondsSinceEpoch,
    'product_quantities_json': jsonEncode(allocation.productQuantities),
    'updated_at_epoch': allocation.updatedAt.millisecondsSinceEpoch,
  };

  Map<String, Object?> _enrollmentRow(DeviceEnrollment enrollment) => {
    'singleton_id': 1,
    'device_id': enrollment.deviceId,
    'device_name': enrollment.deviceName,
    'status': enrollment.status,
    'actor_id': enrollment.actorId,
    'tenant_id': enrollment.scope.tenantId,
    'company_id': enrollment.scope.companyId,
    'branch_id': enrollment.scope.branchId,
    'warehouse_id': enrollment.scope.warehouseId,
    'app_version': enrollment.appVersion,
    'catalog_snapshot_token': enrollment.catalogSnapshotToken,
    'master_data_version': enrollment.masterDataVersion,
    'price_version': enrollment.priceVersion,
    'available_master_data_version': enrollment.availableMasterDataVersion,
    'available_price_version': enrollment.availablePriceVersion,
    'available_catalog_snapshot_token':
        enrollment.availableCatalogSnapshotToken,
    'timezone': enrollment.timezone,
    'offline_enabled': _bool(enrollment.offlineEnabled),
    'transaction_value_limit_minor': enrollment.transactionValueLimitMinor,
    'daily_value_limit_minor': enrollment.dailyValueLimitMinor,
    'remaining_daily_value_minor': enrollment.remainingDailyValueMinor,
    'offline_sales_valid_until_epoch':
        enrollment.offlineSalesValidUntil.millisecondsSinceEpoch,
    'stock_allocations_json': jsonEncode([
      for (final allocation in enrollment.stockAllocations)
        {
          'product_id': allocation.productId,
          'allocated_quantity': allocation.allocatedQuantity,
          'remaining_quantity': allocation.remainingQuantity,
        },
    ]),
    'enrolled_at_epoch': enrollment.enrolledAt.millisecondsSinceEpoch,
    'last_seen_at_epoch': enrollment.lastSeenAt.millisecondsSinceEpoch,
  };

  Map<String, Object?> _customerRow(
    Customer customer,
    int masterDataVersion,
    int updatedAt,
  ) {
    final credit = customer.credit;
    return {
      'id': customer.id,
      'name': customer.name,
      'code': customer.code,
      'is_general': _bool(customer.isGeneral),
      'is_active': _bool(customer.isActive),
      'credit_enabled': _bool(customer.creditEnabled),
      'in_attendant_scope': _bool(customer.inAttendantScope),
      'phone': customer.phone,
      'credit_limit': credit?.limit,
      'current_exposure': credit?.currentExposure,
      'overdue_amount': credit?.overdueAmount,
      'due_date_epoch': credit?.dueDate?.millisecondsSinceEpoch,
      'master_data_version': masterDataVersion,
      'updated_at_epoch': updatedAt,
    };
  }

  Map<String, Object?> _productRow(
    Product product,
    int masterDataVersion,
    int priceVersion,
    int updatedAt,
  ) => {
    'id': product.id,
    'code': product.code,
    'name': product.name,
    'common_description': product.commonDescription,
    'brand': product.brand,
    'category': product.category,
    'unit': product.unit,
    'selling_price': product.sellingPrice,
    'available_quantity': product.availableQuantity,
    'tax_basis_points': product.taxBasisPoints,
    'master_data_version': masterDataVersion,
    'price_version': priceVersion,
    'updated_at_epoch': updatedAt,
  };

  Customer _customerFromRow(Map<String, Object?> row) {
    final limit = row['credit_limit'];
    return Customer(
      id: row['id']! as String,
      name: row['name']! as String,
      code: row['code']! as String,
      isGeneral: row['is_general'] == 1,
      isActive: row['is_active'] == 1,
      creditEnabled: row['credit_enabled'] == 1,
      inAttendantScope: row['in_attendant_scope'] == 1,
      phone: row['phone'] as String?,
      credit:
          limit == null
              ? null
              : CreditSnapshot(
                limit: _exactInt(limit, 'cached_customers.credit_limit'),
                currentExposure: _exactInt(
                  row['current_exposure'],
                  'cached_customers.current_exposure',
                ),
                overdueAmount: _exactInt(
                  row['overdue_amount'],
                  'cached_customers.overdue_amount',
                ),
                dueDate:
                    row['due_date_epoch'] == null
                        ? null
                        : DateTime.fromMillisecondsSinceEpoch(
                          row['due_date_epoch']! as int,
                        ),
              ),
    );
  }

  Product _productFromRow(Map<String, Object?> row) => Product(
    id: row['id']! as String,
    code: row['code']! as String,
    name: row['name']! as String,
    commonDescription: row['common_description']! as String,
    brand: row['brand']! as String,
    category: row['category']! as String,
    unit: row['unit']! as String,
    sellingPrice: _exactInt(
      row['selling_price'],
      'cached_products.selling_price',
    ),
    availableQuantity: _exactInt(
      row['available_quantity'],
      'cached_products.available_quantity',
    ),
    taxBasisPoints:
        row['tax_basis_points'] == null
            ? null
            : _exactInt(
              row['tax_basis_points'],
              'cached_products.tax_basis_points',
            ),
    masterDataVersion: _versionInt(row['master_data_version']),
    priceVersion: _versionInt(row['price_version']),
  );

  int _bool(bool value) => value ? 1 : 0;

  int _exactInt(Object? value, String field) {
    if (value is int) return value;
    if (value is double &&
        value.isFinite &&
        value == value.truncateToDouble()) {
      return value.toInt();
    }
    throw StateError('$field must be an exact integer');
  }

  int _versionInt(Object? value) {
    if (value is int && value > 0) return value;
    if (value is String) return int.tryParse(value) ?? 1;
    return 1;
  }
}
