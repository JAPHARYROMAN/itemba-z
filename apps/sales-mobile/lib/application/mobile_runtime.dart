import 'package:flutter/foundation.dart';

import '../core/uuid.dart';
import '../data/http_api_transport.dart';
import '../data/http_sync_gateway.dart';
import '../data/itemba_api_client.dart';
import '../data/local_store.dart';
import '../data/mobile_credential_store.dart';
import '../domain/connection_models.dart';
import '../domain/models.dart';
import 'sales_controller.dart';

const String salesMobileAppVersion = String.fromEnvironment(
  'ITEMBA_Z_APP_VERSION',
  defaultValue: '1.0.0+1',
);
const String configuredDevelopmentDeviceId = String.fromEnvironment(
  'ITEMBA_DEV_DEVICE_ID',
);

String? selectDevelopmentDeviceId({
  required bool isRelease,
  required MobileIdentityMode identityMode,
  required String configuredValue,
}) {
  if (isRelease || identityMode != MobileIdentityMode.developmentHeaders) {
    return null;
  }
  final value = configuredValue.trim().toLowerCase();
  if (value.isEmpty) return null;
  if (!UuidGenerator.isValid(value)) {
    throw const FormatException('ITEMBA_DEV_DEVICE_ID must be a valid UUID.');
  }
  return value;
}

typedef ApiTransportFactory = ApiTransport Function();

class MobileRuntimeController extends ChangeNotifier {
  MobileRuntimeController({
    required this.store,
    required this.credentials,
    ApiTransportFactory? transportFactory,
    UuidGenerator? uuid,
  }) : _transportFactory = transportFactory ?? DartIoApiTransport.new,
       _uuid = uuid ?? UuidGenerator();

  final EncryptedLocalStore store;
  final MobileCredentialStore credentials;
  final ApiTransportFactory _transportFactory;
  final UuidGenerator _uuid;

  LiveConnectionState state = LiveConnectionState.refreshing;
  String? errorMessage;
  MobileConnection? connection;
  DeviceEnrollment? enrollment;
  SalesController? salesController;
  ItembaApiClient? _api;
  ApiTransport? _transport;

  bool get busy =>
      state == LiveConnectionState.enrolling ||
      state == LiveConnectionState.refreshing;

  Future<void> initialize() async {
    await store.initialize();
    connection = await store.readConnection();
    enrollment = await store.readEnrollment();
    if (connection == null || enrollment == null) {
      _setState(LiveConnectionState.notConfigured);
      return;
    }
    if (enrollment!.status == 'VERSION_UNCERTAIN') {
      errorMessage =
          'Device versions are uncertain after an interrupted enrollment. Reconnect to recover safely.';
      _setState(LiveConnectionState.error);
      return;
    }
    if (enrollment!.status == 'INSTALL_ACK_PENDING') {
      await store.saveCandidateDeviceId(enrollment!.deviceId);
      await _activate(connection!, enrollment!);
      return;
    }
    await store.saveCandidateDeviceId(enrollment!.deviceId);
    if (!enrollment!.isActive) {
      _setState(LiveConnectionState.suspended);
      return;
    }
    await _activate(connection!, enrollment!);
  }

