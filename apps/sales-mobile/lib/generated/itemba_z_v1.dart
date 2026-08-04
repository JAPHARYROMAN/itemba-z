// GENERATED CODE - DO NOT MODIFY BY HAND.
// Source: contracts/openapi/itemba-z.v1.yaml
// Generator: tool/generate_openapi_subset.dart

// ignore_for_file: avoid_dynamic_calls

const String itembaZV1SpecificationSha256 =
    '88f8aa8bfd6519d2741b58419575d180acef29810e661fd0502561cbe829c88d';

typedef GeneratedJson = Map<String, Object?>;

abstract final class ItembaZV1Paths {
  static const customers = '/v1/customers';
  static const products = '/v1/products';
  static const enrollMobileDevice = '/v1/mobile/devices/enroll';
  static const syncMobileSale = '/v1/mobile/sync/sales';
}

enum GeneratedFiscalStatus {
  notConfigured('NOT_CONFIGURED'),
  pending('PENDING'),
  fiscalized('FISCALIZED'),
  failed('FAILED');

  const GeneratedFiscalStatus(this.wireValue);
  final String wireValue;

  static GeneratedFiscalStatus fromJson(Object? value) => values.firstWhere(
    (item) => item.wireValue == _string(value, 'fiscal_status'),
    orElse: () => throw FormatException('Unknown fiscal_status: $value'),
  );
}

class GeneratedScope {
  const GeneratedScope({
    required this.tenantId,
    required this.companyId,
    required this.branchId,
    required this.warehouseId,
  });

  factory GeneratedScope.fromJson(Object? value) {
    final json = _map(value, 'scope');
    return GeneratedScope(
      tenantId: _string(json['tenant_id'], 'scope.tenant_id'),
      companyId: _string(json['company_id'], 'scope.company_id'),
      branchId: _string(json['branch_id'], 'scope.branch_id'),
      warehouseId: _string(json['warehouse_id'], 'scope.warehouse_id'),
    );
  }

  final String tenantId;
  final String companyId;
  final String branchId;
  final String warehouseId;

  GeneratedJson toJson() => {
    'tenant_id': tenantId,
    'company_id': companyId,
    'branch_id': branchId,
    'warehouse_id': warehouseId,
  };
}

class GeneratedMobileStockAllocation {
  const GeneratedMobileStockAllocation({
    required this.productId,
    required this.allocatedQuantity,
    required this.remainingQuantity,
  });

  factory GeneratedMobileStockAllocation.fromJson(Object? value) {
    final json = _map(value, 'stock_allocation');
    return GeneratedMobileStockAllocation(
      productId: _string(json['product_id'], 'product_id'),
      allocatedQuantity: _integer(
        json['allocated_quantity'],
        'allocated_quantity',
      ),
      remainingQuantity: _integer(
        json['remaining_quantity'],
        'remaining_quantity',
      ),
    );
  }

  final String productId;
  final int allocatedQuantity;
  final int remainingQuantity;
}

class GeneratedMobileDeviceEnrollmentCommand {
  const GeneratedMobileDeviceEnrollmentCommand({
    required this.deviceId,
    required this.deviceName,
    required this.appVersion,
    this.installedMasterDataVersion,
    this.installedPriceVersion,
  });

  final String deviceId;
  final String deviceName;
  final String appVersion;
  final int? installedMasterDataVersion;
  final int? installedPriceVersion;

  GeneratedJson toJson() => {
    'device_id': deviceId,
    'device_name': deviceName,
    'app_version': appVersion,
    if (installedMasterDataVersion != null)
      'installed_master_data_version': installedMasterDataVersion,
    if (installedPriceVersion != null)
      'installed_price_version': installedPriceVersion,
  };
}

class GeneratedMobileDeviceEnrollment {
  const GeneratedMobileDeviceEnrollment({
    required this.deviceId,
    required this.status,
    required this.actorId,
    required this.scope,
    required this.deviceName,
    required this.appVersion,
    required this.masterDataVersion,
    required this.priceVersion,
    required this.availableMasterDataVersion,
    required this.availablePriceVersion,
    required this.timezone,
    required this.offlineEnabled,
    required this.transactionValueLimitMinor,
    required this.dailyValueLimitMinor,
    required this.remainingDailyValueMinor,
    required this.offlineSalesValidUntil,
    required this.enrolledAt,
    required this.lastSeenAt,
    required this.stockAllocations,
  });

