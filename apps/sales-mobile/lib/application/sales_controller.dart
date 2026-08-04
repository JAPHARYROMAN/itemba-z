import 'package:flutter/foundation.dart';

import '../core/uuid.dart';
import '../data/demo_data.dart';
import '../data/local_store.dart';
import '../data/sync_service.dart';
import '../domain/connection_models.dart';
import '../domain/models.dart';
import '../domain/sale_rules.dart';

class ClientTransactionIdGenerator {
  ClientTransactionIdGenerator({UuidGenerator? uuid})
    : _uuid = uuid ?? UuidGenerator();

  final UuidGenerator _uuid;

  String next(DeviceContext device, DateTime now) {
    return _uuid.v4();
  }
}

class SalesController extends ChangeNotifier {
  SalesController({
    DeviceContext? device,
    List<Customer>? customers,
    List<Product>? products,
    OfflineSalesPolicy? offlinePolicy,
    DeviceAllocation? deviceAllocation,
    EncryptedLocalStore? store,
    AuthoritativeSyncGateway? gateway,
    ClientTransactionIdGenerator? idGenerator,
    DateTime Function()? clock,
    this.connection,
    this.enrollment,
    this.masterDataRefresher,
    this.liveConnectionState = LiveConnectionState.ready,
    this.allowConnectivitySimulation = true,
  }) : device = device ?? demoDevice,
       offlinePolicy = offlinePolicy ?? demoOfflinePolicy,
       _deviceAllocation = deviceAllocation,
       store = store ?? InMemoryEncryptedLocalStore(),
       gateway = gateway ?? InMemoryAuthoritativeSyncGateway(),
       _idGenerator = idGenerator ?? ClientTransactionIdGenerator(),
       _clock = clock ?? DateTime.now {
    _customers = List.of(customers ?? demoCustomers);
    _products = List.of(products ?? demoProducts);
    syncService = SalesSyncService(store: this.store, gateway: this.gateway);
  }

  DeviceContext device;
  OfflineSalesPolicy offlinePolicy;
  DeviceAllocation? _deviceAllocation;
  final EncryptedLocalStore store;
  final AuthoritativeSyncGateway gateway;
  final ClientTransactionIdGenerator _idGenerator;
  final DateTime Function() _clock;
  final MobileConnection? connection;
  DeviceEnrollment? enrollment;
  final Future<void> Function()? masterDataRefresher;
  final bool allowConnectivitySimulation;
  late final SalesSyncService syncService;
  late List<Customer> _customers;
  late List<Product> _products;

  AppLanguage language = AppLanguage.english;
  bool isOnline = true;
  bool isSyncing = false;
  bool isRefreshingMasterData = false;
  LiveConnectionState liveConnectionState;
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