  Future<void> enroll({
    required Uri baseUrl,
    required String deviceName,
    required MobileIdentityMode identityMode,
    String? bearerToken,
    DevelopmentIdentity? developmentIdentity,
  }) async {
    _setState(LiveConnectionState.enrolling);
    final nextConnection = MobileConnection(
      baseUrl: baseUrl,
      identityMode: identityMode,
      developmentIdentity: developmentIdentity,
    );
    try {
      nextConnection.validate(
        allowDevelopmentIdentity: allowDevelopmentIdentity,
      );
      final pending = await store.readSyncQueue();
      final previousEnrollment = enrollment ?? await store.readEnrollment();
      final previousConnection = connection ?? await store.readConnection();
      final previousAllocation = await store.readAllocation();
      if (pending.isNotEmpty && previousEnrollment == null) {
        throw const ApiException(
          kind: ApiFailureKind.terminal,
          message:
              'Pending transactions require governed recovery before enrollment.',
        );
      }
      if (pending.isNotEmpty &&
          previousConnection != null &&
          previousConnection.baseUrl != nextConnection.baseUrl) {
        throw const ApiException(
          kind: ApiFailureKind.terminal,
          message:
              'The API address cannot change while transactions are pending.',
        );
      }
      final previousToken = await credentials.readBearerToken();
      if (identityMode == MobileIdentityMode.bearer) {
        final token = bearerToken?.trim() ?? '';
        if (token.isEmpty) {
          throw const ApiException(
            kind: ApiFailureKind.authentication,
            message: 'A bearer credential is required.',
          );
        }
        await credentials.saveBearerToken(token);
      }
      final transport = _transportFactory();
      final api = ItembaApiClient(
        connection: nextConnection,
        credentials: credentials,
        transport: transport,
      );
      try {
        final configuredDeviceId = selectDevelopmentDeviceId(
          isRelease: kReleaseMode,
          identityMode: identityMode,
          configuredValue: configuredDevelopmentDeviceId,
        );
        final storedCandidateDeviceId = await store.readCandidateDeviceId();
        if (storedCandidateDeviceId != null &&
            !UuidGenerator.isValid(storedCandidateDeviceId)) {
          throw const FormatException(
            'Stored candidate device identity is invalid.',
          );
        }
        final candidateDeviceId =
            previousEnrollment?.deviceId ??
            storedCandidateDeviceId ??
            configuredDeviceId ??
            _uuid.v4();
        await store.saveCandidateDeviceId(candidateDeviceId);
        final result = await api.enrollDevice(
          deviceId: candidateDeviceId,
          deviceName: deviceName.trim(),
          appVersion: salesMobileAppVersion,
        );
        if (!result.isActive) {
          throw const ApiException(
            kind: ApiFailureKind.suspended,
            message: 'This device is not active.',
          );
        }
        final bindingChanged =
            previousEnrollment != null &&
            !_sameBinding(previousEnrollment, result);
        if (pending.isNotEmpty && bindingChanged) {
          throw const ApiException(
            kind: ApiFailureKind.terminal,
            message:
                'Device or organizational scope cannot change while transactions are pending.',
          );
        }
        final connectionChanged =
            previousConnection != null &&
            previousConnection.baseUrl != nextConnection.baseUrl;
        if (pending.isEmpty &&
            (previousEnrollment == null ||
                bindingChanged ||
                connectionChanged)) {
          await store.clearSensitiveCache();
          await store.saveCandidateDeviceId(result.deviceId);
        }
        final allocation = _reconcileAllocation(
          server: _allocationFromEnrollment(result),
          local: previousAllocation,
          hasPending: pending.isNotEmpty,
        );
        await store.saveConnection(nextConnection);
        await store.saveEnrollment(result);
        await store.saveAllocation(allocation);
        if (identityMode == MobileIdentityMode.developmentHeaders) {
          await credentials.deleteBearerToken();
        }
        await _closeTransport();
        _transport = transport;
        _api = api;
        connection = nextConnection;
        enrollment = result;
        await _buildSalesController(api, result, result, allocation);
      } catch (_) {
        await transport.close();
        if (identityMode == MobileIdentityMode.bearer) {
          if (previousToken == null) {
            await credentials.deleteBearerToken();
          } else {
            await credentials.saveBearerToken(previousToken);
          }
        }
        rethrow;
      }
    } on ApiException catch (error) {
      errorMessage = error.message;
      _setState(_stateForApiFailure(error));
    } on FormatException catch (error) {
      errorMessage = error.message;
      _setState(LiveConnectionState.notConfigured);
    } catch (_) {
      errorMessage = 'Device enrollment failed.';
      _setState(LiveConnectionState.error);
    }
  }

