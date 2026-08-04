import 'models.dart';

enum SaleRuleCode {
  customerRequired,
  generalCustomerCredit,
  inactiveCreditCustomer,
  creditNotEnabled,
  customerOutOfScope,
  creditRequiresOnline,
  emptyCart,
  stockUnavailable,
  offlineCashDisabled,
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
  }) {
    final violations = validateCustomer(draft.customer, draft.saleType);
    if (!draft.device.approved) {
      violations.add(const SaleRuleViolation(SaleRuleCode.deviceNotApproved));
    }
    if (draft.lines.isEmpty) {
      violations.add(const SaleRuleViolation(SaleRuleCode.emptyCart));
    }
    for (final line in draft.lines) {
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
        if (draft.total > credit.availableCredit) {
          violations.add(
            const SaleRuleViolation(SaleRuleCode.creditLimitExceeded),
          );
        }
        if (credit.overdueAmount > 0) {
          violations.add(const SaleRuleViolation(SaleRuleCode.overdueCredit));
        }
      }
    } else if (!isOnline) {
      if (!offlinePolicy.enabled) {
        violations.add(
          const SaleRuleViolation(SaleRuleCode.offlineCashDisabled),
        );
      }
      if (draft.total > offlinePolicy.transactionValueLimit) {
        violations.add(const SaleRuleViolation(SaleRuleCode.offlineValueLimit));
      }
      if (draft.total > offlinePolicy.remainingDailyValue) {
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
  }) {
    final violations = validateCompletion(
      draft: draft,
      isOnline: isOnline,
      offlinePolicy: offlinePolicy,
    );
    if (violations.isNotEmpty) throw SaleRuleException(violations);
  }
}
