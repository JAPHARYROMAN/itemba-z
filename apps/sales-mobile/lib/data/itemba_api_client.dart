import 'dart:convert';

import 'package:flutter/foundation.dart';

import '../core/uuid.dart';
import '../domain/connection_models.dart';
import '../domain/models.dart';
import '../generated/itemba_z_v1.dart';
import 'http_api_transport.dart';
import 'mobile_credential_store.dart';

const bool allowDevelopmentIdentity =
    !kReleaseMode &&
    bool.fromEnvironment('ITEMBA_Z_ALLOW_DEV_IDENTITY', defaultValue: false);
const int maximumSafeApiInteger = 9007199254740991;

enum ApiFailureKind {
  retryable,
  terminal,
  reconciliation,
  authentication,
  suspended,
  invalidResponse,
}

class ApiException implements Exception {
  const ApiException({
    required this.kind,
    required this.message,
    this.statusCode,
    this.code,
    this.correlationId,
  });

  final ApiFailureKind kind;
  final String message;
  final int? statusCode;
  final String? code;
  final String? correlationId;

  bool get isRetryable => kind == ApiFailureKind.retryable;

  @override
  String toString() => message;
}

class MasterDataSnapshot {
  const MasterDataSnapshot({
    required this.customers,
    required this.products,
    required this.masterDataVersion,
    required this.priceVersion,
    required this.catalogSnapshotToken,
  });

  final List<Customer> customers;
  final List<Product> products;
  final int masterDataVersion;
  final int priceVersion;
  final String catalogSnapshotToken;
}

class _CatalogPages {
  const _CatalogPages({
    required this.items,
    required this.catalogSnapshotToken,
    required this.masterDataVersion,
    required this.priceVersion,
  });

  final List<Map<String, Object?>> items;
  final String catalogSnapshotToken;
  final int masterDataVersion;
  final int priceVersion;
}

class ItembaApiClient {
  ItembaApiClient({
    required this.connection,
    required this.credentials,
    required this.transport,
    UuidGenerator? uuid,
    bool? developmentIdentityAllowed,
  }) : _uuid = uuid ?? UuidGenerator(),
       _developmentIdentityAllowed =
           !kReleaseMode &&
           (developmentIdentityAllowed ?? allowDevelopmentIdentity) {
    connection.validate(allowDevelopmentIdentity: _developmentIdentityAllowed);
  }

  final MobileConnection connection;
  final MobileCredentialStore credentials;
  final ApiTransport transport;
  final UuidGenerator _uuid;
  final bool _developmentIdentityAllowed;