  Future<void> refreshMasterData() async {
    final api = _api;
    final controller = salesController;
    final installedEnrollment = controller?.enrollment ?? enrollment;
    if (api == null || controller == null || installedEnrollment == null) {
      return;
    }
    var availableEnrollment = installedEnrollment;
    controller.updateLiveConnectionState(LiveConnectionState.refreshing);
    try {
      availableEnrollment = await api.enrollDevice(
        deviceId: installedEnrollment.deviceId,
        deviceName: installedEnrollment.deviceName,
        appVersion: salesMobileAppVersion,
      );
      if (!availableEnrollment.isActive) {
        throw const ApiException(
          kind: ApiFailureKind.suspended,
          message: 'This device is not active.',
        );
      }
      if (!_sameBinding(installedEnrollment, availableEnrollment)) {
        throw const ApiException(
          kind: ApiFailureKind.terminal,
          message:
              'Device or organizational scope changed during master-data refresh.',
        );
      }
      final snapshot = await api.refreshMasterData();
      _requireSnapshotVersions(snapshot, availableEnrollment);
      final stagedEnrollment = availableEnrollment
          .withInstalledVersions(
            masterDataVersion: snapshot.masterDataVersion,
            priceVersion: snapshot.priceVersion,
          )
          .withLocalStatus('INSTALL_ACK_PENDING');
      await controller.replaceMasterData(
        customers: snapshot.customers,
        products: snapshot.products,
        masterDataVersion: snapshot.masterDataVersion,
        priceVersion: snapshot.priceVersion,
        enrollmentToInstall: stagedEnrollment,
      );
      enrollment = stagedEnrollment;
      final acknowledged = await _acknowledgeInstalledVersions(
        api,
        stagedEnrollment,
        snapshot,
      );
      await store.saveEnrollment(acknowledged);
      controller.applyAcknowledgedEnrollment(acknowledged);
      final pending = await store.readSyncQueue();
      final allocation = _reconcileAllocation(
        server: _allocationFromEnrollment(acknowledged),
        local: await store.readAllocation(),
        hasPending: pending.isNotEmpty,
      );
      await store.saveAllocation(allocation);
      controller.replaceDeviceAllocation(allocation);
      enrollment = acknowledged;
      controller.setConnectivity(true);
      controller.updateLiveConnectionState(LiveConnectionState.ready);
      errorMessage = null;
      _setState(LiveConnectionState.ready);
    } on ApiException catch (error) {
      errorMessage = error.message;
      final installedVersions = await store.readInstalledMasterDataVersions();
      final cacheMatchesInstalled =
          installedVersions?.matches(
            masterDataVersion: availableEnrollment.masterDataVersion,
            priceVersion: availableEnrollment.priceVersion,
          ) ??
          false;
      if (error.kind == ApiFailureKind.retryable &&
          cacheMatchesInstalled &&
          availableEnrollment.masterDataVersion > 0 &&
          availableEnrollment.priceVersion > 0 &&
          availableEnrollment.masterDataVersion ==
              availableEnrollment.availableMasterDataVersion &&
          availableEnrollment.priceVersion ==
              availableEnrollment.availablePriceVersion &&
          installedEnrollment.appVersion == salesMobileAppVersion &&
          availableEnrollment.appVersion == salesMobileAppVersion &&
          controller.offlinePolicy.isLeaseValidAt(DateTime.now()) &&
          installedEnrollment.status != 'INSTALL_ACK_PENDING' &&
          controller.customers.isNotEmpty &&
          controller.products.isNotEmpty &&
          controller.products.every((product) => product.hasValidTaxMetadata)) {
        controller.setConnectivity(false);
        controller.updateLiveConnectionState(LiveConnectionState.offlineCache);
        _setState(LiveConnectionState.offlineCache);
      } else {
        final failureState = _stateForApiFailure(error);
        controller.updateLiveConnectionState(failureState);
        _setState(failureState);
      }
    } catch (_) {
      errorMessage = 'Master data could not be validated.';
      controller.updateLiveConnectionState(LiveConnectionState.error);
      _setState(LiveConnectionState.error);
    }
  }

