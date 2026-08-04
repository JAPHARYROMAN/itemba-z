import 'dart:convert';

import 'package:sqflite_sqlcipher/sqlite_api.dart';

class LocalDatabaseSchema {
  const LocalDatabaseSchema._();

  static const version = 10;

  static Future<void> configure(Database database) async {
    await database.execute('PRAGMA foreign_keys = ON');
    await database.execute('PRAGMA secure_delete = ON');
    await database.execute('PRAGMA cipher_memory_security = ON');
  }

  static Future<void> create(Database database, int targetVersion) async {
    final exactIntegers = targetVersion >= 4;
    await _createCustomerTable(database, exactIntegers: exactIntegers);
    await _createProductTable(database, exactIntegers: exactIntegers);
    if (targetVersion >= 2) {
      await _createDraftTables(database, exactIntegers: exactIntegers);
    }
    if (targetVersion >= 3) await _createSalesAndSyncTables(database);
    if (targetVersion >= 5) {
      await _createMobileIdentityTables(
        database,
        includeOfflineLease: targetVersion >= 10,
      );
    }
    if (targetVersion >= 8) await _createCandidateDeviceTable(database);
  }

  /// Sqflite invokes this callback inside the same transaction used to update
  /// `PRAGMA user_version`. Any failed exactness check rolls back every table
  /// rename, JSON rewrite, and version change together.
  static Future<void> migrate(
    Database database,
    int oldVersion,
    int newVersion,
  ) async {
    if (oldVersion < 1) {
      await _createCustomerTable(database, exactIntegers: newVersion >= 4);
      await _createProductTable(database, exactIntegers: newVersion >= 4);
    }
    if (oldVersion < 2 && newVersion >= 2) {
      await _createDraftTables(database, exactIntegers: newVersion >= 4);
    }
    if (oldVersion < 3 && newVersion >= 3) {
      await _createSalesAndSyncTables(database);
    }
    if (oldVersion < 4 && newVersion >= 4) {
      await _migrateV3ToExactIntegers(database);
    }
    if (oldVersion < 5 && newVersion >= 5) {
      await _createMobileIdentityTables(
        database,
        includeOfflineLease: newVersion >= 10,
      );
      await _upgradeSyncResultsV5(database);
    }
    if (oldVersion < 6 && newVersion >= 6) {
      await _upgradeMobileEnrollmentV6(database);
      await _upgradeSyncResultsV5(database);
    }
    if (oldVersion < 7 && newVersion >= 7) {
      await _upgradeProductsV7(database);
    }
    if (oldVersion < 8 && newVersion >= 8) {
      await _createCandidateDeviceTable(database);
    }
    if (oldVersion < 9 && newVersion >= 9) {
      await _upgradeEnrollmentVersionsV9(database);
    }
    if (oldVersion < 10 && newVersion >= 10) {
      await _upgradeOfflineLeaseV10(database);
    }
  }

  static Future<void> _createCustomerTable(
    Database database, {
    required bool exactIntegers,
  }) async {
    final numericType = exactIntegers ? 'INTEGER' : 'REAL';
    await database.execute('''
      CREATE TABLE IF NOT EXISTS cached_customers (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        code TEXT NOT NULL,
        is_general INTEGER NOT NULL CHECK (is_general IN (0, 1)),
        is_active INTEGER NOT NULL CHECK (is_active IN (0, 1)),
        credit_enabled INTEGER NOT NULL CHECK (credit_enabled IN (0, 1)),
        in_attendant_scope INTEGER NOT NULL CHECK (in_attendant_scope IN (0, 1)),
        phone TEXT,
        credit_limit $numericType,
        current_exposure $numericType,
        overdue_amount $numericType,
        due_date_epoch INTEGER,
        master_data_version INTEGER NOT NULL,
        updated_at_epoch INTEGER NOT NULL
      )
    ''');
  }