  Future<DeviceEnrollment> enrollDevice({
    required String deviceId,
    required String deviceName,
    required String appVersion,
    int? installedMasterDataVersion,
    int? installedPriceVersion,
    String? installedCatalogSnapshotToken,
  }) async {
    _requireUuid(deviceId, 'device_id');
    if ((installedMasterDataVersion == null) !=
            (installedPriceVersion == null) ||
        (installedMasterDataVersion == null) !=
            (installedCatalogSnapshotToken == null) ||
        (installedMasterDataVersion != null &&
            (installedMasterDataVersion < 1 || installedPriceVersion! < 1))) {
      throw const FormatException(
        'Installed catalog token and versions must be valid and supplied together.',
      );
    }
    if (installedCatalogSnapshotToken != null) {
      _requireCatalogToken(
        installedCatalogSnapshotToken,
        'installed_catalog_snapshot_token',
      );
    }
    final response = await _sendJson(
      method: 'POST',
      path: ItembaZV1Paths.enrollMobileDevice,
      body:
          GeneratedMobileDeviceEnrollmentCommand(
            deviceId: deviceId,
            deviceName: deviceName.trim(),
            appVersion: appVersion,
            installedMasterDataVersion: installedMasterDataVersion,
            installedPriceVersion: installedPriceVersion,
            installedCatalogSnapshotToken: installedCatalogSnapshotToken,
          ).toJson(),
    );
    final scope = _map(response['scope'], 'scope');
    final allocationValues = response['stock_allocations'];
    if (allocationValues is! List<Object?>) {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'stock_allocations must be an array.',
      );
    }
    final stockAllocations = <MobileStockAllocation>[];
    final allocatedProductIds = <String>{};
    for (final value in allocationValues) {
      final allocation = _map(value, 'stock_allocations[]');
      final productId = _uuidValue(
        allocation['product_id'],
        'stock_allocations[].product_id',
      );
      final allocated = _nonNegativeInt(
        allocation['allocated_quantity'],
        'stock_allocations[].allocated_quantity',
      );
      final remaining = _nonNegativeInt(
        allocation['remaining_quantity'],
        'stock_allocations[].remaining_quantity',
      );
      if (!allocatedProductIds.add(productId) || remaining > allocated) {
        throw const ApiException(
          kind: ApiFailureKind.invalidResponse,
          message: 'The server returned an invalid stock allocation.',
        );
      }
      stockAllocations.add(
        MobileStockAllocation(
          productId: productId,
          allocatedQuantity: allocated,
          remainingQuantity: remaining,
        ),
      );
    }
    final dailyLimit = _nonNegativeInt(
      response['daily_value_limit_minor'],
      'daily_value_limit_minor',
    );
    final remainingDaily = _nonNegativeInt(
      response['remaining_daily_value_minor'],
      'remaining_daily_value_minor',
    );
    if (remainingDaily > dailyLimit) {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'The remaining daily value exceeds the daily limit.',
      );
    }
    final installedMasterDataVersionValue = _nonNegativeInt(
      response['master_data_version'],
      'master_data_version',
    );
    final installedPriceVersionValue = _nonNegativeInt(
      response['price_version'],
      'price_version',
    );
    final availableMasterDataVersion = _positiveInt(
      response['available_master_data_version'],
      'available_master_data_version',
    );
    final availablePriceVersion = _positiveInt(
      response['available_price_version'],
      'available_price_version',
    );
    if (installedMasterDataVersionValue > availableMasterDataVersion ||
        installedPriceVersionValue > availablePriceVersion) {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'Installed device versions exceed available versions.',
      );
    }
    final enrolledAt = _date(response['enrolled_at'], 'enrolled_at');
    final lastSeenAt = _date(response['last_seen_at'], 'last_seen_at');
    final offlineSalesValidUntil = _date(
      response['offline_sales_valid_until'],
      'offline_sales_valid_until',
    );
    if (offlineSalesValidUntil.isAfter(
      lastSeenAt.add(const Duration(hours: 4)),
    )) {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'The server returned an excessive offline sales lease.',
      );
    }
    return DeviceEnrollment(
      deviceId: _uuidValue(response['device_id'], 'device_id'),
      deviceName: _string(response['device_name'], 'device_name'),
      status: _string(response['status'], 'status'),
      actorId: _uuidValue(response['actor_id'], 'actor_id'),
      scope: EnrollmentScope(
        tenantId: _uuidValue(scope['tenant_id'], 'scope.tenant_id'),
        companyId: _uuidValue(scope['company_id'], 'scope.company_id'),
        branchId: _uuidValue(scope['branch_id'], 'scope.branch_id'),
        warehouseId: _uuidValue(scope['warehouse_id'], 'scope.warehouse_id'),
      ),
      appVersion: _string(response['app_version'], 'app_version'),
      catalogSnapshotToken: _catalogToken(
        response['catalog_snapshot_token'],
        'catalog_snapshot_token',
        allowUnacknowledged: true,
      ),
      masterDataVersion: installedMasterDataVersionValue,
      priceVersion: installedPriceVersionValue,
      availableCatalogSnapshotToken: _catalogToken(
        response['available_catalog_snapshot_token'],
        'available_catalog_snapshot_token',
      ),
      availableMasterDataVersion: availableMasterDataVersion,
      availablePriceVersion: availablePriceVersion,
      timezone: _string(response['timezone'], 'timezone'),
      offlineEnabled: _bool(response['offline_enabled'], 'offline_enabled'),
      transactionValueLimitMinor: _nonNegativeInt(
        response['transaction_value_limit_minor'],
        'transaction_value_limit_minor',
      ),
      dailyValueLimitMinor: dailyLimit,
      remainingDailyValueMinor: remainingDaily,
      offlineSalesValidUntil: offlineSalesValidUntil,
      stockAllocations: List.unmodifiable(stockAllocations),
      enrolledAt: enrolledAt,
      lastSeenAt: lastSeenAt,
    );
  }

  Future<MasterDataSnapshot> refreshMasterData({
    required String catalogSnapshotToken,
  }) async {
    _requireCatalogToken(catalogSnapshotToken, 'catalog_snapshot_token');
    final customerPages = await _allPages(
      ItembaZV1Paths.customers,
      catalogSnapshotToken: catalogSnapshotToken,
    );
    final productPages = await _allPages(
      ItembaZV1Paths.products,
      catalogSnapshotToken: catalogSnapshotToken,
    );
    if (customerPages.catalogSnapshotToken !=
            productPages.catalogSnapshotToken ||
        customerPages.masterDataVersion != productPages.masterDataVersion ||
        customerPages.priceVersion != productPages.priceVersion) {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'Customer and product catalog snapshots do not match.',
      );
    }
    final products = productPages.items.map(_product).toList(growable: false);
    if (products.isEmpty) {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'The server returned no products.',
      );
    }
    final masterDataVersion = productPages.masterDataVersion;
    final priceVersion = productPages.priceVersion;
    if (products.any(
      (product) =>
          product.masterDataVersion != masterDataVersion ||
          product.priceVersion != priceVersion,
    )) {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'The product cache contains inconsistent versions.',
      );
    }
    return MasterDataSnapshot(
      customers: customerPages.items.map(_customer).toList(growable: false),
      products: products,
      masterDataVersion: masterDataVersion,
      priceVersion: priceVersion,
      catalogSnapshotToken: productPages.catalogSnapshotToken,
    );
  }

  Future<SyncResult> syncSale(
    SyncCommand command, {
    required int attempt,
  }) async {
    final wireClientId = UuidGenerator.normalizeLegacyTransactionId(
      command.clientTransactionId,
    );
    final body = await _sendJson(
      method: 'POST',
      path: ItembaZV1Paths.syncMobileSale,
      body:
          GeneratedMobileSaleSyncCommand(
            customerId: command.sale.customer.id,
            kind: command.sale.saleType == SaleType.cash ? 'CASH' : 'CREDIT',
            paymentMethod: _paymentMethod(command.sale.paymentMethod),
            lines: [
              for (final line in command.sale.lines)
                GeneratedSaleLineCommand(
                  productId: line.product.id,
                  quantity: line.quantity,
                ),
            ],
            deviceId: command.deviceId,
            clientTransactionId: wireClientId,
            clientTimestamp: command.clientTimestamp.toUtc().toIso8601String(),
            appVersion: command.appVersion,
            masterDataVersion: command.masterDataVersion,
            priceVersion: command.priceVersion,
            catalogSnapshotToken: command.catalogSnapshotToken,
            syncAttempt: attempt,
            offline: command.sale.createdOffline,
          ).toJson(),
    );
    final state = _string(body['state'], 'state');
    if (state != 'synced') {
      throw ApiException(
        kind: ApiFailureKind.terminal,
        message:
            _optionalString(body['rejection_message']) ??
            'The server did not accept this sale.',
        code: _optionalString(body['rejection_code']),
      );
    }
    final sale = _map(body['sale'], 'sale');
    final serverSaleId = _uuidValue(sale['id'], 'sale.id');
    final responseClientId = _uuidValue(
      body['client_transaction_id'],
      'client_transaction_id',
    );
    if (responseClientId != wireClientId) {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'The server returned a different client transaction ID.',
      );
    }
    final reference = _string(body['receipt_reference'], 'receipt_reference');
    final statusValue = _string(body['fiscal_status'], 'fiscal_status');
    final saleReference = _string(
      sale['receipt_reference'],
      'sale.receipt_reference',
    );
    final saleFiscalStatus = _string(
      sale['fiscal_status'],
      'sale.fiscal_status',
    );
    if (saleReference != reference || saleFiscalStatus != statusValue) {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'The sale receipt or fiscal status does not reconcile.',
      );
    }
    if (_uuidValue(sale['customer_id'], 'sale.customer_id') !=
            command.sale.customer.id ||
        _string(sale['currency'], 'sale.currency') != 'TZS') {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'The authoritative sale identity does not match the command.',
      );
    }
    final authoritativeLines = _authoritativeLines(sale, command);
    final subtotalMinor = _exactInt(
      sale['subtotal_minor'],
      'sale.subtotal_minor',
    );
    final taxMinor = _exactInt(sale['tax_minor'], 'sale.tax_minor');
    final totalMinor = _exactInt(sale['total_minor'], 'sale.total_minor');
    if (authoritativeLines.fold<int>(
              0,
              (sum, line) => sum + line.subtotalMinor,
            ) !=
            subtotalMinor ||
        authoritativeLines.fold<int>(0, (sum, line) => sum + line.taxMinor) !=
            taxMinor ||
        authoritativeLines.fold<int>(0, (sum, line) => sum + line.totalMinor) !=
            totalMinor ||
        subtotalMinor + taxMinor != totalMinor) {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'The authoritative sale totals do not reconcile.',
      );
    }
    return SyncResult(
      serverSaleId: serverSaleId,
      receiptReference: reference,
      fiscalStatus: _fiscalStatus(statusValue),
      serverTotalMinor: totalMinor,
      serverSubtotalMinor: subtotalMinor,
      serverTaxMinor: taxMinor,
      serverCogsMinor: null,
      serverLines: authoritativeLines,
      wasDuplicate: _bool(body['idempotent_replay'], 'idempotent_replay'),
    );
  }

  List<AuthoritativeSaleLine> _authoritativeLines(
    Map<String, Object?> sale,
    SyncCommand command,
  ) {
    final values = sale['lines'];
    if (values is! List<Object?>) {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'sale.lines must be an array.',
      );
    }
    final expected = {
      for (final line in command.sale.lines) line.product.id: line,
    };
    final result = <AuthoritativeSaleLine>[];
    final seen = <String>{};
    for (final value in values) {
      final line = _map(value, 'sale.lines[]');
      final productId = _uuidValue(
        line['product_id'],
        'sale.lines[].product_id',
      );
      final expectedLine = expected[productId];
      final quantity = _positiveInt(line['quantity'], 'sale.lines[].quantity');
      if (expectedLine == null ||
          expectedLine.quantity != quantity ||
          !seen.add(productId)) {
        throw const ApiException(
          kind: ApiFailureKind.invalidResponse,
          message: 'The authoritative sale lines do not match the command.',
        );
      }
      final unitPrice = _nonNegativeInt(
        line['unit_price_minor'],
        'sale.lines[].unit_price_minor',
      );
      final subtotal = _nonNegativeInt(
        line['subtotal_minor'],
        'sale.lines[].subtotal_minor',
      );
      final tax = _nonNegativeInt(line['tax_minor'], 'sale.lines[].tax_minor');
      final total = _nonNegativeInt(
        line['total_minor'],
        'sale.lines[].total_minor',
      );
      if (unitPrice * quantity != subtotal || subtotal + tax != total) {
        throw const ApiException(
          kind: ApiFailureKind.invalidResponse,
          message: 'An authoritative sale line does not reconcile.',
        );
      }
      result.add(
        AuthoritativeSaleLine(
          productId: productId,
          quantity: quantity,
          unitPriceMinor: unitPrice,
          subtotalMinor: subtotal,
          taxMinor: tax,
          totalMinor: total,
        ),
      );
    }
    if (seen.length != expected.length) {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'The server omitted an authoritative sale line.',
      );
    }
    return List.unmodifiable(result);
  }

  Future<_CatalogPages> _allPages(
    String path, {
    required String catalogSnapshotToken,
  }) async {
    final items = <Map<String, Object?>>[];
    final seenCursors = <String>{};
    int? masterDataVersion;
    int? priceVersion;
    String? cursor;
    for (var page = 0; page < 1000; page += 1) {
      final response = await _sendJson(
        method: 'GET',
        path: path,
        query: {
          'page_size': '200',
          'snapshot_token': catalogSnapshotToken,
          if (cursor != null) 'cursor': cursor,
        },
      );
      final responseToken = _catalogToken(
        response['catalog_snapshot_token'],
        'catalog_snapshot_token',
      );
      final responseMasterDataVersion = _positiveInt(
        response['master_data_version'],
        'master_data_version',
      );
      final responsePriceVersion = _positiveInt(
        response['price_version'],
        'price_version',
      );
      if (responseToken != catalogSnapshotToken ||
          (masterDataVersion != null &&
              masterDataVersion != responseMasterDataVersion) ||
          (priceVersion != null && priceVersion != responsePriceVersion)) {
        throw const ApiException(
          kind: ApiFailureKind.invalidResponse,
          message: 'The catalog snapshot changed during download.',
        );
      }
      masterDataVersion = responseMasterDataVersion;
      priceVersion = responsePriceVersion;
      final pageItems = response['items'];
      if (pageItems is! List<Object?>) {
        throw const ApiException(
          kind: ApiFailureKind.invalidResponse,
          message: 'A master-data page is invalid.',
        );
      }
      items.addAll(pageItems.map((item) => _map(item, 'items[]')));
      final nextCursor = response['next_cursor'];
      if (nextCursor != null && nextCursor is! String) {
        throw const ApiException(
          kind: ApiFailureKind.invalidResponse,
          message: 'next_cursor must be a string or null.',
        );
      }
      cursor = nextCursor as String?;
      if (cursor == null || cursor.isEmpty) {
        return _CatalogPages(
          items: List.unmodifiable(items),
          catalogSnapshotToken: responseToken,
          masterDataVersion: masterDataVersion,
          priceVersion: priceVersion,
        );
      }
      if (!seenCursors.add(cursor)) {
        throw const ApiException(
          kind: ApiFailureKind.invalidResponse,
          message: 'The server repeated a pagination cursor.',
        );
      }
    }
    throw const ApiException(
      kind: ApiFailureKind.invalidResponse,
      message: 'The master-data result exceeded the page safety limit.',
    );
  }

  Future<Map<String, Object?>> _sendJson({
    required String method,
    required String path,
    Map<String, String> query = const {},
    Map<String, String> headers = const {},
    Map<String, Object?>? body,
  }) async {
    final authentication = await _authenticationHeaders();
    final uri = connection.baseUrl
        .resolve(path)
        .replace(queryParameters: query.isEmpty ? null : query);
    ApiResponse response;
    try {
      response = await transport.send(
        ApiRequest(
          method: method,
          uri: uri,
          headers: {
            'Accept': 'application/json',
            if (body != null) 'Content-Type': 'application/json',
            'X-Correlation-ID': _uuid.v4(),
            ...authentication,
            ...headers,
          },
          body: body == null ? null : jsonEncode(body),
        ),
      );
    } on ApiTransportException {
      throw const ApiException(
        kind: ApiFailureKind.retryable,
        message: 'The API could not be reached.',
      );
    }
    if (response.statusCode < 200 || response.statusCode >= 300) {
      throw _httpFailure(response);
    }
    if (response.body.trim().isEmpty) return const {};
    try {
      return _map(jsonDecode(response.body), 'response');
    } on ApiException {
      rethrow;
    } catch (_) {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'The API returned invalid JSON.',
      );
    }
  }

  Future<Map<String, String>> _authenticationHeaders() async {
    switch (connection.identityMode) {
      case MobileIdentityMode.bearer:
        final token = (await credentials.readBearerToken())?.trim();
        if (token == null || token.isEmpty) {
          throw const ApiException(
            kind: ApiFailureKind.authentication,
            message: 'A bearer credential is required.',
          );
        }
        return {'Authorization': 'Bearer $token'};
      case MobileIdentityMode.developmentHeaders:
        if (!_developmentIdentityAllowed) {
          throw const ApiException(
            kind: ApiFailureKind.authentication,
            message: 'Development identity is disabled.',
          );
        }
        return connection.developmentIdentity!.headers;
    }
  }

  ApiException _httpFailure(ApiResponse response) {
    Map<String, Object?> problem = const {};
    try {
      if (response.body.trim().isNotEmpty) {
        problem = _map(jsonDecode(response.body), 'problem');
      }
    } catch (_) {
      problem = const {};
    }
    final status = response.statusCode;
    final problemStatus = problem['status'];
    final problemCode = _safeNonEmptyString(problem['code']);
    final problemCorrelation = _safeNonEmptyString(problem['correlation_id']);
    final authoritativeRejection =
        status >= 400 &&
        status < 500 &&
        problemStatus is int &&
        problemStatus == status &&
        _safeNonEmptyString(problem['type']) != null &&
        _safeNonEmptyString(problem['title']) != null &&
        problemCode != null &&
        problemCorrelation != null &&
        UuidGenerator.isValid(problemCorrelation);
    final kind = switch ((status, problemCode)) {
      (409, 'offline_reconciliation_required') => ApiFailureKind.reconciliation,
      (401, _) => ApiFailureKind.authentication,
      (403, _) => ApiFailureKind.suspended,
      (408 || 429, _) => ApiFailureKind.retryable,
      (>= 500, _) => ApiFailureKind.retryable,
      _ when authoritativeRejection => ApiFailureKind.terminal,
      _ => ApiFailureKind.invalidResponse,
    };
    return ApiException(
      kind: kind,
      statusCode: status,
      code: problemCode,
      correlationId: problemCorrelation,
      message:
          _safeNonEmptyString(problem['detail']) ??
          _safeNonEmptyString(problem['title']) ??
          (kind == ApiFailureKind.invalidResponse
              ? 'The API returned an invalid rejection response ($status).'
              : 'The API rejected the request ($status).'),
    );
  }

  static String? _safeNonEmptyString(Object? value) {
    if (value is! String) return null;
    final trimmed = value.trim();
    return trimmed.isEmpty ? null : trimmed;
  }

  Customer _customer(Map<String, Object?> map) {
    final limit = _nonNegativeInt(
      map['credit_limit_minor'],
      'credit_limit_minor',
    );
    final exposure = _exactInt(
      map['current_exposure_minor'],
      'current_exposure_minor',
    );
    final available = _exactInt(
      map['available_credit_minor'],
      'available_credit_minor',
    );
    if (limit - exposure != available) {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'A customer credit balance does not reconcile.',
      );
    }
    final overdue =
        map['overdue_amount_minor'] == null
            ? null
            : _nonNegativeInt(
              map['overdue_amount_minor'],
              'overdue_amount_minor',
            );
    final creditEnabled = _bool(map['credit_enabled'], 'credit_enabled');
    final dueDateValue = _optionalString(map['due_date']);
    return Customer(
      id: _uuidValue(map['id'], 'customer.id'),
      code: _string(map['code'], 'customer.code'),
      name: _string(map['name'], 'customer.name'),
      phone: _optionalString(map['phone']),
      isGeneral: _bool(map['is_general_customer'], 'is_general_customer'),
      isActive: _string(map['status'], 'customer.status') == 'active',
      creditEnabled: creditEnabled,
      inAttendantScope: true,
      credit:
          !creditEnabled || overdue == null
              ? null
              : CreditSnapshot(
                limit: limit,
                currentExposure: exposure,
                overdueAmount: overdue,
                dueDate:
                    dueDateValue == null
                        ? null
                        : _date(dueDateValue, 'due_date'),
              ),
    );
  }

  Product _product(Map<String, Object?> map) {
    if (_string(map['currency'], 'product.currency') != 'TZS') {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'The product price is not denominated in TZS.',
      );
    }
    final taxBasisPoints = _nonNegativeInt(
      map['tax_basis_points'],
      'tax_basis_points',
    );
    if (taxBasisPoints > 10000) {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'tax_basis_points must not exceed 10000.',
      );
    }
    return Product(
      id: _uuidValue(map['id'], 'product.id'),
      code: _string(map['code'], 'product.code'),
      name: _string(map['name'], 'product.name'),
      commonDescription: map['common_description'] as String? ?? '',
      brand: map['brand'] as String? ?? '',
      category: map['category'] as String? ?? '',
      unit: _string(map['unit'], 'product.unit'),
      sellingPrice: _positiveInt(map['unit_price_minor'], 'unit_price_minor'),
      availableQuantity: _wholeQuantity(
        map['available_quantity'],
        'available_quantity',
      ),
      taxBasisPoints: taxBasisPoints,
      masterDataVersion: _positiveInt(
        map['master_data_version'],
        'master_data_version',
      ),
      priceVersion: _positiveInt(map['price_version'], 'price_version'),
    );
  }

  int _wholeQuantity(Object? value, String field) {
    if (value is int) return _exactInt(value, field);
    return _wholeDecimal(_string(value, field), field);
  }

  int _wholeDecimal(String value, String field) {
    final match = RegExp(r'^(-?[0-9]+)(?:\.([0-9]+))?$').firstMatch(value);
    if (match == null ||
        (match.group(2)?.replaceAll('0', '').isNotEmpty ?? false)) {
      throw ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: '$field must be a whole exact value.',
      );
    }
    return _exactInt(int.parse(match.group(1)!), field);
  }

  FiscalStatus _fiscalStatus(String value) => switch (value) {
    'NOT_CONFIGURED' => FiscalStatus.notConfigured,
    'PENDING' => FiscalStatus.pending,
    'FISCALIZED' => FiscalStatus.fiscalized,
    'FAILED' => FiscalStatus.failed,
    _ =>
      throw ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: 'Unknown fiscal status: $value',
      ),
  };

  String _paymentMethod(PaymentMethod value) => switch (value) {
    PaymentMethod.cash => 'CASH',
    PaymentMethod.mobileMoney => 'MOBILE_MONEY',
    PaymentMethod.card => 'BANK_CARD',
    PaymentMethod.bankTransfer => 'BANK_TRANSFER',
  };

  static Map<String, Object?> _map(Object? value, String field) {
    if (value is Map<String, Object?>) return value;
    if (value is Map<Object?, Object?>) {
      final result = <String, Object?>{};
      for (final entry in value.entries) {
        if (entry.key is! String) {
          throw ApiException(
            kind: ApiFailureKind.invalidResponse,
            message: '$field contains a non-string key.',
          );
        }
        result[entry.key! as String] = entry.value;
      }
      return result;
    }
    throw ApiException(
      kind: ApiFailureKind.invalidResponse,
      message: '$field must be an object.',
    );
  }

  static String _string(Object? value, String field) {
    if (value is String && value.isNotEmpty) return value;
    throw ApiException(
      kind: ApiFailureKind.invalidResponse,
      message: '$field must be a non-empty string.',
    );
  }

  static String? _optionalString(Object? value) {
    if (value == null) return null;
    if (value is String) return value;
    throw const ApiException(
      kind: ApiFailureKind.invalidResponse,
      message: 'An optional string field has an invalid value.',
    );
  }

  static bool _bool(Object? value, String field) {
    if (value is bool) return value;
    throw ApiException(
      kind: ApiFailureKind.invalidResponse,
      message: '$field must be a boolean.',
    );
  }

  static int _exactInt(Object? value, String field) {
    if (value is int && value.abs() <= maximumSafeApiInteger) return value;
    throw ApiException(
      kind: ApiFailureKind.invalidResponse,
      message: '$field must be a JavaScript-safe exact integer.',
    );
  }

  static int _positiveInt(Object? value, String field) {
    final result = _exactInt(value, field);
    if (result > 0) return result;
    throw ApiException(
      kind: ApiFailureKind.invalidResponse,
      message: '$field must be positive.',
    );
  }

  static int _nonNegativeInt(Object? value, String field) {
    final result = _exactInt(value, field);
    if (result >= 0) return result;
    throw ApiException(
      kind: ApiFailureKind.invalidResponse,
      message: '$field must not be negative.',
    );
  }

  static String _uuidValue(Object? value, String field) {
    final result = _string(value, field);
    _requireUuid(result, field);
    return result.toLowerCase();
  }

  static String _catalogToken(
    Object? value,
    String field, {
    bool allowUnacknowledged = false,
  }) {
    final result = _string(value, field).toLowerCase();
    if (allowUnacknowledged && result == unacknowledgedCatalogSnapshotToken) {
      return result;
    }
    _requireCatalogToken(result, field);
    return result;
  }

  static void _requireCatalogToken(String value, String field) {
    if (value == unacknowledgedCatalogSnapshotToken ||
        !UuidGenerator.isValid(value)) {
      throw ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: '$field must be an acknowledged catalog snapshot UUID.',
      );
    }
  }

  static void _requireUuid(String value, String field) {
    if (!UuidGenerator.isValid(value)) {
      throw ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: '$field must be a UUID.',
      );
    }
  }

  static DateTime _date(Object? value, String field) {
    try {
      return DateTime.parse(_string(value, field)).toUtc();
    } on FormatException {
      throw ApiException(
        kind: ApiFailureKind.invalidResponse,
        message: '$field must be an RFC 3339 timestamp.',
      );
    }
  }
}