  factory GeneratedMobileDeviceEnrollment.fromJson(Object? value) {
    final json = _map(value, 'MobileDeviceEnrollment');
    return GeneratedMobileDeviceEnrollment(
      deviceId: _string(json['device_id'], 'device_id'),
      status: _string(json['status'], 'status'),
      actorId: _string(json['actor_id'], 'actor_id'),
      scope: GeneratedScope.fromJson(json['scope']),
      deviceName: _string(json['device_name'], 'device_name'),
      appVersion: _string(json['app_version'], 'app_version'),
      masterDataVersion: _integer(
        json['master_data_version'],
        'master_data_version',
      ),
      priceVersion: _integer(json['price_version'], 'price_version'),
      availableMasterDataVersion: _integer(
        json['available_master_data_version'],
        'available_master_data_version',
      ),
      availablePriceVersion: _integer(
        json['available_price_version'],
        'available_price_version',
      ),
      timezone: _string(json['timezone'], 'timezone'),
      offlineEnabled: _boolean(json['offline_enabled'], 'offline_enabled'),
      transactionValueLimitMinor: _integer(
        json['transaction_value_limit_minor'],
        'transaction_value_limit_minor',
      ),
      dailyValueLimitMinor: _integer(
        json['daily_value_limit_minor'],
        'daily_value_limit_minor',
      ),
      remainingDailyValueMinor: _integer(
        json['remaining_daily_value_minor'],
        'remaining_daily_value_minor',
      ),
      offlineSalesValidUntil: _string(
        json['offline_sales_valid_until'],
        'offline_sales_valid_until',
      ),
      enrolledAt: _string(json['enrolled_at'], 'enrolled_at'),
      lastSeenAt: _string(json['last_seen_at'], 'last_seen_at'),
      stockAllocations: _list(
        json['stock_allocations'],
        'stock_allocations',
      ).map(GeneratedMobileStockAllocation.fromJson).toList(growable: false),
    );
  }

  final String deviceId;
  final String status;
  final String actorId;
  final GeneratedScope scope;
  final String deviceName;
  final String appVersion;
  final int masterDataVersion;
  final int priceVersion;
  final int availableMasterDataVersion;
  final int availablePriceVersion;
  final String timezone;
  final bool offlineEnabled;
  final int transactionValueLimitMinor;
  final int dailyValueLimitMinor;
  final int remainingDailyValueMinor;
  final String offlineSalesValidUntil;
  final String enrolledAt;
  final String lastSeenAt;
  final List<GeneratedMobileStockAllocation> stockAllocations;
}

class GeneratedCustomerSummary {
  const GeneratedCustomerSummary({
    required this.id,
    required this.code,
    required this.name,
    required this.status,
    required this.isGeneralCustomer,
    required this.creditEnabled,
    required this.creditLimitMinor,
    required this.currentExposureMinor,
    required this.availableCreditMinor,
  });

  factory GeneratedCustomerSummary.fromJson(Object? value) {
    final json = _map(value, 'CustomerSummary');
    return GeneratedCustomerSummary(
      id: _string(json['id'], 'id'),
      code: _string(json['code'], 'code'),
      name: _string(json['name'], 'name'),
      status: _string(json['status'], 'status'),
      isGeneralCustomer: _boolean(
        json['is_general_customer'],
        'is_general_customer',
      ),
      creditEnabled: _boolean(json['credit_enabled'], 'credit_enabled'),
      creditLimitMinor: _integer(
        json['credit_limit_minor'],
        'credit_limit_minor',
      ),
      currentExposureMinor: _integer(
        json['current_exposure_minor'],
        'current_exposure_minor',
      ),
      availableCreditMinor: _integer(
        json['available_credit_minor'],
        'available_credit_minor',
      ),
    );
  }

  final String id;
  final String code;
  final String name;
  final String status;
  final bool isGeneralCustomer;
  final bool creditEnabled;
  final int creditLimitMinor;
  final int currentExposureMinor;
  final int availableCreditMinor;
}

class GeneratedCustomerPage {
  const GeneratedCustomerPage({required this.items, this.nextCursor});

  factory GeneratedCustomerPage.fromJson(Object? value) {
    final json = _map(value, 'CustomerPage');
    return GeneratedCustomerPage(
      items: _list(
        json['items'],
        'items',
      ).map(GeneratedCustomerSummary.fromJson).toList(growable: false),
      nextCursor: json['next_cursor'] as String?,
    );
  }

