enum MobileIdentityMode { bearer, developmentHeaders }

class DevelopmentIdentity {
  const DevelopmentIdentity({
    required this.actorId,
    required this.tenantId,
    required this.companyId,
    required this.branchId,
    required this.warehouseId,
  });

  final String actorId;
  final String tenantId;
  final String companyId;
  final String branchId;
  final String warehouseId;

  Map<String, String> get headers => {
    'X-Actor-ID': actorId,
    'X-Tenant-ID': tenantId,
    'X-Company-ID': companyId,
    'X-Branch-ID': branchId,
    'X-Warehouse-ID': warehouseId,
  };
}

class MobileConnection {
  const MobileConnection({
    required this.baseUrl,
    required this.identityMode,
    this.developmentIdentity,
  });

  final Uri baseUrl;
  final MobileIdentityMode identityMode;
  final DevelopmentIdentity? developmentIdentity;

  static final RegExp _uuid = RegExp(
    r'^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$',
    caseSensitive: false,
  );

  void validate({required bool allowDevelopmentIdentity}) {
    if (!baseUrl.hasScheme || !baseUrl.hasAuthority) {
      throw const FormatException('Base URL must be absolute.');
    }
    if (baseUrl.hasQuery ||
        baseUrl.hasFragment ||
        baseUrl.userInfo.isNotEmpty) {
      throw const FormatException(
        'Base URL cannot contain credentials, a query, or a fragment.',
      );
    }
    if (identityMode == MobileIdentityMode.bearer &&
        baseUrl.scheme != 'https') {
      throw const FormatException('Production API connections require HTTPS.');
    }
    if (identityMode == MobileIdentityMode.developmentHeaders) {
      if (!allowDevelopmentIdentity || developmentIdentity == null) {
        throw const FormatException('Development identity is disabled.');
      }
      final host = baseUrl.host.toLowerCase();
      if (baseUrl.scheme != 'http' ||
          !const {'localhost', '127.0.0.1', '10.0.2.2'}.contains(host)) {
        throw const FormatException(
          'Development identity is restricted to a local HTTP API.',
        );
      }
      final identity = developmentIdentity!;
      if ([
        identity.actorId,
        identity.tenantId,
        identity.companyId,
        identity.branchId,
        identity.warehouseId,
      ].any((value) => !_uuid.hasMatch(value))) {
        throw const FormatException(
          'Development identity fields must be UUIDs.',
        );
      }
    }
  }
}

class EnrollmentScope {
  const EnrollmentScope({
    required this.tenantId,
    required this.companyId,
    required this.branchId,
    required this.warehouseId,
  });

  final String tenantId;
  final String companyId;
  final String branchId;
  final String warehouseId;
}

class DeviceEnrollment {
  const DeviceEnrollment({
    required this.deviceId,
    required this.deviceName,
    required this.status,
    required this.actorId,
    required this.scope,
    required this.appVersion,
    required this.masterDataVersion,
    required this.priceVersion,
    required this.availableMasterDataVersion,
    required this.availablePriceVersion,
    required this.timezone,
    required this.offlineEnabled,
    required this.transactionValueLimitMinor,
    required this.dailyValueLimitMinor,
    required this.remainingDailyValueMinor,
    required this.offlineSalesValidUntil,
    required this.stockAllocations,
    required this.enrolledAt,
    required this.lastSeenAt,
  });

  final String deviceId;
  final String deviceName;
  final String status;
  final String actorId;
  final EnrollmentScope scope;
  final String appVersion;
  final int masterDataVersion;
  final int priceVersion;
  final int availableMasterDataVersion;
  final int availablePriceVersion;
  final String timezone;
  final bool offlineEnabled;
  final int transactionValueLimitMinor;
  final int dailyValueLimitMinor;
  final int remainingDailyValueMinor;
  final DateTime offlineSalesValidUntil;
  final List<MobileStockAllocation> stockAllocations;
  final DateTime enrolledAt;
  final DateTime lastSeenAt;

  bool get isActive => status == 'ACTIVE';

  DeviceEnrollment withInstalledVersions({
    required int masterDataVersion,
    required int priceVersion,
  }) => DeviceEnrollment(
    deviceId: deviceId,
    deviceName: deviceName,
    status: status,
    actorId: actorId,
    scope: scope,
    appVersion: appVersion,
    masterDataVersion: masterDataVersion,
    priceVersion: priceVersion,
    availableMasterDataVersion: availableMasterDataVersion,
    availablePriceVersion: availablePriceVersion,
    timezone: timezone,
    offlineEnabled: offlineEnabled,
    transactionValueLimitMinor: transactionValueLimitMinor,
    dailyValueLimitMinor: dailyValueLimitMinor,
    remainingDailyValueMinor: remainingDailyValueMinor,
    offlineSalesValidUntil: offlineSalesValidUntil,
    stockAllocations: stockAllocations,
    enrolledAt: enrolledAt,
    lastSeenAt: lastSeenAt,
  );

  DeviceEnrollment withLocalStatus(String status) => DeviceEnrollment(
    deviceId: deviceId,
    deviceName: deviceName,
    status: status,
    actorId: actorId,
    scope: scope,
    appVersion: appVersion,
    masterDataVersion: masterDataVersion,
    priceVersion: priceVersion,
    availableMasterDataVersion: availableMasterDataVersion,
    availablePriceVersion: availablePriceVersion,
    timezone: timezone,
    offlineEnabled: offlineEnabled,
    transactionValueLimitMinor: transactionValueLimitMinor,
    dailyValueLimitMinor: dailyValueLimitMinor,
    remainingDailyValueMinor: remainingDailyValueMinor,
    offlineSalesValidUntil: offlineSalesValidUntil,
    stockAllocations: stockAllocations,
    enrolledAt: enrolledAt,
    lastSeenAt: lastSeenAt,
  );
}

class MobileStockAllocation {
  const MobileStockAllocation({
    required this.productId,
    required this.allocatedQuantity,
    required this.remainingQuantity,
  });

  final String productId;
  final int allocatedQuantity;
  final int remainingQuantity;
}

/// A server-issued or administrator-provisioned offline allocation. Enrollment
/// supplies limits and per-product quantities; an empty allocation fails closed.
class DeviceAllocation {
  const DeviceAllocation({
    required this.offlineEnabled,
    required this.transactionLimitMinor,
    required this.dailyValueLimitMinor,
    required this.remainingDailyMinor,
    required this.offlineSalesValidUntil,
    required this.productQuantities,
    required this.updatedAt,
  });

  factory DeviceAllocation.failClosed({required bool serverOfflineEnabled}) =>
      DeviceAllocation(
        offlineEnabled: false,
        transactionLimitMinor: 0,
        dailyValueLimitMinor: 0,
        remainingDailyMinor: 0,
        offlineSalesValidUntil: DateTime.fromMillisecondsSinceEpoch(
          0,
          isUtc: true,
        ),
        productQuantities: const {},
        updatedAt: DateTime.now().toUtc(),
      );

  final bool offlineEnabled;
  final int transactionLimitMinor;
  final int dailyValueLimitMinor;
  final int remainingDailyMinor;
  final DateTime offlineSalesValidUntil;
  final Map<String, int> productQuantities;
  final DateTime updatedAt;
}

enum LiveConnectionState {
  notConfigured,
  enrolling,
  refreshing,
  ready,
  offlineCache,
  authenticationRequired,
  suspended,
  error,
}