  static Future<void> _createProductTable(
    Database database, {
    required bool exactIntegers,
  }) async {
    final numericType = exactIntegers ? 'INTEGER' : 'REAL';
    await database.execute('''
      CREATE TABLE IF NOT EXISTS cached_products (
        id TEXT PRIMARY KEY,
        code TEXT NOT NULL,
        name TEXT NOT NULL,
        common_description TEXT NOT NULL,
        brand TEXT NOT NULL,
        category TEXT NOT NULL,
        unit TEXT NOT NULL,
        selling_price $numericType NOT NULL CHECK (selling_price >= 0),
        available_quantity $numericType NOT NULL CHECK (available_quantity >= 0),
        tax_basis_points INTEGER CHECK (tax_basis_points BETWEEN 0 AND 10000),
        master_data_version INTEGER NOT NULL,
        price_version INTEGER NOT NULL,
        updated_at_epoch INTEGER NOT NULL
      )
    ''');
  }

  static Future<void> _createCandidateDeviceTable(Database database) async {
    await database.execute('''
      CREATE TABLE IF NOT EXISTS mobile_device_identity (
        singleton_id INTEGER PRIMARY KEY CHECK (singleton_id = 1),
        candidate_device_id TEXT NOT NULL,
        updated_at_epoch INTEGER NOT NULL
      )
    ''');
  }

  static Future<void> _upgradeProductsV7(Database database) async {
    final columns = await database.rawQuery(
      'PRAGMA table_info(cached_products)',
    );
    if (!columns.any((row) => row['name'] == 'tax_basis_points')) {
      await database.execute(
        'ALTER TABLE cached_products ADD COLUMN tax_basis_points INTEGER '
        'CHECK (tax_basis_points BETWEEN 0 AND 10000)',
      );
    }
  }

  static Future<void> _createDraftTables(
    Database database, {
    required bool exactIntegers,
  }) async {
    await database.execute('''
      CREATE TABLE IF NOT EXISTS sale_drafts (
        client_transaction_id TEXT PRIMARY KEY,
        created_at_epoch INTEGER NOT NULL,
        device_json TEXT NOT NULL,
        sale_type TEXT NOT NULL CHECK (sale_type IN ('cash', 'credit')),
        customer_json TEXT NOT NULL,
        payment_method TEXT NOT NULL,
        updated_at_epoch INTEGER NOT NULL
      )
    ''');
    final numericType = exactIntegers ? 'INTEGER' : 'REAL';
    await database.execute('''
      CREATE TABLE IF NOT EXISTS sale_draft_lines (
        client_transaction_id TEXT NOT NULL,
        line_number INTEGER NOT NULL,
        product_json TEXT NOT NULL,
        unit_price $numericType NOT NULL CHECK (unit_price >= 0),
        quantity $numericType NOT NULL CHECK (quantity > 0),
        PRIMARY KEY (client_transaction_id, line_number),
        FOREIGN KEY (client_transaction_id)
          REFERENCES sale_drafts(client_transaction_id) ON DELETE CASCADE
      )
    ''');
  }

