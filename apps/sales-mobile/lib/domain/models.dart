enum AppLanguage { english, swahili }

enum SaleType { cash, credit }

enum PaymentMethod { cash, mobileMoney, card, bankTransfer }

enum SyncStatus {
  draft,
  pendingSync,
  syncing,
  synced,
  rejected,
  requiresReview,
  reconciliationRequired,
}

enum FiscalStatus { notConfigured, pending, fiscalized, failed }

/// Largest integer that can cross the JSON/OpenAPI boundary without losing
/// precision in JavaScript clients.
const int maximumSafeApiIntegerValue = 9007199254740991;

int? checkedApiAmount(int value) =>
    value >= 0 && value <= maximumSafeApiIntegerValue ? value : null;

/// Multiplies non-negative API amounts without first performing a potentially
/// overflowing multiplication.
int? checkedApiProduct(int left, int right) {
  if (left < 0 || right < 0) return null;
  if (left == 0 || right == 0) return 0;
  if (left > maximumSafeApiIntegerValue ~/ right) return null;
  return left * right;
}

int? checkedApiSum(Iterable<int?> values) {
  var total = 0;
  for (final value in values) {
    if (value == null || value < 0) return null;
    if (value > maximumSafeApiIntegerValue - total) return null;
    total += value;
  }
  return total;
}

/// Calculates half-up tax without constructing `subtotal * basisPoints`,
/// which can exceed the API-safe range even when the final tax does not.
int? checkedTaxAmount(int subtotalMinor, int basisPoints) {
  if (checkedApiAmount(subtotalMinor) == null ||
      basisPoints < 0 ||
      basisPoints > 10000) {
    return null;
  }
  final wholeUnits = subtotalMinor ~/ 10000;
  final remainder = subtotalMinor % 10000;
  final wholeTax = checkedApiProduct(wholeUnits, basisPoints);
  final remainderTax = ((remainder * basisPoints) + 5000) ~/ 10000;
  return checkedApiSum([wholeTax, remainderTax]);
}

class Customer {
  const Customer({
    required this.id,
    required this.name,
    required this.code,
    required this.isGeneral,
    required this.isActive,
    required this.creditEnabled,
    required this.inAttendantScope,
    this.phone,
    this.credit,
  });

  final String id;
  final String name;
  final String code;
  final bool isGeneral;
  final bool isActive;
  final bool creditEnabled;
  final bool inAttendantScope;
  final String? phone;
  final CreditSnapshot? credit;
}

class CreditSnapshot {
  const CreditSnapshot({
    required this.limit,
    required this.currentExposure,
    required this.overdueAmount,
    this.dueDate,
  });

  /// Exact TZS minor units. 100 stored minor units equal TZS 1.00.
  final int limit;
  final int currentExposure;
  final int overdueAmount;
  final DateTime? dueDate;

  int get availableCredit => limit - currentExposure;

  int expectedBalance(int proposedSale) => currentExposure + proposedSale;
}

class Product {
  const Product({
    required this.id,
    required this.code,
    required this.name,
    required this.commonDescription,
    required this.brand,
    required this.category,
    required this.unit,
    required this.sellingPrice,
    required this.availableQuantity,
    this.taxBasisPoints,
    this.masterDataVersion = 1,
    this.priceVersion = 1,
  });

  final String id;
  final String code;
  final String name;
  final String commonDescription;
  final String brand;
  final String category;
  final String unit;

  /// Exact TZS minor units. 100 stored minor units equal TZS 1.00.
  final int sellingPrice;
  final int availableQuantity;
  final int? taxBasisPoints;
  final int masterDataVersion;
  final int priceVersion;

  bool matches(String query) {
    final normalized = query.trim().toLowerCase();
    if (normalized.isEmpty) return true;
    return [
      name,
      code,
      brand,
      category,
      commonDescription,
    ].any((value) => value.toLowerCase().contains(normalized));
  }

  bool get hasValidTaxMetadata {
    final value = taxBasisPoints;
    return value != null && value >= 0 && value <= 10000;
  }

