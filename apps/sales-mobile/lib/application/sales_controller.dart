import 'package:flutter/foundation.dart';

import '../data/demo_data.dart';
import '../data/local_store.dart';
import '../data/sync_service.dart';
import '../domain/models.dart';
import '../domain/sale_rules.dart';

class ClientTransactionIdGenerator {
  int _sequence = 0;

  String next(DeviceContext device, DateTime now) {
    _sequence += 1;
    return '${device.deviceId}-${now.microsecondsSinceEpoch}-$_sequence';
  }
}

class SalesController extends ChangeNotifier {
  SalesController({
    DeviceContext? device,
    List<Customer>? customers,
    List<Product>? products,
    OfflineSalesPolicy? offlinePolicy,
    EncryptedLocalStore? store,
    AuthoritativeSyncGateway? gateway,
    ClientTransactionIdGenerator? idGenerator,
  }) : device = device ?? demoDevice,
       offlinePolicy = offlinePolicy ?? demoOfflinePolicy,
       store = store ?? InMemoryEncryptedLocalStore(),
       gateway = gateway ?? InMemoryAuthoritativeSyncGateway(),
       _idGenerator = idGenerator ?? ClientTransactionIdGenerator() {
    _customers = List.of(customers ?? demoCustomers);
    _products = List.of(products ?? demoProducts);
    syncService = SalesSyncService(store: this.store, gateway: this.gateway);
  }

  final DeviceContext device;
  final OfflineSalesPolicy offlinePolicy;
  final EncryptedLocalStore store;
  final AuthoritativeSyncGateway gateway;
  final ClientTransactionIdGenerator _idGenerator;
  late final SalesSyncService syncService;
  late List<Customer> _customers;
  late List<Product> _products;

  AppLanguage language = AppLanguage.english;
  bool isOnline = true;
  bool isSyncing = false;
  DateTime? lastSyncAt;
  final List<CompletedSale> sales = [];
  final List<SaleDraft> drafts = [];

  List<Customer> get customers => List.unmodifiable(_customers);
  List<Product> get products => List.unmodifiable(_products);

  Customer get generalCustomer =>
      customers.firstWhere((customer) => customer.isGeneral);

  int get pendingCount =>
      sales
          .where(
            (sale) =>
                sale.syncStatus == SyncStatus.pendingSync ||
                sale.syncStatus == SyncStatus.rejected ||
                sale.syncStatus == SyncStatus.requiresReview,
          )
          .length;

  int get todayTotal => sales.fold(0, (sum, sale) => sum + sale.total);

  Future<void> initializeLocalData() async {
    await store.initialize();
    final cachedCustomers = await store.readCachedCustomers();
    if (cachedCustomers.isEmpty) {
      await store.replaceCachedCustomers(
        _customers,
        masterDataVersion: device.masterDataVersion,
      );
    } else {
      _customers = List.of(cachedCustomers);
    }

    final cachedProducts = await store.readCachedProducts();
    if (cachedProducts.isEmpty) {
      await store.replaceCachedProducts(
        _products,
        masterDataVersion: device.masterDataVersion,
        priceVersion: device.priceVersion,
      );
    } else {
      _products = List.of(cachedProducts);
    }

    sales
      ..clear()
      ..addAll(await store.readSales());
    drafts
      ..clear()
      ..addAll(await store.readDrafts());
    notifyListeners();
  }

  void toggleLanguage() {
    language =
        language == AppLanguage.english
            ? AppLanguage.swahili
            : AppLanguage.english;
    notifyListeners();
  }

  void setLanguage(AppLanguage value) {
    if (language == value) return;
    language = value;
    notifyListeners();
  }

  void setConnectivity(bool online) {
    if (isOnline == online) return;
    isOnline = online;
    notifyListeners();
  }

  SaleDraft createNewSale({DateTime? now}) {
    final timestamp = now ?? DateTime.now();
    return SaleDraft(
      clientTransactionId: _idGenerator.next(device, timestamp),
      createdAt: timestamp,
      device: device,
      saleType: SaleType.cash,
      customer: generalCustomer,
    );
  }