  static Future<void> _createSalesAndSyncTables(Database database) async {
    await database.execute('''
      CREATE TABLE IF NOT EXISTS local_sales (
        client_transaction_id TEXT PRIMARY KEY,
        device_id TEXT NOT NULL,
        created_at_epoch INTEGER NOT NULL,
        sync_status TEXT NOT NULL,
        sale_json TEXT NOT NULL,
        updated_at_epoch INTEGER NOT NULL
      )
    ''');
    await database.execute('''
      CREATE TABLE IF NOT EXISTS sync_queue (
        idempotency_key TEXT PRIMARY KEY,
        device_id TEXT NOT NULL,
        client_transaction_id TEXT NOT NULL,
        user_id TEXT NOT NULL,
        client_timestamp_epoch INTEGER NOT NULL,
        company_id TEXT NOT NULL,
        branch_id TEXT NOT NULL,
        warehouse_id TEXT NOT NULL,
        app_version TEXT NOT NULL,
        master_data_version INTEGER NOT NULL,
        price_version INTEGER NOT NULL,
        sync_attempt_number INTEGER NOT NULL CHECK (sync_attempt_number > 0),
        command_json TEXT NOT NULL,
        queued_at_epoch INTEGER NOT NULL,
        UNIQUE (device_id, client_transaction_id)
      )
    ''');
    await database.execute('''
      CREATE TABLE IF NOT EXISTS sync_results (
        idempotency_key TEXT PRIMARY KEY,
        server_sale_id TEXT NOT NULL,
        receipt_reference TEXT NOT NULL,
        fiscal_status TEXT NOT NULL,
        server_total_minor INTEGER,
        recorded_at_epoch INTEGER NOT NULL
      )
    ''');
    await database.execute('''
      CREATE INDEX IF NOT EXISTS idx_local_sales_created
      ON local_sales(created_at_epoch DESC)
    ''');
    await database.execute('''
      CREATE INDEX IF NOT EXISTS idx_sync_queue_queued
      ON sync_queue(queued_at_epoch ASC)
    ''');
  }

  static Future<void> _createMobileIdentityTables(
    Database database, {
    required bool includeOfflineLease,
  }) async {
    final offlineLeaseColumn =
        includeOfflineLease
            ? 'offline_sales_valid_until_epoch INTEGER NOT NULL,'
            : '';
    await database.execute('''
      CREATE TABLE IF NOT EXISTS mobile_connection (
        singleton_id INTEGER PRIMARY KEY CHECK (singleton_id = 1),
        base_url TEXT NOT NULL,
        identity_mode TEXT NOT NULL
          CHECK (identity_mode IN ('bearer', 'developmentHeaders')),
        dev_actor_id TEXT,
        dev_tenant_id TEXT,
        dev_company_id TEXT,
        dev_branch_id TEXT,
        dev_warehouse_id TEXT,
        updated_at_epoch INTEGER NOT NULL
      )
    ''');
    await database.execute('''
      CREATE TABLE IF NOT EXISTS device_enrollment (
        singleton_id INTEGER PRIMARY KEY CHECK (singleton_id = 1),
        device_id TEXT NOT NULL,
        device_name TEXT NOT NULL,
        status TEXT NOT NULL,
        actor_id TEXT NOT NULL,
        tenant_id TEXT NOT NULL,
        company_id TEXT NOT NULL,
        branch_id TEXT NOT NULL,
        warehouse_id TEXT NOT NULL,
        app_version TEXT NOT NULL,
        master_data_version INTEGER NOT NULL,
        price_version INTEGER NOT NULL,
        available_master_data_version INTEGER NOT NULL,
        available_price_version INTEGER NOT NULL,
        timezone TEXT NOT NULL,
        offline_enabled INTEGER NOT NULL CHECK (offline_enabled IN (0, 1)),
        transaction_value_limit_minor INTEGER NOT NULL,
        daily_value_limit_minor INTEGER NOT NULL,
        remaining_daily_value_minor INTEGER NOT NULL,
        $offlineLeaseColumn
        stock_allocations_json TEXT NOT NULL,
        enrolled_at_epoch INTEGER NOT NULL,
        last_seen_at_epoch INTEGER NOT NULL
      )
    ''');
    await database.execute('''
      CREATE TABLE IF NOT EXISTS device_allocation (
        singleton_id INTEGER PRIMARY KEY CHECK (singleton_id = 1),
        offline_enabled INTEGER NOT NULL CHECK (offline_enabled IN (0, 1)),
        transaction_limit_minor INTEGER NOT NULL,
        daily_value_limit_minor INTEGER NOT NULL,
        remaining_daily_minor INTEGER NOT NULL,
        $offlineLeaseColumn
        product_quantities_json TEXT NOT NULL,
        updated_at_epoch INTEGER NOT NULL
      )
    ''');
  }