  int taxForSubtotal(int subtotalMinor) {
    final basisPoints = taxBasisPoints;
    if (basisPoints == null || basisPoints < 0 || basisPoints > 10000) {
      throw StateError('Product tax metadata is unavailable.');
    }
    final tax = checkedTaxAmount(subtotalMinor, basisPoints);
    if (tax == null) {
      throw RangeError.value(
        subtotalMinor,
        'subtotalMinor',
        'Tax calculation exceeds the API-safe integer range.',
      );
    }
    return tax;
  }
}

class CartLine {
  CartLine({
    required this.product,
    required int quantity,
    int? unitPrice,
    this.authoritativeSubtotalMinor,
    this.authoritativeTaxMinor,
    this.authoritativeTotalMinor,
  }) : _quantity = quantity,
       unitPrice = unitPrice ?? product.sellingPrice;

  final Product product;

  /// Captured only from the authoritative product/price cache. The UI has no
  /// price setter, so a sales attendant can never type an arbitrary price.
  final int unitPrice;
  final int? authoritativeSubtotalMinor;
  final int? authoritativeTaxMinor;
  final int? authoritativeTotalMinor;
  int _quantity;

  int get quantity => _quantity;
  int? get checkedSubtotal =>
      authoritativeSubtotalMinor == null
          ? checkedApiProduct(unitPrice, quantity)
          : checkedApiAmount(authoritativeSubtotalMinor!);
  int? get checkedTax {
    final authoritative = authoritativeTaxMinor;
    if (authoritative != null) return checkedApiAmount(authoritative);
    if (!product.hasValidTaxMetadata) return 0;
    final subtotalValue = checkedSubtotal;
    if (subtotalValue == null) return null;
    return checkedTaxAmount(subtotalValue, product.taxBasisPoints!);
  }

  int? get checkedLineTotal =>
      authoritativeTotalMinor == null
          ? checkedApiSum([checkedSubtotal, checkedTax])
          : checkedApiAmount(authoritativeTotalMinor!);

  int get subtotal =>
      checkedSubtotal ??
      (throw StateError('Line subtotal exceeds the API-safe integer range.'));
  int get tax =>
      checkedTax ??
      (throw StateError('Line tax exceeds the API-safe integer range.'));
  int get lineTotal =>
      checkedLineTotal ??
      (throw StateError('Line total exceeds the API-safe integer range.'));

  void setQuantity(int value) {
    if (value <= 0 || value > product.availableQuantity) {
      throw ArgumentError.value(value, 'value', 'Quantity is unavailable');
    }
    _quantity = value;
  }
}

class DeviceContext {
  const DeviceContext({
    required this.deviceId,
    required this.userId,
    required this.tenantId,
    required this.attendantName,
    required this.companyId,
    required this.companyName,
    required this.branchId,
    required this.branchName,
    required this.warehouseId,
    required this.warehouseName,
    required this.appVersion,
    required this.catalogSnapshotToken,
    required this.masterDataVersion,
    required this.priceVersion,
    required this.approved,
  });

  final String deviceId;
  final String userId;
  final String tenantId;
  final String attendantName;
  final String companyId;
  final String companyName;
  final String branchId;
  final String branchName;
  final String warehouseId;
  final String warehouseName;
  final String appVersion;
  final String catalogSnapshotToken;
  final int masterDataVersion;
  final int priceVersion;
  final bool approved;

  DeviceContext copyWithVersions({
    required String catalogSnapshotToken,
    required int masterDataVersion,
    required int priceVersion,
  }) {
    return DeviceContext(
      deviceId: deviceId,
      userId: userId,
      tenantId: tenantId,
      attendantName: attendantName,
      companyId: companyId,
      companyName: companyName,
      branchId: branchId,
      branchName: branchName,
      warehouseId: warehouseId,
      warehouseName: warehouseName,
      appVersion: appVersion,
      catalogSnapshotToken: catalogSnapshotToken,
      masterDataVersion: masterDataVersion,
      priceVersion: priceVersion,
      approved: approved,
    );
  }

