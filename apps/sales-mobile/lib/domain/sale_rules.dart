import 'models.dart';

enum SaleRuleCode {
  customerRequired,
  generalCustomerCredit,
  inactiveCreditCustomer,
  creditNotEnabled,
  customerOutOfScope,
  creditRequiresOnline,
  emptyCart,
  amountOutsideApiRange,
  staleDraftContext,
  staleProductVersion,
  stockUnavailable,
  offlineCashDisabled,
  offlinePhysicalCashRequired,
  offlineTaxUnsupported,
  offlineAuthorizationExpired,
  offlineValueLimit,
  offlineDailyLimit,
  offlineAllocationExceeded,
  deviceNotApproved,
  creditLimitExceeded,
  overdueCredit,
}

class SaleRuleViolation {
  const SaleRuleViolation(this.code, {this.productName});

  final SaleRuleCode code;
  final String? productName;
}

class SaleRuleException implements Exception {
  SaleRuleException(this.violations);

  final List<SaleRuleViolation> violations;

  @override
  String toString() => 'SaleRuleException(${violations.map((v) => v.code)})';
}

class SaleRules {
  const SaleRules._();

  static bool isCustomerEligible(Customer customer, SaleType saleType) {
    if (!customer.inAttendantScope || !customer.isActive) return false;
    if (saleType == SaleType.cash) return true;
    return !customer.isGeneral &&
        customer.creditEnabled &&
        customer.credit != null;
  }

  static List<SaleRuleViolation> validateCustomer(
    Customer customer,
    SaleType saleType,
  ) {
    final violations = <SaleRuleViolation>[];
    if (!customer.inAttendantScope) {
      violations.add(const SaleRuleViolation(SaleRuleCode.customerOutOfScope));
    }
    if (!customer.isActive && saleType == SaleType.credit) {
      violations.add(
        const SaleRuleViolation(SaleRuleCode.inactiveCreditCustomer),
      );
    }
    if (saleType == SaleType.credit) {
      if (customer.isGeneral) {
        violations.add(
          const SaleRuleViolation(SaleRuleCode.generalCustomerCredit),
        );
      }
      if (!customer.creditEnabled || customer.credit == null) {
        violations.add(const SaleRuleViolation(SaleRuleCode.creditNotEnabled));
      }
    }
    return violations;
  }

  static List<SaleRuleViolation> validateCompletion({
    required SaleDraft draft,
    required bool isOnline,
    required OfflineSalesPolicy offlinePolicy,
    DeviceContext? currentDevice,
    DateTime? evaluatedAt,
  }) {
    final violations = validateCustomer(draft.customer, draft.saleType);
    final authoritativeDevice = currentDevice ?? draft.device;
    if (!authoritativeDevice.approved) {
      violations.add(const SaleRuleViolation(SaleRuleCode.deviceNotApproved));
    }
    if (draft.device.deviceId != authoritativeDevice.deviceId ||
        draft.device.userId != authoritativeDevice.userId ||
        draft.device.tenantId != authoritativeDevice.tenantId ||
        draft.device.companyId != authoritativeDevice.companyId ||
        draft.device.branchId != authoritativeDevice.branchId ||
        draft.device.warehouseId != authoritativeDevice.warehouseId ||
        draft.device.appVersion != authoritativeDevice.appVersion ||
        draft.device.masterDataVersion !=
            authoritativeDevice.masterDataVersion ||
        draft.device.priceVersion != authoritativeDevice.priceVersion) {
      violations.add(const SaleRuleViolation(SaleRuleCode.staleDraftContext));
    }
    if (draft.lines.isEmpty) {
      violations.add(const SaleRuleViolation(SaleRuleCode.emptyCart));
    }
    final checkedTotal = draft.checkedTotal;
    if (!draft.hasApiSafeAmounts) {
      violations.add(
        const SaleRuleViolation(SaleRuleCode.amountOutsideApiRange),
      );
    }
    for (final line in draft.lines) {
      if (line.product.masterDataVersion !=
              authoritativeDevice.masterDataVersion ||
          line.product.priceVersion != authoritativeDevice.priceVersion) {
        violations.add(
          SaleRuleViolation(
            SaleRuleCode.staleProductVersion,
            productName: line.product.name,
          ),
        );
      }
      if (line.quantity > line.product.availableQuantity) {
        violations.add(
          SaleRuleViolation(
            SaleRuleCode.stockUnavailable,
            productName: line.product.name,
          ),
        );
      }
    }

    if (draft.saleType == SaleType.credit) {
      if (!isOnline) {
        violations.add(
          const SaleRuleViolation(SaleRuleCode.creditRequiresOnline),
        );
      }
      final credit = draft.customer.credit;
      if (credit != null) {
        if (checkedTotal != null && checkedTotal > credit.availableCredit) {
          violations.add(
            const SaleRuleViolation(SaleRuleCode.creditLimitExceeded),
          );
        }
        if (credit.overdueAmount > 0) {
          violations.add(const SaleRuleViolation(SaleRuleCode.overdueCredit));
        }
      }
    } else if (!isOnline) {
      if (!offlinePolicy.isLeaseValidAt(evaluatedAt ?? DateTime.now())) {
        violations.add(
          const SaleRuleViolation(SaleRuleCode.offlineAuthorizationExpired),
        );
      }
      if (draft.paymentMethod != PaymentMethod.cash) {
        violations.add(
          const SaleRuleViolation(SaleRuleCode.offlinePhysicalCashRequired),
        );
      }
      if (draft.lines.any(
        (line) =>
            !line.product.hasValidTaxMetadata ||
            line.product.taxBasisPoints != 0,
      )) {
        violations.add(
          const SaleRuleViolation(SaleRuleCode.offlineTaxUnsupported),
        );
      }
      if (!offlinePolicy.enabled) {
        violations.add(
          const SaleRuleViolation(SaleRuleCode.offlineCashDisabled),
        );
      }
      if (checkedTotal != null &&
          checkedTotal > offlinePolicy.transactionValueLimit) {
        violations.add(const SaleRuleViolation(SaleRuleCode.offlineValueLimit));
      }
      if (checkedTotal != null &&
          checkedTotal > offlinePolicy.remainingDailyValue) {
        violations.add(const SaleRuleViolation(SaleRuleCode.offlineDailyLimit));
      }
      for (final line in draft.lines) {
        if (line.quantity > offlinePolicy.allocationFor(line.product.id)) {
          violations.add(
            SaleRuleViolation(
              SaleRuleCode.offlineAllocationExceeded,
              productName: line.product.name,
            ),
          );
        }
      }
    }
    return violations;
  }

  static void enforceCompletion({
    required SaleDraft draft,
    required bool isOnline,
    required OfflineSalesPolicy offlinePolicy,
    DeviceContext? currentDevice,
    DateTime? evaluatedAt,
  }) {
    final violations = validateCompletion(
      draft: draft,
      isOnline: isOnline,
      offlinePolicy: offlinePolicy,
      currentDevice: currentDevice,
      evaluatedAt: evaluatedAt,
    );
    if (violations.isNotEmpty) throw SaleRuleException(violations);
  }
}