  Future<void> _activate(
    MobileConnection activeConnection,
    DeviceEnrollment activeEnrollment,
  ) async {
    try {
      activeConnection.validate(
        allowDevelopmentIdentity: allowDevelopmentIdentity,
      );
      if (activeConnection.identityMode == MobileIdentityMode.bearer &&
          (await credentials.readBearerToken())?.trim().isEmpty != false) {
        _setState(LiveConnectionState.authenticationRequired);
        return;
      }
      await _closeTransport();
      final transport = _transportFactory();
      final api = ItembaApiClient(
        connection: activeConnection,
        credentials: credentials,
        transport: transport,
      );
      _transport = transport;
      _api = api;
      var effectiveEnrollment = activeEnrollment;
      var availableEnrollment = activeEnrollment;
      var allocation =
          await store.readAllocation() ??
          _allocationFromEnrollment(activeEnrollment);
      final pending = await store.readSyncQueue();
      try {
        final refreshed = await api.enrollDevice(
          deviceId: activeEnrollment.deviceId,
          deviceName: activeEnrollment.deviceName,
          appVersion: salesMobileAppVersion,
        );
        if (!refreshed.isActive) {
          throw const ApiException(
            kind: ApiFailureKind.suspended,
            message: 'This device is not active.',
          );
        }
        final bindingChanged = !_sameBinding(activeEnrollment, refreshed);
        if (pending.isNotEmpty && bindingChanged) {
          throw const ApiException(
            kind: ApiFailureKind.terminal,
            message:
                'Device or organizational scope cannot change while transactions are pending.',
          );
        }
        if (pending.isEmpty && bindingChanged) {
          await store.clearSensitiveCache();
          await store.saveCandidateDeviceId(refreshed.deviceId);
          await store.saveConnection(activeConnection);
        }
        availableEnrollment = refreshed;
        effectiveEnrollment = refreshed;
        allocation = _reconcileAllocation(
          server: _allocationFromEnrollment(refreshed),
          local: allocation,
          hasPending: pending.isNotEmpty,
        );
        await store.saveEnrollment(effectiveEnrollment);
        await store.saveAllocation(allocation);
        enrollment = effectiveEnrollment;
      } on ApiException catch (error) {
        if (!error.isRetryable) rethrow;
      }
      await _buildSalesController(
        api,
        effectiveEnrollment,
        availableEnrollment,
        allocation,
      );
    } on ApiException catch (error) {
      errorMessage = error.message;
      _setState(_stateForApiFailure(error));
    } on FormatException catch (error) {
      errorMessage = error.message;
      _setState(LiveConnectionState.notConfigured);
    } catch (_) {
      errorMessage = 'The saved mobile configuration could not be opened.';
      _setState(LiveConnectionState.error);
    }
  }

