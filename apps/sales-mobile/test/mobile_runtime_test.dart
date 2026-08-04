import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:sales_mobile/application/mobile_runtime.dart';
import 'package:sales_mobile/data/http_api_transport.dart';
import 'package:sales_mobile/data/local_store.dart';
import 'package:sales_mobile/data/mobile_credential_store.dart';
import 'package:sales_mobile/domain/connection_models.dart';
import 'package:sales_mobile/domain/models.dart';

const actorId = '00000000-0000-4000-8000-000000000001';
const tenantId = '00000000-0000-4000-8000-000000000002';
const companyId = '00000000-0000-4000-8000-000000000003';
const branchId = '00000000-0000-4000-8000-000000000004';
const warehouseId = '00000000-0000-4000-8000-000000000005';
const otherWarehouseId = '00000000-0000-4000-8000-000000000099';
const deviceId = '00000000-0000-4000-8000-000000000006';
const customerId = '00000000-0000-4000-8000-000000000007';
const productId = '00000000-0000-4000-8000-000000000008';
const transactionId = '00000000-0000-4000-8000-000000000009';

void main() {
  test(
    'pending queue prevents enrollment from restoring quota or changing scope',
    () async {
      final store = InMemoryEncryptedLocalStore();
      final credentials = InMemoryMobileCredentialStore();
      await credentials.saveBearerToken('token');
      final connection = MobileConnection(
        baseUrl: Uri.parse('https://api.example.test'),
        identityMode: MobileIdentityMode.bearer,
      );
      final savedEnrollment = _enrollment(warehouseId, remaining: 10);
      final localAllocation = DeviceAllocation(
        offlineEnabled: true,
        transactionLimitMinor: 100000,
        dailyValueLimitMinor: 200000,
        remainingDailyMinor: 40000,
        offlineSalesValidUntil: DateTime.now().toUtc().add(
          const Duration(hours: 2),
        ),
        productQuantities: const {productId: 4},
        updatedAt: DateTime.utc(2026, 8, 4, 8),
      );
      await store.saveConnection(connection);
      await store.saveEnrollment(savedEnrollment);
      await store.saveAllocation(localAllocation);
      final command = _pendingCommand();
      await store.persistOfflineCompletion(
        sale: command.sale,
        command: command,
        allocation: localAllocation,
      );
      var responseWarehouse = warehouseId;
      final transport = InMemoryApiTransport((request) {
        switch (request.uri.path) {
          case '/v1/mobile/devices/enroll':
            return _json(
              200,
              _enrollmentJson(responseWarehouse, remaining: 10),
            );
          case '/v1/customers':
            return _json(200, _customerPage);
          case '/v1/products':
            return _json(200, _productPage);
          case '/v1/mobile/sync/sales':
            return _json(500, {
              'type': 'about:blank',
              'title': 'Temporary failure',
              'status': 500,
              'correlation_id': actorId,
            });
          default:
            throw StateError('Unexpected request: ${request.uri}');
        }
      });
      final runtime = MobileRuntimeController(
        store: store,
        credentials: credentials,
        transportFactory: () => transport,
      );
      addTearDown(runtime.close);

      await runtime.initialize();

      expect(runtime.state, LiveConnectionState.ready);
      expect((await store.readAllocation())?.remainingDailyMinor, 40000);
      expect((await store.readAllocation())?.productQuantities[productId], 4);
      expect(await store.readSyncQueue(), hasLength(1));

      await runtime.enroll(
        baseUrl: connection.baseUrl,
        deviceName: 'Counter 1',
        identityMode: MobileIdentityMode.bearer,
        bearerToken: 'refreshed-token',
      );
      expect((await store.readAllocation())?.remainingDailyMinor, 40000);
      expect((await store.readAllocation())?.productQuantities[productId], 4);

      responseWarehouse = otherWarehouseId;
      await runtime.enroll(
        baseUrl: connection.baseUrl,
        deviceName: 'Counter 1',
        identityMode: MobileIdentityMode.bearer,
        bearerToken: 'refreshed-token',
      );
      expect(runtime.state, LiveConnectionState.error);
      expect((await store.readEnrollment())?.scope.warehouseId, warehouseId);
      expect(await store.readSyncQueue(), hasLength(1));
    },
  );

  test('lost first enrollment response reuses the durable device ID', () async {
    final store = InMemoryEncryptedLocalStore();
    final credentials = InMemoryMobileCredentialStore();
    final observedDeviceIds = <String>[];

    MobileRuntimeController runtime() => MobileRuntimeController(
      store: store,
      credentials: credentials,
      transportFactory:
          () => InMemoryApiTransport((request) {
            final body = jsonDecode(request.body!) as Map<String, Object?>;
            observedDeviceIds.add(body['device_id']! as String);
            throw const ApiTransportException();
          }),
    );

    final first = runtime();
    await first.initialize();
    await first.enroll(
      baseUrl: Uri.parse('https://api.example.test'),
      deviceName: 'Counter 1',
      identityMode: MobileIdentityMode.bearer,
      bearerToken: 'first-token',
    );
    await first.close();

    final second = runtime();
    await second.initialize();
    await second.enroll(
      baseUrl: Uri.parse('https://api.example.test'),
      deviceName: 'Counter 1',
      identityMode: MobileIdentityMode.bearer,
      bearerToken: 'second-token',
    );
    await second.close();

    expect(observedDeviceIds, hasLength(2));
    expect(observedDeviceIds.first, observedDeviceIds.last);
    expect(await store.readCandidateDeviceId(), observedDeviceIds.first);
  });

  test(
    'new available versions plus failed download disable offline cache',
    () async {
      final store = InMemoryEncryptedLocalStore();
      final credentials = InMemoryMobileCredentialStore();
      await credentials.saveBearerToken('token');
      final connection = MobileConnection(
        baseUrl: Uri.parse('https://api.example.test'),
        identityMode: MobileIdentityMode.bearer,
      );
      final installed = _enrollment(warehouseId, remaining: 10);
      await store.saveConnection(connection);
      await store.installMasterData(
        customers: const [_generalCustomer],
        products: const [_cachedProduct],
        masterDataVersion: 1,
        priceVersion: 1,
        catalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
        enrollment: installed,
      );
      final runtime = MobileRuntimeController(
        store: store,
        credentials: credentials,
        transportFactory:
            () => InMemoryApiTransport((request) {
              switch (request.uri.path) {
                case '/v1/mobile/devices/enroll':
                  return _json(
                    200,
                    _enrollmentJson(
                      warehouseId,
                      remaining: 10,
                      availableMasterDataVersion: 2,
                      availablePriceVersion: 2,
                    ),
                  );
                case '/v1/customers':
                  return _json(200, _customerPage);
                case '/v1/products':
                  return _json(500, {
                    'type': 'about:blank',
                    'title': 'Temporary failure',
                    'status': 500,
                    'correlation_id': actorId,
                  });
                default:
                  throw StateError('Unexpected request: ${request.uri}');
              }
            }),
      );
      addTearDown(runtime.close);

      await runtime.initialize();

      expect(runtime.state, LiveConnectionState.error);
      expect(runtime.salesController, isNull);
      final saved = await store.readEnrollment();
      expect(saved?.masterDataVersion, 1);
      expect(saved?.availableMasterDataVersion, 2);
      final cached = await store.readInstalledMasterDataVersions();
      expect(cached?.masterDataVersion, 1);
      expect(cached?.priceVersion, 1);
    },
  );

  test(
    'lost app-version enrollment response cannot reopen an old offline cache',
    () async {
      final store = InMemoryEncryptedLocalStore();
      final credentials = InMemoryMobileCredentialStore();
      await credentials.saveBearerToken('token');
      final connection = MobileConnection(
        baseUrl: Uri.parse('https://api.example.test'),
        identityMode: MobileIdentityMode.bearer,
      );
      final installed = _enrollment(
        warehouseId,
        remaining: 10,
        appVersion: '0.9.0+9',
      );
      await store.saveConnection(connection);
      await store.installMasterData(
        customers: const [_generalCustomer],
        products: const [_cachedProduct],
        masterDataVersion: 1,
        priceVersion: 1,
        catalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
        enrollment: installed,
      );
      var enrollmentAttempts = 0;
      final runtime = MobileRuntimeController(
        store: store,
        credentials: credentials,
        transportFactory:
            () => InMemoryApiTransport((request) {
              if (request.uri.path == '/v1/mobile/devices/enroll') {
                enrollmentAttempts += 1;
              }
              throw const ApiTransportException();
            }),
      );
      addTearDown(runtime.close);

      await runtime.initialize();

      expect(enrollmentAttempts, 1);
      expect(runtime.state, LiveConnectionState.error);
      expect(runtime.salesController, isNull);
      expect((await store.readEnrollment())?.appVersion, '0.9.0+9');
      expect(await store.readSyncQueue(), isEmpty);
    },
  );

  test(
    'install acknowledgement keeps cache and sync versions coherent',
    () async {
      final store = InMemoryEncryptedLocalStore();
      final credentials = InMemoryMobileCredentialStore();
      await credentials.saveBearerToken('token');
      final connection = MobileConnection(
        baseUrl: Uri.parse('https://api.example.test'),
        identityMode: MobileIdentityMode.bearer,
      );
      final installed = _enrollment(warehouseId, remaining: 10);
      await store.saveConnection(connection);
      await store.installMasterData(
        customers: const [_generalCustomer],
        products: const [_cachedProduct],
        masterDataVersion: 1,
        priceVersion: 1,
        catalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
        enrollment: installed,
      );
      Map<String, Object?>? syncedCommand;
      final runtime = MobileRuntimeController(
        store: store,
        credentials: credentials,
        transportFactory:
            () => InMemoryApiTransport((request) {
              if (request.uri.path == '/v1/mobile/devices/enroll') {
                final body = jsonDecode(request.body!) as Map<String, Object?>;
                final acknowledged = body['installed_master_data_version'] == 2;
                return _json(
                  200,
                  _enrollmentJson(
                    warehouseId,
                    remaining: 10,
                    installedMasterDataVersion: acknowledged ? 2 : 1,
                    installedPriceVersion: acknowledged ? 2 : 1,
                    availableMasterDataVersion: 2,
                    availablePriceVersion: 2,
                  ),
                );
              }
              if (request.uri.path == '/v1/customers') {
                return _json(200, _customerPageForVersions(2, 2));
              }
              if (request.uri.path == '/v1/products') {
                return _json(200, _productPageForVersions(2, 2));
              }
              if (request.uri.path == '/v1/mobile/sync/sales') {
                syncedCommand =
                    jsonDecode(request.body!) as Map<String, Object?>;
                return _json(200, _syncResult(syncedCommand!));
              }
              throw StateError('Unexpected request: ${request.uri}');
            }),
      );
      addTearDown(runtime.close);

      await runtime.initialize();
      expect(runtime.state, LiveConnectionState.ready);
      expect(runtime.salesController?.device.masterDataVersion, 2);
      expect(runtime.salesController?.device.priceVersion, 2);
      expect((await store.readEnrollment())?.masterDataVersion, 2);
      expect((await store.readInstalledMasterDataVersions())?.priceVersion, 2);

      final controller = runtime.salesController!;
      final draft =
          controller.createNewSale()..addProduct(controller.products.single);
      final sale = await controller.completeSale(draft);

      expect(sale.syncStatus, SyncStatus.synced);
      expect(syncedCommand?['master_data_version'], 2);
      expect(syncedCommand?['price_version'], 2);
    },
  );

  test(
    'existing-device app upgrade uses the acknowledged build in sales',
    () async {
      final store = InMemoryEncryptedLocalStore();
      final credentials = InMemoryMobileCredentialStore();
      await credentials.saveBearerToken('token');
      final connection = MobileConnection(
        baseUrl: Uri.parse('https://api.example.test'),
        identityMode: MobileIdentityMode.bearer,
      );
      final installed = _enrollment(
        warehouseId,
        remaining: 10,
        appVersion: '0.9.0+9',
      );
      await store.saveConnection(connection);
      await store.installMasterData(
        customers: const [_generalCustomer],
        products: const [_cachedProduct],
        masterDataVersion: 1,
        priceVersion: 1,
        catalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
        enrollment: installed,
      );
      Map<String, Object?>? syncedCommand;
      final runtime = MobileRuntimeController(
        store: store,
        credentials: credentials,
        transportFactory:
            () => InMemoryApiTransport((request) {
              if (request.uri.path == '/v1/mobile/devices/enroll') {
                final body = jsonDecode(request.body!) as Map<String, Object?>;
                final acknowledged = body.containsKey(
                  'installed_master_data_version',
                );
                return _json(
                  200,
                  _enrollmentJson(
                    warehouseId,
                    remaining: 10,
                    appVersion:
                        acknowledged ? salesMobileAppVersion : '0.9.0+9',
                    offlineSalesValidUntil:
                        acknowledged
                            ? DateTime.now()
                                .toUtc()
                                .add(const Duration(hours: 2))
                                .toIso8601String()
                            : DateTime.fromMillisecondsSinceEpoch(
                              0,
                              isUtc: true,
                            ).toIso8601String(),
                  ),
                );
              }
              if (request.uri.path == '/v1/customers') {
                return _json(200, _customerPage);
              }
              if (request.uri.path == '/v1/products') {
                return _json(200, _productPage);
              }
              if (request.uri.path == '/v1/mobile/sync/sales') {
                syncedCommand =
                    jsonDecode(request.body!) as Map<String, Object?>;
                return _json(200, _syncResult(syncedCommand!));
              }
              throw StateError('Unexpected request: ${request.uri}');
            }),
      );
      addTearDown(runtime.close);

      await runtime.initialize();

      expect(runtime.state, LiveConnectionState.ready);
      expect(runtime.salesController?.device.appVersion, salesMobileAppVersion);
      expect((await store.readEnrollment())?.appVersion, salesMobileAppVersion);
      final controller = runtime.salesController!;
      final draft =
          controller.createNewSale()..addProduct(controller.products.single);
      await controller.completeSale(draft);
      expect(syncedCommand?['app_version'], salesMobileAppVersion);
    },
  );

  test(
    'lost install acknowledgement is recovered safely after restart',
    () async {
      final store = InMemoryEncryptedLocalStore();
      final credentials = InMemoryMobileCredentialStore();
      await credentials.saveBearerToken('token');
      final connection = MobileConnection(
        baseUrl: Uri.parse('https://api.example.test'),
        identityMode: MobileIdentityMode.bearer,
      );
      final installed = _enrollment(warehouseId, remaining: 10);
      await store.saveConnection(connection);
      await store.installMasterData(
        customers: const [_generalCustomer],
        products: const [_cachedProduct],
        masterDataVersion: 1,
        priceVersion: 1,
        catalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
        enrollment: installed,
      );
      var serverInstalledVersion = 1;
      var loseAcknowledgement = true;

      MobileRuntimeController runtime() => MobileRuntimeController(
        store: store,
        credentials: credentials,
        transportFactory:
            () => InMemoryApiTransport((request) {
              if (request.uri.path == '/v1/mobile/devices/enroll') {
                final body = jsonDecode(request.body!) as Map<String, Object?>;
                if (body['installed_master_data_version'] == 2) {
                  serverInstalledVersion = 2;
                  if (loseAcknowledgement) {
                    loseAcknowledgement = false;
                    throw const ApiTransportException();
                  }
                }
                return _json(
                  200,
                  _enrollmentJson(
                    warehouseId,
                    remaining: 10,
                    installedMasterDataVersion: serverInstalledVersion,
                    installedPriceVersion: serverInstalledVersion,
                    availableMasterDataVersion: 2,
                    availablePriceVersion: 2,
                  ),
                );
              }
              if (request.uri.path == '/v1/customers') {
                return _json(200, _customerPageForVersions(2, 2));
              }
              if (request.uri.path == '/v1/products') {
                return _json(200, _productPageForVersions(2, 2));
              }
              throw StateError('Unexpected request: ${request.uri}');
            }),
      );

      final first = runtime();
      await first.initialize();
      expect(first.state, LiveConnectionState.error);
      expect((await store.readEnrollment())?.status, 'INSTALL_ACK_PENDING');
      expect((await store.readInstalledMasterDataVersions())?.priceVersion, 2);
      await first.close();

      final restarted = runtime();
      await restarted.initialize();
      addTearDown(restarted.close);

      expect(restarted.state, LiveConnectionState.ready);
      expect((await store.readEnrollment())?.status, 'ACTIVE');
      expect((await store.readEnrollment())?.masterDataVersion, 2);
      expect(restarted.salesController?.device.masterDataVersion, 2);
    },
  );

  test('development device override is ignored outside debug headers', () {
    const configured = '00000000-0000-4000-8000-000000000011';
    expect(
      selectDevelopmentDeviceId(
        isRelease: false,
        identityMode: MobileIdentityMode.developmentHeaders,
        configuredValue: configured,
      ),
      configured,
    );
    expect(
      selectDevelopmentDeviceId(
        isRelease: true,
        identityMode: MobileIdentityMode.developmentHeaders,
        configuredValue: configured,
      ),
      isNull,
    );
    expect(
      selectDevelopmentDeviceId(
        isRelease: false,
        identityMode: MobileIdentityMode.bearer,
        configuredValue: configured,
      ),
      isNull,
    );
  });
}