  final List<GeneratedCustomerSummary> items;
  final String? nextCursor;
}

class GeneratedProductSummary {
  const GeneratedProductSummary({
    required this.id,
    required this.code,
    required this.name,
    required this.unit,
    required this.currency,
    required this.unitPriceMinor,
    required this.availableQuantity,
    required this.taxBasisPoints,
    required this.priceVersion,
    required this.masterDataVersion,
  });

  factory GeneratedProductSummary.fromJson(Object? value) {
    final json = _map(value, 'ProductSummary');
    return GeneratedProductSummary(
      id: _string(json['id'], 'id'),
      code: _string(json['code'], 'code'),
      name: _string(json['name'], 'name'),
      unit: _string(json['unit'], 'unit'),
      currency: _string(json['currency'], 'currency'),
      unitPriceMinor: _integer(json['unit_price_minor'], 'unit_price_minor'),
      availableQuantity: _integer(
        json['available_quantity'],
        'available_quantity',
      ),
      taxBasisPoints: _integer(json['tax_basis_points'], 'tax_basis_points'),
      priceVersion: _integer(json['price_version'], 'price_version'),
      masterDataVersion: _integer(
        json['master_data_version'],
        'master_data_version',
      ),
    );
  }

  final String id;
  final String code;
  final String name;
  final String unit;
  final String currency;
  final int unitPriceMinor;
  final int availableQuantity;
  final int taxBasisPoints;
  final int priceVersion;
  final int masterDataVersion;
}

class GeneratedProductPage {
  const GeneratedProductPage({required this.items, this.nextCursor});

  factory GeneratedProductPage.fromJson(Object? value) {
    final json = _map(value, 'ProductPage');
    return GeneratedProductPage(
      items: _list(
        json['items'],
        'items',
      ).map(GeneratedProductSummary.fromJson).toList(growable: false),
      nextCursor: json['next_cursor'] as String?,
    );
  }

  final List<GeneratedProductSummary> items;
  final String? nextCursor;
}

class GeneratedSaleLineCommand {
  const GeneratedSaleLineCommand({
    required this.productId,
    required this.quantity,
  });
  final String productId;
  final int quantity;
  GeneratedJson toJson() => {'product_id': productId, 'quantity': quantity};
}

class GeneratedMobileSaleSyncCommand {
  const GeneratedMobileSaleSyncCommand({
    required this.customerId,
    required this.kind,
    required this.paymentMethod,
    required this.lines,
    required this.deviceId,
    required this.clientTransactionId,
    required this.clientTimestamp,
    required this.appVersion,
    required this.masterDataVersion,
    required this.priceVersion,
    required this.syncAttempt,
    required this.offline,
  });

  final String customerId;
  final String kind;
  final String paymentMethod;
  final List<GeneratedSaleLineCommand> lines;
  final String deviceId;
  final String clientTransactionId;
  final String clientTimestamp;
  final String appVersion;
  final int masterDataVersion;
  final int priceVersion;
  final int syncAttempt;
  final bool offline;

  GeneratedJson toJson() => {
    'customer_id': customerId,
    'kind': kind,
    'payment_method': paymentMethod,
    'lines': lines.map((line) => line.toJson()).toList(growable: false),
    'device_id': deviceId,
    'client_transaction_id': clientTransactionId,
    'client_timestamp': clientTimestamp,
    'app_version': appVersion,
    'master_data_version': masterDataVersion,
    'price_version': priceVersion,
    'sync_attempt': syncAttempt,
    'offline': offline,
  };
}

class GeneratedSaleResult {
  const GeneratedSaleResult({
    required this.id,
    required this.totalMinor,
    required this.receiptReference,
    required this.fiscalStatus,
  });

  factory GeneratedSaleResult.fromJson(Object? value) {
    final json = _map(value, 'Sale');
    return GeneratedSaleResult(
      id: _string(json['id'], 'sale.id'),
      totalMinor: _integer(json['total_minor'], 'sale.total_minor'),
      receiptReference: _string(
        json['receipt_reference'],
        'sale.receipt_reference',
      ),
      fiscalStatus: GeneratedFiscalStatus.fromJson(json['fiscal_status']),
    );
  }