  DeviceContext copyWithAcknowledgedRuntime({
    required String appVersion,
    required String catalogSnapshotToken,
    required int masterDataVersion,
    required int priceVersion,
    required bool approved,
  }) {
    return DeviceContext(
      deviceId: deviceId,
      userId: userId,
      tenantId: tenantId,
      attendantName: attendantName,
      companyId: companyId,
      companyName: companyName,
      branchId: branchId,
      branchName: branchName,
      warehouseId: warehouseId,
      warehouseName: warehouseName,
      appVersion: appVersion,
      catalogSnapshotToken: catalogSnapshotToken,
      masterDataVersion: masterDataVersion,
      priceVersion: priceVersion,
      approved: approved,
    );
  }
}

class OfflineSalesPolicy {
  const OfflineSalesPolicy({
    required this.enabled,
    required this.transactionValueLimit,
    required this.remainingDailyValue,
    required this.offlineSalesValidUntil,
    required this.productAllocations,
  });

  final bool enabled;
  final int transactionValueLimit;
  final int remainingDailyValue;
  final DateTime offlineSalesValidUntil;
  final Map<String, int> productAllocations;

  int allocationFor(String productId) => productAllocations[productId] ?? 0;

  bool isLeaseValidAt(DateTime instant) =>
      instant.toUtc().isBefore(offlineSalesValidUntil.toUtc());
}

class SaleDraft {
  SaleDraft({
    required this.clientTransactionId,
    required this.createdAt,
    required this.device,
    required this.saleType,
    required this.customer,
  });

  final String clientTransactionId;
  final DateTime createdAt;
  final DeviceContext device;
  SaleType saleType;
  Customer customer;
  PaymentMethod paymentMethod = PaymentMethod.cash;
  final List<CartLine> lines = [];

  int? get checkedSubtotal =>
      checkedApiSum(lines.map((line) => line.checkedSubtotal));
  int? get checkedTax => checkedApiSum(lines.map((line) => line.checkedTax));
  int? get checkedTotal =>
      checkedApiSum(lines.map((line) => line.checkedLineTotal));
  bool get hasApiSafeAmounts =>
      checkedSubtotal != null && checkedTax != null && checkedTotal != null;

  int get subtotal =>
      checkedSubtotal ??
      (throw StateError('Sale subtotal exceeds the API-safe integer range.'));
  int get tax =>
      checkedTax ??
      (throw StateError('Sale tax exceeds the API-safe integer range.'));
  int get total =>
      checkedTotal ??
      (throw StateError('Sale total exceeds the API-safe integer range.'));

  void addProduct(Product product) {
    final existing = lines.where((line) => line.product.id == product.id);
    if (existing.isNotEmpty) {
      existing.first.setQuantity(existing.first.quantity + 1);
    } else {
      lines.add(CartLine(product: product, quantity: 1));
    }
  }

  void removeProduct(String productId) {
    lines.removeWhere((line) => line.product.id == productId);
  }
}

class CompletedSale {
  const CompletedSale({
    required this.serverSaleId,
    required this.receiptReference,
    required this.clientTransactionId,
    required this.deviceId,
    required this.customer,
    required this.saleType,
    required this.paymentMethod,
    required this.lines,
    required this.total,
    required this.createdAt,
    required this.paymentStatus,
    required this.syncStatus,
    this.subtotalMinor,
    this.taxMinor,
    this.cogsMinor,
    this.fiscalStatus = FiscalStatus.notConfigured,
    this.createdOffline = false,
    this.syncMessage,
  });

  final String serverSaleId;

  /// Authoritative internal reference. This is not a TRA fiscal receipt number.
  final String receiptReference;
  final String clientTransactionId;
  final String deviceId;
  final Customer customer;
  final SaleType saleType;
  final PaymentMethod paymentMethod;
  final List<CartLine> lines;
  final int total;
  final DateTime createdAt;
  final String paymentStatus;
  final SyncStatus syncStatus;
  final int? subtotalMinor;
  final int? taxMinor;
  final int? cogsMinor;
  final FiscalStatus fiscalStatus;
  final bool createdOffline;
  final String? syncMessage;