DeviceEnrollment _enrollment(
  String warehouse, {
  required int remaining,
  String appVersion = salesMobileAppVersion,
}) => DeviceEnrollment(
  deviceId: deviceId,
  deviceName: 'Counter 1',
  status: 'ACTIVE',
  actorId: actorId,
  scope: EnrollmentScope(
    tenantId: tenantId,
    companyId: companyId,
    branchId: branchId,
    warehouseId: warehouse,
  ),
  appVersion: appVersion,
  catalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
  masterDataVersion: 1,
  priceVersion: 1,
  availableMasterDataVersion: 1,
  availablePriceVersion: 1,
  availableCatalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
  timezone: 'Africa/Dar_es_Salaam',
  offlineEnabled: true,
  transactionValueLimitMinor: 100000,
  dailyValueLimitMinor: 200000,
  remainingDailyValueMinor: 100000,
  offlineSalesValidUntil: DateTime.now().toUtc().add(const Duration(hours: 2)),
  stockAllocations: [
    MobileStockAllocation(
      productId: productId,
      allocatedQuantity: 10,
      remainingQuantity: remaining,
    ),
  ],
  enrolledAt: DateTime.utc(2026, 8, 4, 7),
  lastSeenAt: DateTime.utc(2026, 8, 4, 8),
);

