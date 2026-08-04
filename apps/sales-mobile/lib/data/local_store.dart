import '../domain/models.dart';
import '../domain/connection_models.dart';

class InstalledMasterDataVersions {
  const InstalledMasterDataVersions({
    required this.catalogSnapshotToken,
    required this.masterDataVersion,
    required this.priceVersion,
  });

  final String catalogSnapshotToken;
  final int masterDataVersion;
  final int priceVersion;

  bool matches({
    required String catalogSnapshotToken,
    required int masterDataVersion,
    required int priceVersion,
  }) =>
      this.catalogSnapshotToken == catalogSnapshotToken &&
      this.masterDataVersion == masterDataVersion &&
      this.priceVersion == priceVersion;
}

/// Persistence boundary for data that may be used while the device is offline.
/// Production uses [SqlCipherLocalStore]; tests may use the memory adapter.
abstract interface class EncryptedLocalStore {
  Future<void> initialize();

  Future<void> saveConnection(MobileConnection connection);
  Future<MobileConnection?> readConnection();
  Future<void> saveCandidateDeviceId(String deviceId);
  Future<String?> readCandidateDeviceId();
  Future<void> saveEnrollment(DeviceEnrollment enrollment);
  Future<DeviceEnrollment?> readEnrollment();
  Future<void> saveAllocation(DeviceAllocation allocation);
  Future<DeviceAllocation?> readAllocation();

  Future<void> replaceCachedCustomers(
    List<Customer> customers, {
    required int masterDataVersion,
  });
  Future<List<Customer>> readCachedCustomers();

  Future<void> replaceCachedProducts(
    List<Product> products, {
    required int masterDataVersion,
    required int priceVersion,
  });
  Future<List<Product>> readCachedProducts();
  Future<InstalledMasterDataVersions?> readInstalledMasterDataVersions();
  Future<void> installMasterData({
    required List<Customer> customers,
    required List<Product> products,
    required int masterDataVersion,
    required int priceVersion,
    required String catalogSnapshotToken,
    DeviceEnrollment? enrollment,
  });

  Future<void> saveDraft(SaleDraft draft);
  Future<List<SaleDraft>> readDrafts();
  Future<void> deleteDraft(String clientTransactionId);

  Future<void> saveSale(CompletedSale sale);
  Future<void> persistPendingSync({
    required CompletedSale sale,
    required SyncCommand command,
  });
  Future<void> persistOfflineCompletion({
    required CompletedSale sale,
    required SyncCommand command,
    required DeviceAllocation allocation,
  });
  Future<List<CompletedSale>> readSales();
  Future<void> replaceSale(CompletedSale sale);

  Future<void> enqueueSync(SyncCommand command);
  Future<List<SyncCommand>> readSyncQueue();
  Future<void> removeSyncCommand(String idempotencyKey);

  Future<void> saveSyncResult(String idempotencyKey, SyncResult result);
  Future<SyncResult?> readSyncResult(String idempotencyKey);
  Future<void> finalizeSync({
    required CompletedSale sale,
    required SyncCommand command,
    required SyncResult result,
  });
  Future<void> recordSyncFailure({
    required CompletedSale sale,
    required String idempotencyKey,
    required bool authoritativeRejection,
  });

  Future<void> clearSensitiveCache();
  Future<void> close();
}

class InMemoryEncryptedLocalStore implements EncryptedLocalStore {
  MobileConnection? _connection;
  String? _candidateDeviceId;
  DeviceEnrollment? _enrollment;
  DeviceAllocation? _allocation;
  final Map<String, Customer> _customers = {};
  final Map<String, Product> _products = {};
  int? _customerMasterDataVersion;
  int? _productMasterDataVersion;
  int? _productPriceVersion;
  String? _catalogSnapshotToken;
  final Map<String, SaleDraft> _drafts = {};
  final Map<String, CompletedSale> _sales = {};
  final Map<String, SyncCommand> _syncQueue = {};
  final Map<String, SyncResult> _syncResults = {};

  @override
  Future<void> initialize() async {}

  @override
  Future<void> saveConnection(MobileConnection connection) async {
    _connection = connection;
  }

  @override
  Future<MobileConnection?> readConnection() async => _connection;

  @override
  Future<void> saveCandidateDeviceId(String deviceId) async {
    _candidateDeviceId = deviceId;
  }

  @override
  Future<String?> readCandidateDeviceId() async => _candidateDeviceId;

  @override
  Future<void> saveEnrollment(DeviceEnrollment enrollment) async {
    _enrollment = enrollment;
  }

  @override
  Future<DeviceEnrollment?> readEnrollment() async => _enrollment;

  @override
  Future<void> saveAllocation(DeviceAllocation allocation) async {
    _allocation = allocation;
  }

  @override
  Future<DeviceAllocation?> readAllocation() async => _allocation;

  @override
  Future<void> replaceCachedCustomers(
    List<Customer> customers, {
    required int masterDataVersion,
  }) async {
    _customers
      ..clear()
      ..addEntries(
        customers.map((customer) => MapEntry(customer.id, customer)),
      );
    _customerMasterDataVersion = masterDataVersion;
  }