  Iterable<Customer> eligibleCustomers(SaleType type) => customers.where(
    (customer) => SaleRules.isCustomerEligible(customer, type),
  );

  List<Product> searchProducts(String query) =>
      products.where((product) => product.matches(query)).toList();

  Future<void> saveDraft(SaleDraft draft) async {
    if (draft.lines.isEmpty) return;
    await store.saveDraft(draft);
    final existingIndex = drafts.indexWhere(
      (item) => item.clientTransactionId == draft.clientTransactionId,
    );
    if (existingIndex >= 0) {
      drafts[existingIndex] = draft;
    } else {
      drafts.insert(0, draft);
    }
    notifyListeners();
  }

  Future<CompletedSale> completeSale(SaleDraft draft) async {
    SaleRules.enforceCompletion(
      draft: draft,
      isOnline: isOnline,
      offlinePolicy: offlinePolicy,
    );

    var sale = CompletedSale(
      serverSaleId: '',
      receiptNumber: isOnline ? 'Pending' : 'Offline pending',
      clientTransactionId: draft.clientTransactionId,
      deviceId: device.deviceId,
      customer: draft.customer,
      saleType: draft.saleType,
      paymentMethod: draft.paymentMethod,
      lines: List.unmodifiable(draft.lines),
      total: draft.total,
      createdAt: draft.createdAt,
      paymentStatus: draft.saleType == SaleType.cash ? 'Paid' : 'Receivable',
      syncStatus: isOnline ? SyncStatus.syncing : SyncStatus.pendingSync,
    );

    await store.saveSale(sale);
    if (isOnline) {
      sale = await _syncOne(sale, attempt: 1);
    } else {
      await store.enqueueSync(_syncCommand(sale, attempt: 1));
    }
    sales.insert(0, sale);
    await store.deleteDraft(draft.clientTransactionId);
    drafts.removeWhere(
      (item) => item.clientTransactionId == draft.clientTransactionId,
    );
    notifyListeners();
    return sale;
  }

  Future<CompletedSale> _syncOne(
    CompletedSale sale, {
    required int attempt,
  }) async {
    final command = _syncCommand(sale, attempt: attempt);
    await store.enqueueSync(command);
    try {
      final result = await syncService.synchronize(command);
      final synced = sale.copyWith(
        serverSaleId: result.serverSaleId,
        receiptNumber: result.receiptNumber,
        syncStatus: SyncStatus.synced,
        syncMessage:
            result.wasDuplicate
                ? 'Original server result restored safely.'
                : 'Accepted by server.',
      );
      await store.replaceSale(synced);
      await store.removeSyncCommand(command.idempotencyKey);
      return synced;
    } catch (_) {
      final rejected = sale.copyWith(
        syncStatus: SyncStatus.requiresReview,
        syncMessage: 'Synchronization failed. Retry or review.',
      );
      await store.replaceSale(rejected);
      return rejected;
    }
  }

  SyncCommand _syncCommand(CompletedSale sale, {required int attempt}) {
    return SyncCommand(
      deviceId: device.deviceId,
      userId: device.userId,
      clientTransactionId: sale.clientTransactionId,
      clientTimestamp: sale.createdAt,
      companyId: device.companyId,
      branchId: device.branchId,
      warehouseId: device.warehouseId,
      appVersion: device.appVersion,
      masterDataVersion: device.masterDataVersion,
      priceVersion: device.priceVersion,
      syncAttemptNumber: attempt,
      sale: sale,
    );
  }

  Future<void> synchronizePending() async {
    if (!isOnline || isSyncing) return;
    isSyncing = true;
    notifyListeners();
    for (var index = 0; index < sales.length; index++) {
      final sale = sales[index];
      if (sale.syncStatus == SyncStatus.pendingSync ||
          sale.syncStatus == SyncStatus.requiresReview) {
        sales[index] = sale.copyWith(syncStatus: SyncStatus.syncing);
        notifyListeners();
        sales[index] = await _syncOne(sale, attempt: 2);
      }
    }
    lastSyncAt = DateTime.now();
    isSyncing = false;
    notifyListeners();
  }
}