  static Future<void> _upgradeSyncResultsV5(Database database) async {
    final columns = await database.rawQuery('PRAGMA table_info(sync_results)');
    final names = columns.map((row) => row['name']).whereType<String>().toSet();
    if (!names.contains('receipt_reference')) {
      await database.execute(
        "ALTER TABLE sync_results ADD COLUMN receipt_reference TEXT NOT NULL DEFAULT ''",
      );
      if (names.contains('receipt_number')) {
        await database.execute(
          "UPDATE sync_results SET receipt_reference = receipt_number WHERE receipt_reference = ''",
        );
      }
    }
    if (!names.contains('fiscal_status')) {
      await database.execute(
        "ALTER TABLE sync_results ADD COLUMN fiscal_status TEXT NOT NULL DEFAULT 'notConfigured'",
      );
    }
    if (!names.contains('server_total_minor')) {
      await database.execute(
        'ALTER TABLE sync_results ADD COLUMN server_total_minor INTEGER',
      );
    }
  }

  static Future<void> _upgradeMobileEnrollmentV6(Database database) async {
    final enrollmentColumns = await database.rawQuery(
      'PRAGMA table_info(device_enrollment)',
    );
    final enrollmentNames =
        enrollmentColumns.map((row) => row['name']).whereType<String>().toSet();
    final enrollmentAdditions = <String, String>{
      'timezone': "TEXT NOT NULL DEFAULT 'Africa/Dar_es_Salaam'",
      'transaction_value_limit_minor': 'INTEGER NOT NULL DEFAULT 0',
      'daily_value_limit_minor': 'INTEGER NOT NULL DEFAULT 0',
      'remaining_daily_value_minor': 'INTEGER NOT NULL DEFAULT 0',
      'stock_allocations_json': "TEXT NOT NULL DEFAULT '[]'",
    };
    for (final entry in enrollmentAdditions.entries) {
      if (!enrollmentNames.contains(entry.key)) {
        await database.execute(
          'ALTER TABLE device_enrollment ADD COLUMN ${entry.key} ${entry.value}',
        );
      }
    }
    final allocationColumns = await database.rawQuery(
      'PRAGMA table_info(device_allocation)',
    );
    final allocationNames =
        allocationColumns.map((row) => row['name']).whereType<String>().toSet();
    if (!allocationNames.contains('daily_value_limit_minor')) {
      await database.execute(
        'ALTER TABLE device_allocation ADD COLUMN daily_value_limit_minor INTEGER NOT NULL DEFAULT 0',
      );
    }
  }

  static Future<void> _upgradeEnrollmentVersionsV9(Database database) async {
    final columns = await database.rawQuery(
      'PRAGMA table_info(device_enrollment)',
    );
    final names = columns.map((row) => row['name']).whereType<String>().toSet();
    if (!names.contains('available_master_data_version')) {
      await database.execute(
        'ALTER TABLE device_enrollment ADD COLUMN '
        'available_master_data_version INTEGER NOT NULL DEFAULT 1',
      );
      await database.execute(
        'UPDATE device_enrollment SET available_master_data_version = '
        'CASE WHEN master_data_version > 0 THEN master_data_version ELSE 1 END',
      );
    }
    if (!names.contains('available_price_version')) {
      await database.execute(
        'ALTER TABLE device_enrollment ADD COLUMN '
        'available_price_version INTEGER NOT NULL DEFAULT 1',
      );
      await database.execute(
        'UPDATE device_enrollment SET available_price_version = '
        'CASE WHEN price_version > 0 THEN price_version ELSE 1 END',
      );
    }
  }