  @override
  Future<List<Customer>> readCachedCustomers() async =>
      List.unmodifiable(_customers.values);

  @override
  Future<void> replaceCachedProducts(
    List<Product> products, {
    required int masterDataVersion,
    required int priceVersion,
  }) async {
    _products
      ..clear()
      ..addEntries(products.map((product) => MapEntry(product.id, product)));
    _productMasterDataVersion = masterDataVersion;
    _productPriceVersion = priceVersion;
  }

  @override
  Future<List<Product>> readCachedProducts() async =>
      List.unmodifiable(_products.values);

  @override
  Future<InstalledMasterDataVersions?> readInstalledMasterDataVersions() async {
    if (_customers.isEmpty || _products.isEmpty) return null;
    final customerVersion = _customerMasterDataVersion;
    final productVersion = _productMasterDataVersion;
    final priceVersion = _productPriceVersion;
    if (customerVersion == null ||
        productVersion == null ||
        priceVersion == null ||
        _catalogSnapshotToken == null ||
        customerVersion != productVersion) {
      return null;
    }
    return InstalledMasterDataVersions(
      catalogSnapshotToken: _catalogSnapshotToken!,
      masterDataVersion: customerVersion,
      priceVersion: priceVersion,
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
    _customers
      ..clear()
      ..addEntries(
        customers.map((customer) => MapEntry(customer.id, customer)),
      );
    _products
      ..clear()
      ..addEntries(products.map((product) => MapEntry(product.id, product)));
    _customerMasterDataVersion = masterDataVersion;
    _productMasterDataVersion = masterDataVersion;
    _productPriceVersion = priceVersion;
    _catalogSnapshotToken = catalogSnapshotToken;
    if (enrollment != null) _enrollment = enrollment;
  }

  @override
  Future<void> saveDraft(SaleDraft draft) async {
    _drafts[draft.clientTransactionId] = draft;
  }

  @override
  Future<List<SaleDraft>> readDrafts() async {
    final values =
        _drafts.values.toList()
          ..sort((a, b) => b.createdAt.compareTo(a.createdAt));
    return values;
  }

  @override
  Future<void> deleteDraft(String clientTransactionId) async {
    _drafts.remove(clientTransactionId);
  }

  @override
  Future<void> saveSale(CompletedSale sale) async {
    _sales.putIfAbsent(sale.clientTransactionId, () => sale);
  }

  @override
  Future<void> persistPendingSync({
    required CompletedSale sale,
    required SyncCommand command,
  }) async {
    _sales.putIfAbsent(sale.clientTransactionId, () => sale);
    _syncQueue[command.idempotencyKey] = command;
    _drafts.remove(sale.clientTransactionId);
  }

  @override
  Future<void> persistOfflineCompletion({
    required CompletedSale sale,
    required SyncCommand command,
    required DeviceAllocation allocation,
  }) async {
    _sales.putIfAbsent(sale.clientTransactionId, () => sale);
    _syncQueue[command.idempotencyKey] = command;
    _allocation = allocation;
    _drafts.remove(sale.clientTransactionId);
  }

  @override
  Future<List<CompletedSale>> readSales() async {
    final values =
        _sales.values.toList()
          ..sort((a, b) => b.createdAt.compareTo(a.createdAt));
    return values;
  }

  @override
  Future<void> replaceSale(CompletedSale sale) async {
    _sales[sale.clientTransactionId] = sale;
  }

  @override
  Future<void> enqueueSync(SyncCommand command) async {
    _syncQueue[command.idempotencyKey] = command;
  }

  @override
  Future<List<SyncCommand>> readSyncQueue() async =>
      List.unmodifiable(_syncQueue.values);

  @override
  Future<void> removeSyncCommand(String idempotencyKey) async {
    _syncQueue.remove(idempotencyKey);
  }

  @override
  Future<void> saveSyncResult(String idempotencyKey, SyncResult result) async {
    _syncResults.putIfAbsent(idempotencyKey, () => result);
  }

  @override
  Future<SyncResult?> readSyncResult(String idempotencyKey) async =>
      _syncResults[idempotencyKey];

  @override
  Future<void> finalizeSync({
    required CompletedSale sale,
    required SyncCommand command,
    required SyncResult result,
  }) async {
    _syncResults.putIfAbsent(command.idempotencyKey, () => result);
    _sales[sale.clientTransactionId] = sale;
    _syncQueue.remove(command.idempotencyKey);
  }

  @override
  Future<void> recordSyncFailure({
    required CompletedSale sale,
    required String idempotencyKey,
    required bool authoritativeRejection,
  }) async {
    _sales[sale.clientTransactionId] = sale;
    if (authoritativeRejection) _syncQueue.remove(idempotencyKey);
  }

  @override
  Future<void> clearSensitiveCache() async {
    _connection = null;
    _candidateDeviceId = null;
    _enrollment = null;
    _allocation = null;
    _customers.clear();
    _products.clear();
    _customerMasterDataVersion = null;
    _productMasterDataVersion = null;
    _productPriceVersion = null;
    _catalogSnapshotToken = null;
    _drafts.clear();
    _sales.clear();
    _syncQueue.clear();
    _syncResults.clear();
  }

  @override
  Future<void> close() async {}
}
