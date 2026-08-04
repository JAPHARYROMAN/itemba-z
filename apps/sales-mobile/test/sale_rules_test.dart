import 'package:flutter_test/flutter_test.dart';
import 'package:sales_mobile/application/sales_controller.dart';
import 'package:sales_mobile/data/demo_data.dart';
import 'package:sales_mobile/domain/models.dart';
import 'package:sales_mobile/domain/sale_rules.dart';

void main() {
  group('new sale defaults', () {
    test('is Cash + General Customer with assigned scope', () {
      final controller = SalesController();
      final draft = controller.createNewSale(now: DateTime(2026, 8, 4));

      expect(draft.saleType, SaleType.cash);
      expect(draft.customer.isGeneral, isTrue);
      expect(draft.device.companyId, isNotEmpty);
      expect(draft.device.branchId, isNotEmpty);
      expect(draft.device.warehouseId, isNotEmpty);
    });
  });

  group('customer eligibility', () {
    test('General Customer is always rejected for credit', () {
      final general = demoCustomers.firstWhere((value) => value.isGeneral);
      final violations = SaleRules.validateCustomer(general, SaleType.credit);

      expect(
        violations.map((value) => value.code),
        contains(SaleRuleCode.generalCustomerCredit),
      );
      expect(SaleRules.isCustomerEligible(general, SaleType.credit), isFalse);
    });

    test(
      'only active scoped credit-enabled registered customer is eligible',
      () {
        final eligible = demoCustomers.firstWhere(
          (value) => value.id == 'customer-kijiji',
        );
        final inactive = demoCustomers.firstWhere(
          (value) => value.id == 'customer-inactive',
        );

        expect(SaleRules.isCustomerEligible(eligible, SaleType.credit), isTrue);
        expect(
          SaleRules.isCustomerEligible(inactive, SaleType.credit),
          isFalse,
        );
      },
    );
  });

  group('completion controls', () {
    test('credit sale is blocked while offline', () {
      final controller = SalesController();
      final draft = controller.createNewSale();
      draft.saleType = SaleType.credit;
      draft.customer = demoCustomers.firstWhere(
        (value) => value.id == 'customer-kijiji',
      );
      draft.addProduct(demoProducts.first);

      final violations = SaleRules.validateCompletion(
        draft: draft,
        isOnline: false,
        offlinePolicy: demoOfflinePolicy,
      );

      expect(
        violations.map((value) => value.code),
        contains(SaleRuleCode.creditRequiresOnline),
      );
    });

    test('offline cash respects per-device stock allocation', () {
      final controller = SalesController();
      final draft = controller.createNewSale();
      draft.addProduct(demoProducts.first);
      draft.lines.first.setQuantity(21);

      final violations = SaleRules.validateCompletion(
        draft: draft,
        isOnline: false,
        offlinePolicy: demoOfflinePolicy,
      );

      expect(
        violations.map((value) => value.code),
        contains(SaleRuleCode.offlineAllocationExceeded),
      );
    });

    test('cart line price cannot diverge from product selling price', () {
      final line = CartLine(product: demoProducts.first, quantity: 2);
      expect(line.unitPrice, demoProducts.first.sellingPrice);
      expect(line.subtotal, demoProducts.first.sellingPrice * 2);
    });
  });
}
