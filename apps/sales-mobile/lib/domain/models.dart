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
    required this.dueDate,
  });

  /// Exact TZS minor units. One stored minor unit currently equals one shilling.
  final int limit;
  final int currentExposure;
  final int overdueAmount;
  final DateTime dueDate;

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
  });

  final String id;
  final String code;
  final String name;
  final String commonDescription;
  final String brand;
  final String category;
  final String unit;
  final int sellingPrice;
  final int availableQuantity;

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
}

class CartLine {
  CartLine({required this.product, required int quantity})
    : _quantity = quantity,
      unitPrice = product.sellingPrice;

  final Product product;

  /// Captured only from the authoritative product/price cache. The UI has no
  /// price setter, so a sales attendant can never type an arbitrary price.
  final int unitPrice;
  int _quantity;

  int get quantity => _quantity;
  int get subtotal => unitPrice * quantity;

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
    required this.attendantName,
    required this.companyId,
    required this.companyName,
    required this.branchId,
    required this.branchName,
    required this.warehouseId,
    required this.warehouseName,
    required this.appVersion,
    required this.masterDataVersion,
    required this.priceVersion,
    required this.approved,
  });

  final String deviceId;
  final String userId;
  final String attendantName;
  final String companyId;
  final String companyName;
  final String branchId;
  final String branchName;
  final String warehouseId;
  final String warehouseName;
  final String appVersion;
  final String masterDataVersion;
  final String priceVersion;
  final bool approved;
}

class OfflineSalesPolicy {
  const OfflineSalesPolicy({
    required this.enabled,
    required this.transactionValueLimit,
    required this.remainingDailyValue,
    required this.productAllocations,
  });

  final bool enabled;
  final int transactionValueLimit;
  final int remainingDailyValue;
  final Map<String, int> productAllocations;

  int allocationFor(String productId) => productAllocations[productId] ?? 0;
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

  int get total => lines.fold(0, (sum, line) => sum + line.subtotal);

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
    required this.receiptNumber,
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
    this.syncMessage,
  });

  final String serverSaleId;
  final String receiptNumber;
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
  final String? syncMessage;

  CompletedSale copyWith({
    String? serverSaleId,
    String? receiptNumber,
    SyncStatus? syncStatus,
    String? syncMessage,
  }) {
    return CompletedSale(
      serverSaleId: serverSaleId ?? this.serverSaleId,
      receiptNumber: receiptNumber ?? this.receiptNumber,
      clientTransactionId: clientTransactionId,
      deviceId: deviceId,
      customer: customer,
      saleType: saleType,
      paymentMethod: paymentMethod,
      lines: lines,
      total: total,
      createdAt: createdAt,
      paymentStatus: paymentStatus,
      syncStatus: syncStatus ?? this.syncStatus,
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
  final String masterDataVersion;
  final String priceVersion;
  final int syncAttemptNumber;
  final CompletedSale sale;

  String get idempotencyKey => '$deviceId::$clientTransactionId';
}

class SyncResult {
  const SyncResult({
    required this.serverSaleId,
    required this.receiptNumber,
    required this.wasDuplicate,
  });

  final String serverSaleId;
  final String receiptNumber;
  final bool wasDuplicate;
}

String money(int minorUnits) {
  final whole = minorUnits.toString();
  final buffer = StringBuffer();
  for (var index = 0; index < whole.length; index++) {
    final remaining = whole.length - index;
    buffer.write(whole[index]);
    if (remaining > 1 && remaining % 3 == 1) buffer.write(',');
  }
  return 'TZS $buffer';
}