  final String id;
  final int totalMinor;
  final String receiptReference;
  final GeneratedFiscalStatus fiscalStatus;
}

class GeneratedMobileSyncResult {
  const GeneratedMobileSyncResult({
    required this.clientTransactionId,
    required this.state,
    required this.receiptReference,
    required this.fiscalStatus,
    required this.idempotentReplay,
    required this.sale,
  });

  factory GeneratedMobileSyncResult.fromJson(Object? value) {
    final json = _map(value, 'MobileSyncResult');
    return GeneratedMobileSyncResult(
      clientTransactionId: _string(
        json['client_transaction_id'],
        'client_transaction_id',
      ),
      state: _string(json['state'], 'state'),
      receiptReference: _string(json['receipt_reference'], 'receipt_reference'),
      fiscalStatus: GeneratedFiscalStatus.fromJson(json['fiscal_status']),
      idempotentReplay: _boolean(
        json['idempotent_replay'],
        'idempotent_replay',
      ),
      sale: GeneratedSaleResult.fromJson(json['sale']),
    );
  }

  final String clientTransactionId;
  final String state;
  final String receiptReference;
  final GeneratedFiscalStatus fiscalStatus;
  final bool idempotentReplay;
  final GeneratedSaleResult sale;
}

class GeneratedProblem {
  const GeneratedProblem({
    required this.title,
    required this.status,
    required this.correlationId,
    this.detail,
    this.code,
  });

  factory GeneratedProblem.fromJson(Object? value) {
    final json = _map(value, 'Problem');
    return GeneratedProblem(
      title: _string(json['title'], 'title'),
      status: _integer(json['status'], 'status'),
      correlationId: _string(json['correlation_id'], 'correlation_id'),
      detail: json['detail'] as String?,
      code: json['code'] as String?,
    );
  }

  final String title;
  final int status;
  final String correlationId;
  final String? detail;
  final String? code;
}

typedef GeneratedJsonExecutor =
    Future<GeneratedJson> Function({
      required String method,
      required String path,
      Map<String, String> query,
      Map<String, String> headers,
      GeneratedJson? body,
    });

class ItembaZV1GeneratedClient {
  const ItembaZV1GeneratedClient(this.execute);
  final GeneratedJsonExecutor execute;

  Future<GeneratedMobileDeviceEnrollment> enrollMobileDevice(
    GeneratedMobileDeviceEnrollmentCommand command,
  ) async => GeneratedMobileDeviceEnrollment.fromJson(
    await execute(
      method: 'POST',
      path: ItembaZV1Paths.enrollMobileDevice,
      body: command.toJson(),
    ),
  );

  Future<GeneratedCustomerPage> listCustomers({String? cursor}) async =>
      GeneratedCustomerPage.fromJson(
        await execute(
          method: 'GET',
          path: ItembaZV1Paths.customers,
          query: {'page_size': '200', if (cursor != null) 'cursor': cursor},
        ),
      );

  Future<GeneratedProductPage> listProducts({String? cursor}) async =>
      GeneratedProductPage.fromJson(
        await execute(
          method: 'GET',
          path: ItembaZV1Paths.products,
          query: {'page_size': '200', if (cursor != null) 'cursor': cursor},
        ),
      );

  Future<GeneratedMobileSyncResult> syncMobileSale(
    GeneratedMobileSaleSyncCommand command,
  ) async => GeneratedMobileSyncResult.fromJson(
    await execute(
      method: 'POST',
      path: ItembaZV1Paths.syncMobileSale,
      body: command.toJson(),
    ),
  );
}

GeneratedJson _map(Object? value, String field) {
  if (value is Map<String, Object?>) return value;
  if (value is Map<Object?, Object?>) {
    return value.map((key, item) => MapEntry(key as String, item));
  }
  throw FormatException('$field must be an object.');
}

List<Object?> _list(Object? value, String field) {
  if (value is List<Object?>) return value;
  throw FormatException('$field must be an array.');
}

String _string(Object? value, String field) {
  if (value is String && value.isNotEmpty) return value;
  throw FormatException('$field must be a non-empty string.');
}

int _integer(Object? value, String field) {
  if (value is int && value.abs() <= 9007199254740991) return value;
  throw FormatException('$field must be a JavaScript-safe integer.');
}

bool _boolean(Object? value, String field) {
  if (value is bool) return value;
  throw FormatException('$field must be a boolean.');
}