  Future<void> _buildSalesController(
    ItembaApiClient api,
    DeviceEnrollment installedEnrollment,
    DeviceEnrollment availableEnrollment,
    DeviceAllocation allocation,
  ) async {
    final context = DeviceContext(
      deviceId: installedEnrollment.deviceId,
      userId: installedEnrollment.actorId,
      tenantId: installedEnrollment.scope.tenantId,
      attendantName: installedEnrollment.actorId,
      companyId: installedEnrollment.scope.companyId,
      companyName: installedEnrollment.scope.companyId,
      branchId: installedEnrollment.scope.branchId,
      branchName: installedEnrollment.scope.branchId,
      warehouseId: installedEnrollment.scope.warehouseId,
      warehouseName: installedEnrollment.scope.warehouseId,
      appVersion: installedEnrollment.appVersion,
      masterDataVersion: installedEnrollment.masterDataVersion,
      priceVersion: installedEnrollment.priceVersion,
      approved: installedEnrollment.isActive,
    );
    final offlineEnabled =
        allocation.offlineEnabled &&
        DateTime.now().toUtc().isBefore(
          allocation.offlineSalesValidUntil.toUtc(),
        ) &&
        allocation.transactionLimitMinor > 0 &&
        allocation.dailyValueLimitMinor > 0 &&
        allocation.remainingDailyMinor > 0 &&
        allocation.productQuantities.values.any((quantity) => quantity > 0);
    final controller = SalesController(
      device: context,
      customers: const [],
      products: const [],
      offlinePolicy: OfflineSalesPolicy(
        enabled: offlineEnabled,
        transactionValueLimit: allocation.transactionLimitMinor,
        remainingDailyValue: allocation.remainingDailyMinor,
        offlineSalesValidUntil: allocation.offlineSalesValidUntil,
        productAllocations: allocation.productQuantities,
      ),
      deviceAllocation: allocation,
      store: store,
      gateway: HttpAuthoritativeSyncGateway(api: api),
      connection: connection,
      enrollment: installedEnrollment,
      masterDataRefresher: refreshMasterData,
      liveConnectionState: LiveConnectionState.refreshing,
      allowConnectivitySimulation: false,
    );
    await controller.initializeLocalData(seedWhenEmpty: false);
    final installedVersions = await store.readInstalledMasterDataVersions();
    final cacheMatchesInstalled =
        installedVersions?.matches(
          masterDataVersion: installedEnrollment.masterDataVersion,
          priceVersion: installedEnrollment.priceVersion,
        ) ??
        false;
    final hasUsableCache =
        cacheMatchesInstalled &&
        installedEnrollment.masterDataVersion > 0 &&
        installedEnrollment.priceVersion > 0 &&
        installedEnrollment.masterDataVersion ==
            availableEnrollment.availableMasterDataVersion &&
        installedEnrollment.priceVersion ==
            availableEnrollment.availablePriceVersion &&
        installedEnrollment.appVersion == salesMobileAppVersion &&
        availableEnrollment.appVersion == salesMobileAppVersion &&
        DateTime.now().toUtc().isBefore(
          allocation.offlineSalesValidUntil.toUtc(),
        ) &&
        installedEnrollment.status != 'INSTALL_ACK_PENDING' &&
        controller.customers.any(
          (customer) => customer.isGeneral && customer.isActive,
        ) &&
        controller.products.isNotEmpty &&
        controller.products.every((product) => product.hasValidTaxMetadata);
    try {
      final snapshot = await api.refreshMasterData();
      _requireSnapshotVersions(snapshot, availableEnrollment);
      final stagedEnrollment = availableEnrollment
          .withInstalledVersions(
            masterDataVersion: snapshot.masterDataVersion,
            priceVersion: snapshot.priceVersion,
          )
          .withLocalStatus('INSTALL_ACK_PENDING');
      await controller.replaceMasterData(
        customers: snapshot.customers,
        products: snapshot.products,
        masterDataVersion: snapshot.masterDataVersion,
        priceVersion: snapshot.priceVersion,
        enrollmentToInstall: stagedEnrollment,
      );
      enrollment = stagedEnrollment;
      final acknowledged = await _acknowledgeInstalledVersions(
        api,
        stagedEnrollment,
        snapshot,
      );
      await store.saveEnrollment(acknowledged);
      controller.applyAcknowledgedEnrollment(acknowledged);
      enrollment = acknowledged;
      final queuedAfterInstall = await store.readSyncQueue();
      final acknowledgedAllocation = _reconcileAllocation(
        server: _allocationFromEnrollment(acknowledged),
        local: allocation,
        hasPending: queuedAfterInstall.isNotEmpty,
      );
      await store.saveAllocation(acknowledgedAllocation);
      controller.replaceDeviceAllocation(acknowledgedAllocation);
      controller.setConnectivity(true);
      controller.updateLiveConnectionState(LiveConnectionState.ready);
      salesController?.dispose();
      salesController = controller;
      errorMessage = null;
      _setState(LiveConnectionState.ready);
      if (controller.pendingCount > 0) {
        await controller.synchronizePending();
      }
    } on ApiException catch (error) {
      errorMessage = error.message;
      if (error.kind == ApiFailureKind.retryable && hasUsableCache) {
        controller.setConnectivity(false);
        controller.updateLiveConnectionState(LiveConnectionState.offlineCache);
        salesController?.dispose();
        salesController = controller;
        _setState(LiveConnectionState.offlineCache);
      } else {
        controller.dispose();
        _setState(_stateForApiFailure(error));
      }
    }
  }

  LiveConnectionState _stateForApiFailure(ApiException error) => switch (error
      .kind) {
    ApiFailureKind.authentication => LiveConnectionState.authenticationRequired,
    ApiFailureKind.suspended => LiveConnectionState.suspended,
    ApiFailureKind.retryable ||
    ApiFailureKind.terminal ||
    ApiFailureKind.invalidResponse => LiveConnectionState.error,
  };

  DeviceAllocation _allocationFromEnrollment(DeviceEnrollment value) =>
      DeviceAllocation(
        offlineEnabled: value.offlineEnabled,
        transactionLimitMinor: value.transactionValueLimitMinor,
        dailyValueLimitMinor: value.dailyValueLimitMinor,
        remainingDailyMinor: value.remainingDailyValueMinor,
        offlineSalesValidUntil: value.offlineSalesValidUntil,
        productQuantities: Map.unmodifiable({
          for (final allocation in value.stockAllocations)
            allocation.productId: allocation.remainingQuantity,
        }),
        updatedAt: value.lastSeenAt,
      );