Map<String, Object?> _enrollmentJson(
  String warehouse, {
  required int remaining,
  String appVersion = salesMobileAppVersion,
  String? offlineSalesValidUntil,
  int installedMasterDataVersion = 1,
  int installedPriceVersion = 1,
  int availableMasterDataVersion = 1,
  int availablePriceVersion = 1,
}) => {
  'device_id': deviceId,
  'device_name': 'Counter 1',
  'status': 'ACTIVE',
  'actor_id': actorId,
  'scope': {
    'tenant_id': tenantId,
    'company_id': companyId,
    'branch_id': branchId,
    'warehouse_id': warehouse,
  },
  'app_version': appVersion,
  'catalog_snapshot_token':
      installedMasterDataVersion > 0
          ? '00000000-0000-4000-8000-000000000010'
          : unacknowledgedCatalogSnapshotToken,
  'master_data_version': installedMasterDataVersion,
  'price_version': installedPriceVersion,
  'available_master_data_version': availableMasterDataVersion,
  'available_price_version': availablePriceVersion,
  'available_catalog_snapshot_token': '00000000-0000-4000-8000-000000000010',
  'timezone': 'Africa/Dar_es_Salaam',
  'offline_enabled': true,
  'transaction_value_limit_minor': 100000,
  'daily_value_limit_minor': 200000,
  'remaining_daily_value_minor': 100000,
  'offline_sales_valid_until':
      offlineSalesValidUntil ??
      DateTime.now().toUtc().add(const Duration(hours: 2)).toIso8601String(),
  'stock_allocations': [
    {
      'product_id': productId,
      'allocated_quantity': 10,
      'remaining_quantity': remaining,
    },
  ],
  'enrolled_at': '2026-08-04T07:00:00Z',
  'last_seen_at': DateTime.now().toUtc().toIso8601String(),
};