  static Future<void> _upgradeOfflineLeaseV10(Database database) async {
    final enrollmentColumns = await database.rawQuery(
      'PRAGMA table_info(device_enrollment)',
    );
    final enrollmentNames =
        enrollmentColumns.map((row) => row['name']).whereType<String>().toSet();
    if (!enrollmentNames.contains('offline_sales_valid_until_epoch')) {
      await database.execute(
        'ALTER TABLE device_enrollment ADD COLUMN '
        'offline_sales_valid_until_epoch INTEGER NOT NULL DEFAULT 0',
      );
    }
    final allocationColumns = await database.rawQuery(
      'PRAGMA table_info(device_allocation)',
    );
    final allocationNames =
        allocationColumns.map((row) => row['name']).whereType<String>().toSet();
    if (!allocationNames.contains('offline_sales_valid_until_epoch')) {
      await database.execute(
        'ALTER TABLE device_allocation ADD COLUMN '
        'offline_sales_valid_until_epoch INTEGER NOT NULL DEFAULT 0',
      );
    }
  }

  static Future<void> _migrateV3ToExactIntegers(Database database) async {
    await _rewriteLegacyJson(database);
    await _assertIntegralColumns(database, 'cached_customers', [
      'credit_limit',
      'current_exposure',
      'overdue_amount',
    ]);
    await _assertIntegralColumns(database, 'cached_products', [
      'selling_price',
      'available_quantity',
    ]);
    await _assertIntegralColumns(database, 'sale_draft_lines', [
      'unit_price',
      'quantity',
    ]);

    await database.execute(
      'ALTER TABLE cached_customers RENAME TO cached_customers_v3',
    );
    await _createCustomerTable(database, exactIntegers: true);
    await database.execute('''
      INSERT INTO cached_customers (
        id, name, code, is_general, is_active, credit_enabled,
        in_attendant_scope, phone, credit_limit, current_exposure,
        overdue_amount, due_date_epoch, master_data_version, updated_at_epoch
      )
      SELECT
        id, name, code, is_general, is_active, credit_enabled,
        in_attendant_scope, phone, CAST(credit_limit AS INTEGER),
        CAST(current_exposure AS INTEGER), CAST(overdue_amount AS INTEGER),
        due_date_epoch, master_data_version, updated_at_epoch
      FROM cached_customers_v3
    ''');
    await database.execute('DROP TABLE cached_customers_v3');

    await database.execute(
      'ALTER TABLE cached_products RENAME TO cached_products_v3',
    );
    await _createProductTable(database, exactIntegers: true);
    await database.execute('''
      INSERT INTO cached_products (
        id, code, name, common_description, brand, category, unit,
        selling_price, available_quantity, master_data_version,
        price_version, updated_at_epoch
      )
      SELECT
        id, code, name, common_description, brand, category, unit,
        CAST(selling_price AS INTEGER), CAST(available_quantity AS INTEGER),
        master_data_version, price_version, updated_at_epoch
      FROM cached_products_v3
    ''');
    await database.execute('DROP TABLE cached_products_v3');

    await database.execute(
      'ALTER TABLE sale_draft_lines RENAME TO sale_draft_lines_v3',
    );
    await _createDraftTables(database, exactIntegers: true);
    await database.execute('''
      INSERT INTO sale_draft_lines (
        client_transaction_id, line_number, product_json, unit_price, quantity
      )
      SELECT
        client_transaction_id, line_number, product_json,
        CAST(unit_price AS INTEGER), CAST(quantity AS INTEGER)
      FROM sale_draft_lines_v3
    ''');
    await database.execute('DROP TABLE sale_draft_lines_v3');
  }

  static Future<void> _assertIntegralColumns(
    Database database,
    String table,
    List<String> columns,
  ) async {
    for (final column in columns) {
      final rows = await database.rawQuery('''
        SELECT COUNT(*) AS invalid_count
        FROM $table
        WHERE $column IS NOT NULL
          AND $column != CAST($column AS INTEGER)
      ''');
      final invalidCount = rows.single['invalid_count']! as int;
      if (invalidCount != 0) {
        throw StateError('$table.$column contains a fractional legacy value');
      }
    }
  }