  void _requireSnapshotVersions(
    MasterDataSnapshot snapshot,
    DeviceEnrollment availableEnrollment,
  ) {
    if (snapshot.masterDataVersion !=
            availableEnrollment.availableMasterDataVersion ||
        snapshot.priceVersion != availableEnrollment.availablePriceVersion) {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message:
            'Downloaded master data does not match the server-advertised versions.',
      );
    }
  }

  Future<DeviceEnrollment> _acknowledgeInstalledVersions(
    ItembaApiClient api,
    DeviceEnrollment stagedEnrollment,
    MasterDataSnapshot snapshot,
  ) async {
    final acknowledged = await api.enrollDevice(
      deviceId: stagedEnrollment.deviceId,
      deviceName: stagedEnrollment.deviceName,
      appVersion: salesMobileAppVersion,
      installedMasterDataVersion: snapshot.masterDataVersion,
      installedPriceVersion: snapshot.priceVersion,
    );
    if (!acknowledged.isActive) {
      throw const ApiException(
        kind: ApiFailureKind.suspended,
        message: 'This device is not active.',
      );
    }
    if (!_sameBinding(stagedEnrollment, acknowledged) ||
        acknowledged.appVersion != salesMobileAppVersion ||
        acknowledged.masterDataVersion != snapshot.masterDataVersion ||
        acknowledged.priceVersion != snapshot.priceVersion ||
        acknowledged.availableMasterDataVersion != snapshot.masterDataVersion ||
        acknowledged.availablePriceVersion != snapshot.priceVersion) {
      throw const ApiException(
        kind: ApiFailureKind.invalidResponse,
        message:
            'The server did not acknowledge the installed master-data versions.',
      );
    }
    return acknowledged;
  }

  DeviceAllocation _reconcileAllocation({
    required DeviceAllocation server,
    required DeviceAllocation? local,
    required bool hasPending,
  }) {
    if (!hasPending) return server;
    if (local == null) {
      return DeviceAllocation.failClosed(
        serverOfflineEnabled: server.offlineEnabled,
      );
    }
    final quantities = <String, int>{};
    for (final entry in server.productQuantities.entries) {
      final localRemaining = local.productQuantities[entry.key] ?? 0;
      quantities[entry.key] = _minimum(entry.value, localRemaining);
    }
    return DeviceAllocation(
      offlineEnabled: server.offlineEnabled && local.offlineEnabled,
      transactionLimitMinor: _minimum(
        server.transactionLimitMinor,
        local.transactionLimitMinor,
      ),
      dailyValueLimitMinor: _minimum(
        server.dailyValueLimitMinor,
        local.dailyValueLimitMinor,
      ),
      remainingDailyMinor: _minimum(
        server.remainingDailyMinor,
        local.remainingDailyMinor,
      ),
      offlineSalesValidUntil: _earlier(
        server.offlineSalesValidUntil,
        local.offlineSalesValidUntil,
      ),
      productQuantities: Map.unmodifiable(quantities),
      updatedAt: server.updatedAt,
    );
  }

  bool _sameBinding(DeviceEnrollment left, DeviceEnrollment right) =>
      left.deviceId == right.deviceId &&
      left.actorId == right.actorId &&
      left.scope.tenantId == right.scope.tenantId &&
      left.scope.companyId == right.scope.companyId &&
      left.scope.branchId == right.scope.branchId &&
      left.scope.warehouseId == right.scope.warehouseId;

  int _minimum(int left, int right) => left < right ? left : right;

  DateTime _earlier(DateTime left, DateTime right) =>
      left.isBefore(right) ? left : right;

  void _setState(LiveConnectionState value) {
    state = value;
    notifyListeners();
  }

  Future<void> _closeTransport() async {
    final transport = _transport;
    _transport = null;
    _api = null;
    if (transport != null) await transport.close();
  }

  Future<void> close() async {
    salesController?.dispose();
    salesController = null;
    await _closeTransport();
    await store.close();
  }
}