Map<String, Object?> get _customerPage => {
  'catalog_snapshot_token': '00000000-0000-4000-8000-000000000010',
  'master_data_version': 1,
  'price_version': 1,
  'items': [
    {
      'id': customerId,
      'code': 'GENERAL',
      'name': 'General Customer',
      'status': 'active',
      'is_general_customer': true,
      'credit_enabled': false,
      'credit_limit_minor': 0,
      'current_exposure_minor': 0,
      'available_credit_minor': 0,
    },
  ],
  'next_cursor': null,
};

Map<String, Object?> _customerPageForVersions(int master, int price) => {
  ..._customerPage,
  'master_data_version': master,
  'price_version': price,
};

Map<String, Object?> get _productPage => {
  'catalog_snapshot_token': '00000000-0000-4000-8000-000000000010',
  'master_data_version': 1,
  'price_version': 1,
  'items': [
    {
      'id': productId,
      'code': 'P-1',
      'name': 'Cement',
      'unit': 'Bag',
      'currency': 'TZS',
      'unit_price_minor': 10000,
      'available_quantity': 100,
      'tax_basis_points': 0,
      'master_data_version': 1,
      'price_version': 1,
    },
  ],
  'next_cursor': null,
};

Map<String, Object?> _productPageForVersions(int master, int price) => {
  'catalog_snapshot_token': '00000000-0000-4000-8000-000000000010',
  'master_data_version': master,
  'price_version': price,
  'items': [
    {
      'id': productId,
      'code': 'P-1',
      'name': 'Cement',
      'unit': 'Bag',
      'currency': 'TZS',
      'unit_price_minor': 10000,
      'available_quantity': 100,
      'tax_basis_points': 0,
      'master_data_version': master,
      'price_version': price,
    },
  ],
  'next_cursor': null,
};

