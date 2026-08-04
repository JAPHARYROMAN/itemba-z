import 'dart:convert';

import '../domain/models.dart';

class LocalDataCodec {
  const LocalDataCodec._();

  static String encodeCustomer(Customer customer) =>
      jsonEncode(customerToMap(customer));

  static Customer decodeCustomer(String value) =>
      customerFromMap(_decodeMap(value));

  static Map<String, Object?> customerToMap(Customer customer) => {
    'id': customer.id,
    'name': customer.name,
    'code': customer.code,
    'isGeneral': customer.isGeneral,
    'isActive': customer.isActive,
    'creditEnabled': customer.creditEnabled,
    'inAttendantScope': customer.inAttendantScope,
    'phone': customer.phone,
    'credit':
        customer.credit == null
            ? null
            : {
              'limit': customer.credit!.limit,
              'currentExposure': customer.credit!.currentExposure,
              'overdueAmount': customer.credit!.overdueAmount,
              'dueDate': customer.credit!.dueDate.millisecondsSinceEpoch,
            },
  };

  static Customer customerFromMap(Map<String, Object?> map) {
    final creditMap = _nullableMap(map['credit']);
    return Customer(
      id: map['id']! as String,
      name: map['name']! as String,
      code: map['code']! as String,
      isGeneral: map['isGeneral']! as bool,
      isActive: map['isActive']! as bool,
      creditEnabled: map['creditEnabled']! as bool,
      inAttendantScope: map['inAttendantScope']! as bool,
      phone: map['phone'] as String?,
      credit:
          creditMap == null
              ? null
              : CreditSnapshot(
                limit: _exactInt(creditMap['limit'], 'credit.limit'),
                currentExposure: _exactInt(
                  creditMap['currentExposure'],
                  'credit.currentExposure',
                ),
                overdueAmount: _exactInt(
                  creditMap['overdueAmount'],
                  'credit.overdueAmount',
                ),
                dueDate: DateTime.fromMillisecondsSinceEpoch(
                  creditMap['dueDate']! as int,
                ),
              ),
    );
  }

  static String encodeProduct(Product product) =>
      jsonEncode(productToMap(product));

  static Product decodeProduct(String value) =>
      productFromMap(_decodeMap(value));

  static Map<String, Object?> productToMap(Product product) => {
    'id': product.id,
    'code': product.code,
    'name': product.name,
    'commonDescription': product.commonDescription,
    'brand': product.brand,
    'category': product.category,
    'unit': product.unit,
    'sellingPrice': product.sellingPrice,
    'availableQuantity': product.availableQuantity,
  };

  static Product productFromMap(Map<String, Object?> map) => Product(
    id: map['id']! as String,
    code: map['code']! as String,
    name: map['name']! as String,
    commonDescription: map['commonDescription']! as String,
    brand: map['brand']! as String,
    category: map['category']! as String,
    unit: map['unit']! as String,
    sellingPrice: _exactInt(map['sellingPrice'], 'product.sellingPrice'),
    availableQuantity: _exactInt(
      map['availableQuantity'],
      'product.availableQuantity',
    ),
  );

  static String encodeDevice(DeviceContext device) =>
      jsonEncode(deviceToMap(device));

  static DeviceContext decodeDevice(String value) =>
      deviceFromMap(_decodeMap(value));

  static Map<String, Object?> deviceToMap(DeviceContext device) => {
    'deviceId': device.deviceId,
    'userId': device.userId,
    'attendantName': device.attendantName,
    'companyId': device.companyId,
    'companyName': device.companyName,
    'branchId': device.branchId,
    'branchName': device.branchName,
    'warehouseId': device.warehouseId,
    'warehouseName': device.warehouseName,
    'appVersion': device.appVersion,
    'masterDataVersion': device.masterDataVersion,
    'priceVersion': device.priceVersion,
    'approved': device.approved,
  };

  static DeviceContext deviceFromMap(Map<String, Object?> map) => DeviceContext(
    deviceId: map['deviceId']! as String,
    userId: map['userId']! as String,
    attendantName: map['attendantName']! as String,
    companyId: map['companyId']! as String,
    companyName: map['companyName']! as String,
    branchId: map['branchId']! as String,
    branchName: map['branchName']! as String,
    warehouseId: map['warehouseId']! as String,
    warehouseName: map['warehouseName']! as String,
    appVersion: map['appVersion']! as String,
    masterDataVersion: map['masterDataVersion']! as String,
    priceVersion: map['priceVersion']! as String,
    approved: map['approved']! as bool,
  );