  static Future<void> _rewriteLegacyJson(Database database) async {
    final drafts = await database.query(
      'sale_drafts',
      columns: ['client_transaction_id', 'customer_json'],
    );
    for (final row in drafts) {
      final customer = _jsonMap(row['customer_json']! as String);
      _upgradeCustomer(customer);
      await database.update(
        'sale_drafts',
        {'customer_json': jsonEncode(customer)},
        where: 'client_transaction_id = ?',
        whereArgs: [row['client_transaction_id']],
      );
    }

    final lines = await database.query(
      'sale_draft_lines',
      columns: ['client_transaction_id', 'line_number', 'product_json'],
    );
    for (final row in lines) {
      final product = _jsonMap(row['product_json']! as String);
      _upgradeProduct(product);
      await database.update(
        'sale_draft_lines',
        {'product_json': jsonEncode(product)},
        where: 'client_transaction_id = ? AND line_number = ?',
        whereArgs: [row['client_transaction_id'], row['line_number']],
      );
    }

    final sales = await database.query(
      'local_sales',
      columns: ['client_transaction_id', 'sale_json'],
    );
    for (final row in sales) {
      final sale = _jsonMap(row['sale_json']! as String);
      _upgradeSale(sale);
      await database.update(
        'local_sales',
        {'sale_json': jsonEncode(sale)},
        where: 'client_transaction_id = ?',
        whereArgs: [row['client_transaction_id']],
      );
    }

    final commands = await database.query(
      'sync_queue',
      columns: ['idempotency_key', 'command_json'],
    );
    for (final row in commands) {
      final command = _jsonMap(row['command_json']! as String);
      _upgradeSale(_objectMap(command['sale']));
      await database.update(
        'sync_queue',
        {'command_json': jsonEncode(command)},
        where: 'idempotency_key = ?',
        whereArgs: [row['idempotency_key']],
      );
    }
  }

  static void _upgradeCustomer(Map<String, Object?> customer) {
    final creditValue = customer['credit'];
    if (creditValue == null) return;
    final credit = _objectMap(creditValue);
    credit['limit'] = _exactLegacyInt(credit['limit'], 'credit.limit');
    credit['currentExposure'] = _exactLegacyInt(
      credit['currentExposure'],
      'credit.currentExposure',
    );
    credit['overdueAmount'] = _exactLegacyInt(
      credit['overdueAmount'],
      'credit.overdueAmount',
    );
  }

  static void _upgradeProduct(Map<String, Object?> product) {
    product['sellingPrice'] = _exactLegacyInt(
      product['sellingPrice'],
      'product.sellingPrice',
    );
    product['availableQuantity'] = _exactLegacyInt(
      product['availableQuantity'],
      'product.availableQuantity',
    );
  }

  static void _upgradeSale(Map<String, Object?> sale) {
    _upgradeCustomer(_objectMap(sale['customer']));
    final lines = sale['lines']! as List<Object?>;
    for (final lineValue in lines) {
      final line = _objectMap(lineValue);
      _upgradeProduct(_objectMap(line['product']));
      line['unitPrice'] = _exactLegacyInt(line['unitPrice'], 'line.unitPrice');
      line['quantity'] = _exactLegacyInt(line['quantity'], 'line.quantity');
    }
    sale['total'] = _exactLegacyInt(sale['total'], 'sale.total');
  }

  static int _exactLegacyInt(Object? value, String field) {
    if (value is int) return value;
    if (value is double &&
        value.isFinite &&
        value == value.truncateToDouble()) {
      return value.toInt();
    }
    throw StateError('$field contains a fractional legacy value');
  }

  static Map<String, Object?> _jsonMap(String value) =>
      _objectMap(jsonDecode(value));

  static Map<String, Object?> _objectMap(Object? value) =>
      (value! as Map<Object?, Object?>).map(
        (key, item) => MapEntry(key! as String, item),
      );
}
