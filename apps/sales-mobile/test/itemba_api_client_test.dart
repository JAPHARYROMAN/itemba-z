import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:sales_mobile/data/http_api_transport.dart';
import 'package:sales_mobile/data/http_sync_gateway.dart';
import 'package:sales_mobile/data/itemba_api_client.dart';
import 'package:sales_mobile/data/mobile_credential_store.dart';
import 'package:sales_mobile/data/sync_service.dart';
import 'package:sales_mobile/domain/connection_models.dart';
import 'package:sales_mobile/domain/models.dart';

const actorId = '00000000-0000-4000-8000-000000000001';
const tenantId = '00000000-0000-4000-8000-000000000002';
const companyId = '00000000-0000-4000-8000-000000000003';
const branchId = '00000000-0000-4000-8000-000000000004';
const warehouseId = '00000000-0000-4000-8000-000000000005';
const deviceId = '00000000-0000-4000-8000-000000000006';
const customerId = '00000000-0000-4000-8000-000000000007';
const productId = '00000000-0000-4000-8000-000000000008';
const transactionId = '00000000-0000-4000-8000-000000000009';
const saleId = '00000000-0000-4000-8000-000000000010';

void main() {
  test('credential boundary persists and deletes bearer token', () async {
    final credentials = InMemoryMobileCredentialStore();
    await credentials.saveBearerToken('secret-token');
    expect(await credentials.readBearerToken(), 'secret-token');
    await credentials.deleteBearerToken();
    expect(await credentials.readBearerToken(), isNull);
  });

  test(
    'enrollment sends bearer and correlation headers with contract body',
    () async {
      late ApiRequest captured;
      final transport = InMemoryApiTransport((request) {
        captured = request;
        return _json(200, _enrollmentJson);
      });
      final credentials = InMemoryMobileCredentialStore();
      await credentials.saveBearerToken('secret-token');
      final api = ItembaApiClient(
        connection: _bearerConnection,
        credentials: credentials,
        transport: transport,
      );

      final enrollment = await api.enrollDevice(
        deviceId: deviceId,
        deviceName: 'Counter 1',
        appVersion: '1.0.0+1',
      );

      expect(captured.uri.path, '/v1/mobile/devices/enroll');
      expect(captured.headers['Authorization'], 'Bearer secret-token');
      expect(captured.headers['X-Correlation-ID'], matches(_uuidPattern));
      expect(captured.headers, isNot(contains('X-Actor-ID')));
      expect(jsonDecode(captured.body!), {
        'device_id': deviceId,
        'device_name': 'Counter 1',
        'app_version': '1.0.0+1',
      });
      expect(enrollment.scope.tenantId, tenantId);
      expect(enrollment.masterDataVersion, 4);
    },
  );

  test('enrollment acknowledgement sends both installed versions', () async {
    late ApiRequest captured;
    final api = await _bearerApi(
      InMemoryApiTransport((request) {
        captured = request;
        return _json(200, _enrollmentJson);
      }),
    );

    await api.enrollDevice(
      deviceId: deviceId,
      deviceName: 'Counter 1',
      appVersion: '1.0.0+1',
      installedMasterDataVersion: 4,
      installedPriceVersion: 8,
      installedCatalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
    );

    final body = jsonDecode(captured.body!) as Map<String, Object?>;
    expect(body['installed_master_data_version'], 4);
    expect(body['installed_price_version'], 8);
  });

  test('enrollment rejects an offline lease longer than four hours', () async {
    final response = Map<String, Object?>.of(_enrollmentJson)
      ..['offline_sales_valid_until'] = '2026-08-04T13:00:00Z';
    final api = await _bearerApi(
      InMemoryApiTransport((_) => _json(200, response)),
    );

    await expectLater(
      api.enrollDevice(
        deviceId: deviceId,
        deviceName: 'Counter 1',
        appVersion: '1.0.0+1',
      ),
      throwsA(
        isA<ApiException>().having(
          (error) => error.kind,
          'kind',
          ApiFailureKind.invalidResponse,
        ),
      ),
    );
  });

  test('development identity sends exactly the five scoped headers', () async {
    final transport = InMemoryApiTransport((request) {
      if (request.uri.path == '/v1/customers') {
        return _json(200, {
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
            {
              'id': '00000000-0000-4000-8000-000000000011',
              'code': 'C-1',
              'name': 'Credit Customer',
              'status': 'active',
              'is_general_customer': false,
              'credit_enabled': true,
              'credit_limit_minor': 100000,
              'current_exposure_minor': 25000,
              'available_credit_minor': 75000,
            },
          ],
          'next_cursor': null,
        });
      }
      return _json(200, {
        'items': [
          {
            'id': productId,
            'code': 'P-1',
            'name': 'Cement',
            'unit': 'Bag',
            'currency': 'TZS',
            'unit_price_minor': 18500,
            'available_quantity': 12,
            'tax_basis_points': 1800,
            'master_data_version': 4,
            'price_version': 8,
          },
        ],
        'next_cursor': null,
      });
    });
    final api = ItembaApiClient(
      connection: MobileConnection(
        baseUrl: Uri.parse('http://10.0.2.2:8080'),
        identityMode: MobileIdentityMode.developmentHeaders,
        developmentIdentity: DevelopmentIdentity(
          actorId: actorId,
          tenantId: tenantId,
          companyId: companyId,
          branchId: branchId,
          warehouseId: warehouseId,
        ),
      ),
      credentials: InMemoryMobileCredentialStore(),
      transport: transport,
      developmentIdentityAllowed: true,
    );

    final snapshot = await api.refreshMasterData(
      catalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
    );

    expect(snapshot.products.single.sellingPrice, 18500);
    expect(snapshot.products.single.availableQuantity, 12);
    expect(snapshot.products.single.taxBasisPoints, 1800);
    expect(snapshot.masterDataVersion, 4);
    expect(snapshot.priceVersion, 8);
    expect(
      snapshot.customers
          .firstWhere((customer) => customer.code == 'C-1')
          .credit,
      isNull,
      reason: 'overdue policy is absent, so mobile credit fails closed',
    );
    for (final request in transport.requests) {
      expect(request.headers['X-Actor-ID'], actorId);
      expect(request.headers['X-Tenant-ID'], tenantId);
      expect(request.headers['X-Company-ID'], companyId);
      expect(request.headers['X-Branch-ID'], branchId);
      expect(request.headers['X-Warehouse-ID'], warehouseId);
      expect(request.headers, isNot(contains('Authorization')));
    }
  });

  test('retry preserves the exact command; replay comes from server', () async {
    var calls = 0;
    final transport = InMemoryApiTransport((request) {
      calls += 1;
      if (calls == 1) {
        return _json(500, {
          'type': 'about:blank',
          'title': 'Temporary failure',
          'status': 500,
          'correlation_id': actorId,
        });
      }
      return _json(200, _syncResultJson(idempotentReplay: true));
    });
    final gateway = HttpAuthoritativeSyncGateway(
      api: await _bearerApi(transport),
      delay: (_) async {},
    );

    final result = await gateway.submit(_command());

    expect(result.wasDuplicate, isTrue);
    expect(result.receiptReference, 'SALE-$saleId');
    expect(result.fiscalStatus, FiscalStatus.pending);
    expect(result.serverTotalMinor, 23600);
    expect(result.serverLines.single.unitPriceMinor, 20000);
    expect(result.serverLines.single.taxMinor, 3600);
    expect(transport.requests, hasLength(2));
    final first =
        jsonDecode(transport.requests[0].body!) as Map<String, Object?>;
    final second =
        jsonDecode(transport.requests[1].body!) as Map<String, Object?>;
    expect(first['client_transaction_id'], transactionId);
    expect(second['client_transaction_id'], transactionId);
    expect(first['sync_attempt'], 1);
    expect(second['sync_attempt'], 1);
    expect(second, first);
    expect(transport.requests[0].headers, isNot(contains('Idempotency-Key')));
  });

  test('terminal validation failure is not retried', () async {
    final transport = InMemoryApiTransport(
      (_) => _json(422, {
        'type': 'about:blank',
        'title': 'Validation failed',
        'detail': 'Allocation is unavailable.',
        'status': 422,
        'code': 'ALLOCATION_UNAVAILABLE',
        'correlation_id': actorId,
      }),
    );
    final gateway = HttpAuthoritativeSyncGateway(
      api: await _bearerApi(transport),
      delay: (_) async {},
    );

    await expectLater(
      gateway.submit(_command()),
      throwsA(
        isA<SyncFailure>()
            .having((failure) => failure.kind, 'kind', SyncFailureKind.terminal)
            .having(
              (failure) => failure.code,
              'code',
              'ALLOCATION_UNAVAILABLE',
            ),
      ),
    );
    expect(transport.requests, hasLength(1));
  });

  test(
    'catalog evidence gap is classified for governed reconciliation',
    () async {
      final transport = InMemoryApiTransport(
        (_) => _json(409, {
          'type': 'about:blank',
          'title': 'Conflict',
          'detail': 'Retain the exact command for governed reconciliation.',
          'status': 409,
          'code': 'offline_reconciliation_required',
          'correlation_id': actorId,
        }),
      );
      final gateway = HttpAuthoritativeSyncGateway(
        api: await _bearerApi(transport),
        delay: (_) async {},
      );

      await expectLater(
        gateway.submit(_command()),
        throwsA(
          isA<SyncFailure>()
              .having(
                (failure) => failure.kind,
                'kind',
                SyncFailureKind.reconciliation,
              )
              .having(
                (failure) => failure.code,
                'code',
                'offline_reconciliation_required',
              ),
        ),
      );
      expect(transport.requests, hasLength(1));
    },
  );

  test('corrupt success response is ambiguous, never terminal', () async {
    final gateway = HttpAuthoritativeSyncGateway(
      api: await _bearerApi(
        InMemoryApiTransport(
          (_) => const ApiResponse(
            statusCode: 200,
            headers: {'content-type': 'application/json'},
            body: '{"state":"synced",',
          ),
        ),
      ),
      delay: (_) async {},
    );

    await expectLater(
      gateway.submit(_command()),
      throwsA(
        isA<SyncFailure>().having(
          (failure) => failure.kind,
          'kind',
          SyncFailureKind.ambiguous,
        ),
      ),
    );
  });

  test('malformed 4xx is ambiguous, not an authoritative rejection', () async {
    final gateway = HttpAuthoritativeSyncGateway(
      api: await _bearerApi(
        InMemoryApiTransport(
          (_) => const ApiResponse(
            statusCode: 422,
            headers: {'content-type': 'application/problem+json'},
            body: '{"title":"Unprocessable Entity"}',
          ),
        ),
      ),
      delay: (_) async {},
    );

    await expectLater(
      gateway.submit(_command()),
      throwsA(
        isA<SyncFailure>().having(
          (failure) => failure.kind,
          'kind',
          SyncFailureKind.ambiguous,
        ),
      ),
    );
  });

  test('production bearer connections reject cleartext HTTP', () {
    expect(
      () => ItembaApiClient(
        connection: MobileConnection(
          baseUrl: Uri.parse('http://api.example.test'),
          identityMode: MobileIdentityMode.bearer,
        ),
        credentials: InMemoryMobileCredentialStore(),
        transport: InMemoryApiTransport((_) => _json(200, const {})),
      ),
      throwsFormatException,
    );
  });

  test('malformed JSON is classified as an invalid API response', () async {
    final credentials = InMemoryMobileCredentialStore();
    await credentials.saveBearerToken('secret-token');
    final api = ItembaApiClient(
      connection: _bearerConnection,
      credentials: credentials,
      transport: InMemoryApiTransport(
        (_) =>
            const ApiResponse(statusCode: 200, headers: {}, body: '{not-json'),
      ),
    );

    await expectLater(
      api.enrollDevice(
        deviceId: deviceId,
        deviceName: 'Counter 1',
        appVersion: '1.0.0+1',
      ),
      throwsA(
        isA<ApiException>().having(
          (error) => error.kind,
          'kind',
          ApiFailureKind.invalidResponse,
        ),
      ),
    );
  });

  test('malformed pagination cursor fails closed', () async {
    final transport = InMemoryApiTransport(
      (_) => _json(200, {'items': <Object?>[], 'next_cursor': 42}),
    );
    final api = await _bearerApi(transport);

    await expectLater(
      api.refreshMasterData(
        catalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
      ),
      throwsA(
        isA<ApiException>().having(
          (error) => error.kind,
          'kind',
          ApiFailureKind.invalidResponse,
        ),
      ),
    );
  });

  test(
    'catalog download fails closed when a page returns another token',
    () async {
      final transport = InMemoryApiTransport(
        (_) => _json(200, {
          'items': <Object?>[],
          'next_cursor': null,
          'catalog_snapshot_token': '00000000-0000-4000-8000-000000000011',
          'master_data_version': 4,
          'price_version': 8,
        }),
      );
      final api = await _bearerApi(transport);

      await expectLater(
        api.refreshMasterData(
          catalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
        ),
        throwsA(
          isA<ApiException>().having(
            (error) => error.kind,
            'kind',
            ApiFailureKind.invalidResponse,
          ),
        ),
      );
      expect(transport.requests, hasLength(1));
    },
  );

  test(
    'server integers outside the JavaScript-safe range fail closed',
    () async {
      final transport = InMemoryApiTransport((request) {
        if (request.uri.path == '/v1/customers') {
          return _json(200, {
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
          });
        }
        return _json(200, {
          'items': [
            {
              'id': productId,
              'code': 'P-1',
              'name': 'Cement',
              'unit': 'Bag',
              'currency': 'TZS',
              'unit_price_minor': maximumSafeApiInteger + 1,
              'available_quantity': 1,
              'tax_basis_points': 0,
              'master_data_version': 1,
              'price_version': 1,
            },
          ],
          'next_cursor': null,
        });
      });
      final api = await _bearerApi(transport);

      await expectLater(
        api.refreshMasterData(
          catalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
        ),
        throwsA(
          isA<ApiException>().having(
            (error) => error.kind,
            'kind',
            ApiFailureKind.invalidResponse,
          ),
        ),
      );
    },
  );

  test('development identity requires an explicit non-release override', () {
    expect(
      () => ItembaApiClient(
        connection: MobileConnection(
          baseUrl: Uri.parse('http://127.0.0.1'),
          identityMode: MobileIdentityMode.developmentHeaders,
          developmentIdentity: DevelopmentIdentity(
            actorId: actorId,
            tenantId: tenantId,
            companyId: companyId,
            branchId: branchId,
            warehouseId: warehouseId,
          ),
        ),
        credentials: InMemoryMobileCredentialStore(),
        transport: InMemoryApiTransport((_) => _json(200, const {})),
        developmentIdentityAllowed: false,
      ),
      throwsFormatException,
    );
  });
}