const _generalCustomer = Customer(
  id: customerId,
  name: 'General Customer',
  code: 'GENERAL',
  isGeneral: true,
  isActive: true,
  creditEnabled: false,
  inAttendantScope: true,
);

const _cachedProduct = Product(
  id: productId,
  code: 'P-1',
  name: 'Cement',
  commonDescription: '',
  brand: '',
  category: '',
  unit: 'Bag',
  sellingPrice: 10000,
  availableQuantity: 100,
  taxBasisPoints: 0,
);

Map<String, Object?> _syncResult(Map<String, Object?> command) {
  final clientTransactionId = command['client_transaction_id']! as String;
  final lines = command['lines']! as List<Object?>;
  final line = lines.single! as Map<String, Object?>;
  final quantity = line['quantity']! as int;
  final subtotal = 10000 * quantity;
  return {
    'client_transaction_id': clientTransactionId,
    'state': 'synced',
    'receipt_reference': 'SALE-COHERENT',
    'fiscal_status': 'NOT_CONFIGURED',
    'idempotent_replay': false,
    'sale': {
      'id': '00000000-0000-4000-8000-000000000012',
      'customer_id': command['customer_id'],
      'currency': 'TZS',
      'subtotal_minor': subtotal,
      'tax_minor': 0,
      'total_minor': subtotal,
      'receipt_reference': 'SALE-COHERENT',
      'fiscal_status': 'NOT_CONFIGURED',
      'lines': [
        {
          'id': '00000000-0000-4000-8000-000000000013',
          'product_id': line['product_id'],
          'quantity': quantity,
          'unit_price_minor': 10000,
          'subtotal_minor': subtotal,
          'tax_minor': 0,
          'total_minor': subtotal,
        },
      ],
    },
  };
}

