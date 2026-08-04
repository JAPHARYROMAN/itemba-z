import '../domain/models.dart';

/// Persistence boundary for data that may be used while the device is offline.
/// Production uses [SqlCipherLocalStore]; tests may use the memory adapter.
abstract interface class EncryptedLocalStore {
  Future<void> initialize();

  Future<void> replaceCachedCustomers(
    List<Customer> customers, {
    required String masterDataVersion,
  });
  Future<List<Customer>> readCachedCustomers();

  Future<void> replaceCachedProducts(
    List<Product> products, {
    required String masterDataVersion,
    required String priceVersion,
  });
  Future<List<Product>> readCachedProducts();

  Future<void> saveDraft(SaleDraft draft);
  Future<List<SaleDraft>> readDrafts();
  Future<void> deleteDraft(String clientTransactionId);

  Future<void> saveSale(CompletedSale sale);
  Future<List<CompletedSale>> readSales();
  Future<void> replaceSale(CompletedSale sale);

  Future<void> enqueueSync(SyncCommand command);
  Future<List<SyncCommand>> readSyncQueue();
  Future<void> removeSyncCommand(String idempotencyKey);

  Future<void> saveSyncResult(String idempotencyKey, SyncResult result);
  Future<SyncResult?> readSyncResult(String idempotencyKey);

  Future<void> clearSensitiveCache();
  Future<void> close();
}

class InMemoryEncryptedLocalStore implements EncryptedLocalStore {
  final Map<String, Customer> _customers = {};
  final Map<String, Product> _products = {};
  final Map<String, SaleDraft> _drafts = {};
  final Map<String, CompletedSale> _sales = {};
  final Map<String, SyncCommand> _syncQueue = {};
  final Map<String, SyncResult> _syncResults = {};

  @override
  Future<void> initialize() async {}

  @override
  Future<void> replaceCachedCustomers(
    List<Customer> customers, {
    required String masterDataVersion,
  }) async {
    _customers
      ..clear()
      ..addEntries(
        customers.map((customer) => MapEntry(customer.id, customer)),
      );
  }

  @override
  Future<List<Customer>> readCachedCustomers() async =>
      List.unmodifiable(_customers.values);

  @override
  Future<void> replaceCachedProducts(
    List<Product> products, {
    required String masterDataVersion,
    required String priceVersion,
  }) async {
    _products
      ..clear()
      ..addEntries(products.map((product) => MapEntry(product.id, product)));
  }

  @override
  Future<List<Product>> readCachedProducts() async =>
      List.unmodifiable(_products.values);

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
  Future<void> clearSensitiveCache() async {
    _customers.clear();
    _products.clear();
    _drafts.clear();
    _sales.clear();
    _syncQueue.clear();
    _syncResults.clear();
  }

  @override
  Future<void> close() async {}
}