Future<ItembaApiClient> _bearerApi(ApiTransport transport) async {
  final credentials = InMemoryMobileCredentialStore();
  await credentials.saveBearerToken('secret-token');
  return ItembaApiClient(
    connection: _bearerConnection,
    credentials: credentials,
    transport: transport,
  );
}

SyncCommand _command() {
  const product = Product(
    id: productId,
    code: 'P-1',
    name: 'Cement',
    commonDescription: '',
    brand: '',
    category: '',
    unit: 'Bag',
    sellingPrice: 18500,
    availableQuantity: 10,
    taxBasisPoints: 1800,
    masterDataVersion: 4,
    priceVersion: 8,
  );
  const customer = Customer(
    id: customerId,
    name: 'General Customer',
    code: 'GENERAL',
    isGeneral: true,
    isActive: true,
    creditEnabled: false,
    inAttendantScope: true,
  );
  final sale = CompletedSale(
    serverSaleId: '',
    receiptReference: 'Pending',
    clientTransactionId: transactionId,
    deviceId: deviceId,
    customer: customer,
    saleType: SaleType.cash,
    paymentMethod: PaymentMethod.cash,
    lines: [CartLine(product: product, quantity: 1)],
    total: 18500,
    createdAt: DateTime.utc(2026, 8, 4, 8),
    paymentStatus: 'Paid',
    syncStatus: SyncStatus.pendingSync,
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
    masterDataVersion: 4,
    priceVersion: 8,
    syncAttemptNumber: 1,
    sale: sale,
  );
}