  static String encodeSale(CompletedSale sale) => jsonEncode({
    'serverSaleId': sale.serverSaleId,
    'receiptNumber': sale.receiptNumber,
    'clientTransactionId': sale.clientTransactionId,
    'deviceId': sale.deviceId,
    'customer': customerToMap(sale.customer),
    'saleType': sale.saleType.name,
    'paymentMethod': sale.paymentMethod.name,
    'lines':
        sale.lines
            .map(
              (line) => {
                'product': productToMap(line.product),
                'quantity': line.quantity,
                'unitPrice': line.unitPrice,
              },
            )
            .toList(),
    'total': sale.total,
    'createdAt': sale.createdAt.millisecondsSinceEpoch,
    'paymentStatus': sale.paymentStatus,
    'syncStatus': sale.syncStatus.name,
    'syncMessage': sale.syncMessage,
  });

  static CompletedSale decodeSale(String value) {
    final map = _decodeMap(value);
    final lines =
        (map['lines']! as List<Object?>).map((item) {
          final lineMap = _asMap(item);
          final storedProduct = productFromMap(_asMap(lineMap['product']));
          final storedPrice = _exactInt(lineMap['unitPrice'], 'line.unitPrice');
          final product = Product(
            id: storedProduct.id,
            code: storedProduct.code,
            name: storedProduct.name,
            commonDescription: storedProduct.commonDescription,
            brand: storedProduct.brand,
            category: storedProduct.category,
            unit: storedProduct.unit,
            sellingPrice: storedPrice,
            availableQuantity: storedProduct.availableQuantity,
          );
          return CartLine(
            product: product,
            quantity: _exactInt(lineMap['quantity'], 'line.quantity'),
          );
        }).toList();
    return CompletedSale(
      serverSaleId: map['serverSaleId']! as String,
      receiptNumber: map['receiptNumber']! as String,
      clientTransactionId: map['clientTransactionId']! as String,
      deviceId: map['deviceId']! as String,
      customer: customerFromMap(_asMap(map['customer'])),
      saleType: SaleType.values.byName(map['saleType']! as String),
      paymentMethod: PaymentMethod.values.byName(
        map['paymentMethod']! as String,
      ),
      lines: lines,
      total: _exactInt(map['total'], 'sale.total'),
      createdAt: DateTime.fromMillisecondsSinceEpoch(map['createdAt']! as int),
      paymentStatus: map['paymentStatus']! as String,
      syncStatus: SyncStatus.values.byName(map['syncStatus']! as String),
      syncMessage: map['syncMessage'] as String?,
    );
  }

  static String encodeSyncCommand(SyncCommand command) => jsonEncode({
    'deviceId': command.deviceId,
    'userId': command.userId,
    'clientTransactionId': command.clientTransactionId,
    'clientTimestamp': command.clientTimestamp.millisecondsSinceEpoch,
    'companyId': command.companyId,
    'branchId': command.branchId,
    'warehouseId': command.warehouseId,
    'appVersion': command.appVersion,
    'masterDataVersion': command.masterDataVersion,
    'priceVersion': command.priceVersion,
    'syncAttemptNumber': command.syncAttemptNumber,
    'sale': jsonDecode(encodeSale(command.sale)),
  });

  static SyncCommand decodeSyncCommand(String value) {
    final map = _decodeMap(value);
    return SyncCommand(
      deviceId: map['deviceId']! as String,
      userId: map['userId']! as String,
      clientTransactionId: map['clientTransactionId']! as String,
      clientTimestamp: DateTime.fromMillisecondsSinceEpoch(
        map['clientTimestamp']! as int,
      ),
      companyId: map['companyId']! as String,
      branchId: map['branchId']! as String,
      warehouseId: map['warehouseId']! as String,
      appVersion: map['appVersion']! as String,
      masterDataVersion: map['masterDataVersion']! as String,
      priceVersion: map['priceVersion']! as String,
      syncAttemptNumber: map['syncAttemptNumber']! as int,
      sale: decodeSale(jsonEncode(map['sale'])),
    );
  }

  static Map<String, Object?> _decodeMap(String value) =>
      _asMap(jsonDecode(value));

  static Map<String, Object?> _asMap(Object? value) =>
      (value! as Map<Object?, Object?>).map(
        (key, item) => MapEntry(key! as String, item),
      );

  static Map<String, Object?>? _nullableMap(Object? value) =>
      value == null ? null : _asMap(value);

  /// Reads v4 integers and accepts only integral v3 JSON numbers. Fractional
  /// legacy values fail closed rather than being rounded into a different sale.
  static int _exactInt(Object? value, String field) {
    if (value is int) return value;
    if (value is double &&
        value.isFinite &&
        value == value.truncateToDouble()) {
      return value.toInt();
    }
    throw FormatException('$field must be an exact integer');
  }
}