SyncCommand _pendingCommand() {
  const customer = Customer(
    id: customerId,
    name: 'General Customer',
    code: 'GENERAL',
    isGeneral: true,
    isActive: true,
    creditEnabled: false,
    inAttendantScope: true,
  );
  const product = Product(
    id: productId,
    code: 'P-1',
    name: 'Cement',
    commonDescription: '',
    brand: '',
    category: '',
    unit: 'Bag',
    sellingPrice: 10000,
    availableQuantity: 100,
    taxBasisPoints: 0,
  );
  final sale = CompletedSale(
    serverSaleId: '',
    receiptReference: 'Offline pending',
    clientTransactionId: transactionId,
    deviceId: deviceId,
    customer: customer,
    saleType: SaleType.cash,
    paymentMethod: PaymentMethod.cash,
    lines: [CartLine(product: product, quantity: 6)],
    total: 60000,
    createdAt: DateTime.utc(2026, 8, 4, 8),
    paymentStatus: 'Paid',
    syncStatus: SyncStatus.pendingSync,
    createdOffline: true,
  );
  return SyncCommand(
    deviceId: deviceId,
    userId: actorId,
    clientTransactionId: transactionId,
    clientTimestamp: sale.createdAt,
    companyId: companyId,
    branchId: branchId,
    warehouseId: warehouseId,
    appVersion: '1.0.0+1',
    catalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
    masterDataVersion: 1,
    priceVersion: 1,
    syncAttemptNumber: 1,
    sale: sale,
  );
}

ApiResponse _json(int status, Map<String, Object?> body) => ApiResponse(
  statusCode: status,
  headers: const {'content-type': 'application/json'},
  body: jsonEncode(body),
);