ApiResponse _json(int status, Map<String, Object?> value) {
  final body = Map<String, Object?>.of(value);
  if (body.containsKey('items')) {
    body.putIfAbsent(
      'catalog_snapshot_token',
      () => '00000000-0000-4000-8000-000000000010',
    );
    body.putIfAbsent('master_data_version', () => 4);
    body.putIfAbsent('price_version', () => 8);
  }
  return ApiResponse(
    statusCode: status,
    headers: const {'content-type': 'application/json'},
    body: jsonEncode(body),
  );
}

Map<String, Object?> get _enrollmentJson => {
  'device_id': deviceId,
  'device_name': 'Counter 1',
  'status': 'ACTIVE',
  'actor_id': actorId,
  'scope': {
    'tenant_id': tenantId,
    'company_id': companyId,
    'branch_id': branchId,
    'warehouse_id': warehouseId,
  },
  'app_version': '1.0.0+1',
  'catalog_snapshot_token': '00000000-0000-4000-8000-000000000010',
  'master_data_version': 4,
  'price_version': 8,
  'available_master_data_version': 4,
  'available_price_version': 8,
  'available_catalog_snapshot_token': '00000000-0000-4000-8000-000000000010',
  'timezone': 'Africa/Dar_es_Salaam',
  'offline_enabled': false,
  'transaction_value_limit_minor': 0,
  'daily_value_limit_minor': 0,
  'remaining_daily_value_minor': 0,
  'offline_sales_valid_until': '2026-08-04T10:00:00Z',
  'stock_allocations': <Object?>[],
  'enrolled_at': '2026-08-04T08:00:00Z',
  'last_seen_at': '2026-08-04T08:00:00Z',
};

Map<String, Object?> _syncResultJson({required bool idempotentReplay}) => {
  'client_transaction_id': transactionId,
  'state': 'synced',
  'receipt_reference': 'SALE-$saleId',
  'fiscal_status': 'PENDING',
  'idempotent_replay': idempotentReplay,
  'sale': {
    'id': saleId,
    'customer_id': customerId,
    'currency': 'TZS',
    'subtotal_minor': 20000,
    'tax_minor': 3600,
    'total_minor': 23600,
    'receipt_reference': 'SALE-$saleId',
    'fiscal_status': 'PENDING',
    'lines': [
      {
        'id': '00000000-0000-4000-8000-000000000012',
        'product_id': productId,
        'quantity': 1,
        'unit_price_minor': 20000,
        'subtotal_minor': 20000,
        'tax_minor': 3600,
        'total_minor': 23600,
      },
    ],
  },
};

final _bearerConnection = MobileConnection(
  baseUrl: Uri.parse('https://api.example.test'),
  identityMode: MobileIdentityMode.bearer,
);

final _uuidPattern = RegExp(
  r'^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$',
);