  CompletedSale copyWith({
    String? serverSaleId,
    String? receiptReference,
    SyncStatus? syncStatus,
    FiscalStatus? fiscalStatus,
    int? total,
    List<CartLine>? lines,
    int? subtotalMinor,
    int? taxMinor,
    int? cogsMinor,
    String? syncMessage,
  }) {
    return CompletedSale(
      serverSaleId: serverSaleId ?? this.serverSaleId,
      receiptReference: receiptReference ?? this.receiptReference,
      clientTransactionId: clientTransactionId,
      deviceId: deviceId,
      customer: customer,
      saleType: saleType,
      paymentMethod: paymentMethod,
      lines: lines ?? this.lines,
      total: total ?? this.total,
      createdAt: createdAt,
      paymentStatus: paymentStatus,
      syncStatus: syncStatus ?? this.syncStatus,
      subtotalMinor: subtotalMinor ?? this.subtotalMinor,
      taxMinor: taxMinor ?? this.taxMinor,
      cogsMinor: cogsMinor ?? this.cogsMinor,
      fiscalStatus: fiscalStatus ?? this.fiscalStatus,
      createdOffline: createdOffline,
      syncMessage: syncMessage ?? this.syncMessage,
    );
  }
}

class SyncCommand {
  const SyncCommand({
    required this.deviceId,
    required this.userId,
    required this.clientTransactionId,
    required this.clientTimestamp,
    required this.companyId,
    required this.branchId,
    required this.warehouseId,
    required this.appVersion,
    required this.catalogSnapshotToken,
    required this.masterDataVersion,
    required this.priceVersion,
    required this.syncAttemptNumber,
    required this.sale,
  });

  final String deviceId;
  final String userId;
  final String clientTransactionId;
  final DateTime clientTimestamp;
  final String companyId;
  final String branchId;
  final String warehouseId;
  final String appVersion;
  final String catalogSnapshotToken;
  final int masterDataVersion;
  final int priceVersion;
  final int syncAttemptNumber;
  final CompletedSale sale;

  String get idempotencyKey => '$deviceId::$clientTransactionId';
}

class SyncResult {
  const SyncResult({
    required this.serverSaleId,
    required this.receiptReference,
    required this.fiscalStatus,
    this.serverTotalMinor,
    this.serverSubtotalMinor,
    this.serverTaxMinor,
    this.serverCogsMinor,
    this.serverLines = const [],
    required this.wasDuplicate,
  });

  final String serverSaleId;
  final String receiptReference;
  final FiscalStatus fiscalStatus;
  final int? serverTotalMinor;
  final int? serverSubtotalMinor;
  final int? serverTaxMinor;
  final int? serverCogsMinor;
  final List<AuthoritativeSaleLine> serverLines;
  final bool wasDuplicate;
}

class AuthoritativeSaleLine {
  const AuthoritativeSaleLine({
    required this.productId,
    required this.quantity,
    required this.unitPriceMinor,
    required this.subtotalMinor,
    required this.taxMinor,
    required this.totalMinor,
  });

  final String productId;
  final int quantity;
  final int unitPriceMinor;
  final int subtotalMinor;
  final int taxMinor;
  final int totalMinor;
}

String money(int minorUnits) {
  final negative = minorUnits < 0;
  final absolute = minorUnits.abs();
  final whole = (absolute ~/ 100).toString();
  final fraction = (absolute % 100).toString().padLeft(2, '0');
  final buffer = StringBuffer();
  for (var index = 0; index < whole.length; index++) {
    final remaining = whole.length - index;
    buffer.write(whole[index]);
    if (remaining > 1 && remaining % 3 == 1) buffer.write(',');
  }
  return '${negative ? '-' : ''}TZS $buffer.$fraction';
}