  Future<void> initializeLocalData({bool seedWhenEmpty = true}) async {
    await store.initialize();
    final cachedCustomers = await store.readCachedCustomers();
    if (cachedCustomers.isEmpty && seedWhenEmpty) {
      await store.replaceCachedCustomers(
        _customers,
        masterDataVersion: device.masterDataVersion,
      );
    } else {
      _customers = List.of(cachedCustomers);
    }

    final cachedProducts = await store.readCachedProducts();
    if (cachedProducts.isEmpty && seedWhenEmpty) {
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
    final queued = await store.readSyncQueue();
    final queuedTransactionIds = <String>{};
    for (final command in queued) {
      queuedTransactionIds.add(command.clientTransactionId);
      final index = sales.indexWhere(
        (sale) => sale.clientTransactionId == command.clientTransactionId,
      );
      final recovered = (index >= 0 ? sales[index] : command.sale).copyWith(
        syncStatus: SyncStatus.pendingSync,
        syncMessage: 'Recovered from the durable synchronization queue.',
      );
      if (index >= 0) {
        sales[index] = recovered;
      } else {
        sales.add(recovered);
      }
      await store.replaceSale(recovered);
    }
    for (var index = 0; index < sales.length; index += 1) {
      final sale = sales[index];
      if (sale.syncStatus == SyncStatus.syncing &&
          !queuedTransactionIds.contains(sale.clientTransactionId)) {
        final interrupted = sale.copyWith(
          syncStatus: SyncStatus.requiresReview,
          syncMessage:
              'Interrupted before a durable command was recorded. Do not repost.',
        );
        sales[index] = interrupted;
        await store.replaceSale(interrupted);
      }
    }
    sales.sort((a, b) => b.createdAt.compareTo(a.createdAt));
    drafts
      ..clear()
      ..addAll(await store.readDrafts());
    notifyListeners();
  }

  Future<void> replaceMasterData({
    required List<Customer> customers,
    required List<Product> products,
    required int masterDataVersion,
    required int priceVersion,
    required String catalogSnapshotToken,
    DeviceEnrollment? enrollmentToInstall,
  }) async {
    if (!customers.any((customer) => customer.isGeneral && customer.isActive)) {
      throw StateError('An active General Customer is required.');
    }
    if (products.isEmpty) throw StateError('At least one product is required.');
    if (products.any(
      (product) =>
          product.masterDataVersion != masterDataVersion ||
          product.priceVersion != priceVersion,
    )) {
      throw StateError('Product cache versions do not match the snapshot.');
    }
    if (enrollmentToInstall != null &&
        (enrollmentToInstall.catalogSnapshotToken != catalogSnapshotToken ||
            enrollmentToInstall.masterDataVersion != masterDataVersion ||
            enrollmentToInstall.priceVersion != priceVersion)) {
      throw StateError('Enrollment and cache versions do not match.');
    }
    await store.installMasterData(
      customers: customers,
      products: products,
      masterDataVersion: masterDataVersion,
      priceVersion: priceVersion,
      catalogSnapshotToken: catalogSnapshotToken,
      enrollment: enrollmentToInstall,
    );
    _customers = List.of(customers);
    _products = List.of(products);
    if (enrollmentToInstall != null) {
      enrollment = enrollmentToInstall;
      device = device.copyWithAcknowledgedRuntime(
        appVersion: enrollmentToInstall.appVersion,
        catalogSnapshotToken: catalogSnapshotToken,
        masterDataVersion: masterDataVersion,
        priceVersion: priceVersion,
        approved: enrollmentToInstall.isActive,
      );
    } else {
      device = device.copyWithVersions(
        catalogSnapshotToken: catalogSnapshotToken,
        masterDataVersion: masterDataVersion,
        priceVersion: priceVersion,
      );
    }
    notifyListeners();
  }

  void applyAcknowledgedEnrollment(DeviceEnrollment value) {
    enrollment = value;
    device = device.copyWithAcknowledgedRuntime(
      appVersion: value.appVersion,
      catalogSnapshotToken: value.catalogSnapshotToken,
      masterDataVersion: value.masterDataVersion,
      priceVersion: value.priceVersion,
      approved: value.isActive,
    );
    notifyListeners();
  }

  Future<void> refreshLiveMasterData() async {
    final refresh = masterDataRefresher;
    if (refresh == null || isRefreshingMasterData) return;
    isRefreshingMasterData = true;
    notifyListeners();
    try {
      await refresh();
    } finally {
      isRefreshingMasterData = false;
      notifyListeners();
    }
  }

  void updateLiveConnectionState(LiveConnectionState value) {
    if (liveConnectionState == value) return;
    liveConnectionState = value;
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
    final timestamp = now ?? _clock();
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
    if (!draft.hasApiSafeAmounts) {
      throw SaleRuleException([
        const SaleRuleViolation(SaleRuleCode.amountOutsideApiRange),
      ]);
    }
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
    final completedAt = _clock().toUtc();
    SaleRules.enforceCompletion(
      draft: draft,
      isOnline: isOnline,
      offlinePolicy: offlinePolicy,
      currentDevice: device,
      evaluatedAt: completedAt,
    );

    var sale = CompletedSale(
      serverSaleId: '',
      receiptReference: isOnline ? 'Pending' : 'Offline pending',
      clientTransactionId: draft.clientTransactionId,
      deviceId: device.deviceId,
      customer: draft.customer,
      saleType: draft.saleType,
      paymentMethod: draft.paymentMethod,
      lines: List.unmodifiable(draft.lines),
      total: draft.total,
      createdAt: completedAt,
      paymentStatus: draft.saleType == SaleType.cash ? 'Paid' : 'Receivable',
      syncStatus: isOnline ? SyncStatus.syncing : SyncStatus.pendingSync,
      subtotalMinor: draft.subtotal,
      taxMinor: draft.tax,
      createdOffline: !isOnline,
    );

    if (isOnline) {
      final command = _syncCommand(sale, attempt: 1);
      await store.persistPendingSync(sale: sale, command: command);
      sale = await _syncExact(sale, command);
    } else {
      final command = _syncCommand(sale, attempt: 1);
      final allocation = _deviceAllocation;
      if (allocation == null) {
        await store.saveSale(sale);
        await store.enqueueSync(command);
        await _consumeOfflineAllocation(sale);
      } else {
        final quantities = _remainingQuantities(sale);
        final remainingDaily = offlinePolicy.remainingDailyValue - sale.total;
        final updated = DeviceAllocation(
          offlineEnabled: allocation.offlineEnabled,
          transactionLimitMinor: allocation.transactionLimitMinor,
          dailyValueLimitMinor: allocation.dailyValueLimitMinor,
          remainingDailyMinor: remainingDaily,
          offlineSalesValidUntil: allocation.offlineSalesValidUntil,
          productQuantities: Map.unmodifiable(quantities),
          updatedAt: DateTime.now().toUtc(),
        );
        await store.persistOfflineCompletion(
          sale: sale,
          command: command,
          allocation: updated,
        );
        _applyOfflineAllocation(updated);
      }
    }
    sales.insert(0, sale);
    await store.deleteDraft(draft.clientTransactionId);
    drafts.removeWhere(
      (item) => item.clientTransactionId == draft.clientTransactionId,
    );
    notifyListeners();
    return sale;
  }

  Future<void> _consumeOfflineAllocation(CompletedSale sale) async {
    final quantities = _remainingQuantities(sale);
    final remainingDaily = offlinePolicy.remainingDailyValue - sale.total;
    offlinePolicy = OfflineSalesPolicy(
      enabled: offlinePolicy.enabled,
      transactionValueLimit: offlinePolicy.transactionValueLimit,
      remainingDailyValue: remainingDaily,
      offlineSalesValidUntil: offlinePolicy.offlineSalesValidUntil,
      productAllocations: Map.unmodifiable(quantities),
    );
    final allocation = _deviceAllocation;
    if (allocation == null) return;
    final updated = DeviceAllocation(
      offlineEnabled: allocation.offlineEnabled,
      transactionLimitMinor: allocation.transactionLimitMinor,
      dailyValueLimitMinor: allocation.dailyValueLimitMinor,
      remainingDailyMinor: remainingDaily,
      offlineSalesValidUntil: allocation.offlineSalesValidUntil,
      productQuantities: Map.unmodifiable(quantities),
      updatedAt: DateTime.now().toUtc(),
    );
    _deviceAllocation = updated;
    await store.saveAllocation(updated);
  }

  Map<String, int> _remainingQuantities(CompletedSale sale) {
    final quantities = Map<String, int>.of(offlinePolicy.productAllocations);
    for (final line in sale.lines) {
      quantities[line.product.id] =
          (quantities[line.product.id] ?? 0) - line.quantity;
    }
    return quantities;
  }

  void _applyOfflineAllocation(DeviceAllocation allocation) {
    _deviceAllocation = allocation;
    offlinePolicy = OfflineSalesPolicy(
      enabled: allocation.offlineEnabled,
      transactionValueLimit: allocation.transactionLimitMinor,
      remainingDailyValue: allocation.remainingDailyMinor,
      offlineSalesValidUntil: allocation.offlineSalesValidUntil,
      productAllocations: allocation.productQuantities,
    );
  }

  void replaceDeviceAllocation(DeviceAllocation allocation) {
    _applyOfflineAllocation(allocation);
    notifyListeners();
  }

  Future<CompletedSale> _syncExact(
    CompletedSale sale,
    SyncCommand command,
  ) async {
    try {
      final result = await syncService.resolve(command);
      final authoritativeLines = <CartLine>[];
      for (final line in result.serverLines) {
        final local = sale.lines.firstWhere(
          (candidate) => candidate.product.id == line.productId,
        );
        authoritativeLines.add(
          CartLine(
            product: local.product,
            quantity: line.quantity,
            unitPrice: line.unitPriceMinor,
            authoritativeSubtotalMinor: line.subtotalMinor,
            authoritativeTaxMinor: line.taxMinor,
            authoritativeTotalMinor: line.totalMinor,
          ),
        );
      }
      final synced = sale.copyWith(
        serverSaleId: result.serverSaleId,
        receiptReference: result.receiptReference,
        fiscalStatus: result.fiscalStatus,
        total: result.serverTotalMinor,
        lines:
            authoritativeLines.isEmpty
                ? null
                : List.unmodifiable(authoritativeLines),
        subtotalMinor: result.serverSubtotalMinor,
        taxMinor: result.serverTaxMinor,
        cogsMinor: result.serverCogsMinor,
        syncStatus: SyncStatus.synced,
        syncMessage:
            result.wasDuplicate
                ? 'Original server result restored safely.'
                : 'Accepted by server.',
      );
      await store.finalizeSync(sale: synced, command: command, result: result);
      return synced;
    } on SyncFailure catch (failure) {
      final status = switch (failure.kind) {
        SyncFailureKind.retryable => SyncStatus.pendingSync,
        SyncFailureKind.ambiguous => SyncStatus.requiresReview,
        SyncFailureKind.terminal => SyncStatus.rejected,
        SyncFailureKind.authentication ||
        SyncFailureKind.suspended => SyncStatus.requiresReview,
      };
      final failed = sale.copyWith(
        syncStatus: status,
        syncMessage: failure.message,
      );
      await store.recordSyncFailure(
        sale: failed,
        idempotencyKey: command.idempotencyKey,
        authoritativeRejection: failure.kind == SyncFailureKind.terminal,
      );
      if (failure.kind == SyncFailureKind.retryable) {
        isOnline = false;
        liveConnectionState = LiveConnectionState.offlineCache;
      }
      return failed;
    } catch (_) {
      final failed = sale.copyWith(
        syncStatus: SyncStatus.requiresReview,
        syncMessage: 'Synchronization failed. Review this transaction.',
      );
      await store.recordSyncFailure(
        sale: failed,
        idempotencyKey: command.idempotencyKey,
        authoritativeRejection: false,
      );
      return failed;
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
      catalogSnapshotToken: device.catalogSnapshotToken,
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
    try {
      final queue = await store.readSyncQueue();
      for (final command in queue) {
        var index = sales.indexWhere(
          (sale) => sale.clientTransactionId == command.clientTransactionId,
        );
        if (index < 0) {
          sales.add(command.sale);
          index = sales.length - 1;
        }
        final sale = sales[index];
        sales[index] = sale.copyWith(syncStatus: SyncStatus.syncing);
        notifyListeners();
        sales[index] = await _syncExact(sale, command);
        if (!isOnline) break;
      }
      lastSyncAt = DateTime.now();
    } finally {
      isSyncing = false;
      notifyListeners();
    }
  }
}
