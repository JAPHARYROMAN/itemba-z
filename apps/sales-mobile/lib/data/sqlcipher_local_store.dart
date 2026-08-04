import 'package:path/path.dart' as path_utils;
import 'package:sqflite_sqlcipher/sqflite.dart' as sqlcipher;
import 'package:sqflite_sqlcipher/sqlite_api.dart';

import '../domain/models.dart';
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
  Future<void> replaceCachedCustomers(
    List<Customer> customers, {
    required String masterDataVersion,
  }) async {
    final database = await _database;
    final updatedAt = DateTime.now().millisecondsSinceEpoch;
    await database.transaction((transaction) async {
      await transaction.delete('cached_customers');
      for (final customer in customers) {
        final credit = customer.credit;
        await transaction.insert('cached_customers', {
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
          'due_date_epoch': credit?.dueDate.millisecondsSinceEpoch,
          'master_data_version': masterDataVersion,
          'updated_at_epoch': updatedAt,
        });
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
    required String masterDataVersion,
    required String priceVersion,
  }) async {
    final database = await _database;
    final updatedAt = DateTime.now().millisecondsSinceEpoch;
    await database.transaction((transaction) async {
      await transaction.delete('cached_products');
      for (final product in products) {
        await transaction.insert('cached_products', {
          'id': product.id,
          'code': product.code,
          'name': product.name,
          'common_description': product.commonDescription,
          'brand': product.brand,
          'category': product.category,
          'unit': product.unit,
          'selling_price': product.sellingPrice,
          'available_quantity': product.availableQuantity,
          'master_data_version': masterDataVersion,
          'price_version': priceVersion,
          'updated_at_epoch': updatedAt,
        });
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
    await database.insert('sync_queue', {
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
      'sync_attempt_number': command.syncAttemptNumber,
      'command_json': LocalDataCodec.encodeSyncCommand(command),
      'queued_at_epoch': DateTime.now().millisecondsSinceEpoch,
    }, conflictAlgorithm: ConflictAlgorithm.replace);
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
      'receipt_number': result.receiptNumber,
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
      receiptNumber: row['receipt_number']! as String,
      wasDuplicate: true,
    );
  }

  @override
  Future<void> clearSensitiveCache() async {
    final database = await _database;
    await database.transaction((transaction) async {
      await transaction.delete('sync_queue');
      await transaction.delete('sync_results');
      await transaction.delete('local_sales');
      await transaction.delete('sale_drafts');
      await transaction.delete('cached_products');
      await transaction.delete('cached_customers');
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
                dueDate: DateTime.fromMillisecondsSinceEpoch(
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
}
