import 'package:flutter_test/flutter_test.dart';
import 'package:sales_mobile/application/sales_controller.dart';
import 'package:sales_mobile/data/demo_data.dart';
import 'package:sales_mobile/data/local_store.dart';
import 'package:sales_mobile/core/app_strings.dart';
import 'package:sales_mobile/domain/models.dart';
import 'package:sales_mobile/domain/sale_rules.dart';

void main() {
  test('deferred credit policy is explicit in both languages', () {
    expect(
      const AppStrings(AppLanguage.english).t('creditPolicyUnavailable'),
      contains('blocked'),
    );
    expect(
      const AppStrings(AppLanguage.swahili).t('creditPolicyUnavailable'),
      contains('umezuiwa'),
    );
  });

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

    test('tax uses exact half-up integer rounding and total includes tax', () {
      const product = Product(
        id: 'taxed-product',
        code: 'TAX-1',
        name: 'Taxed product',
        commonDescription: '',
        brand: '',
        category: '',
        unit: 'EA',
        sellingPrice: 25,
        availableQuantity: 10,
        taxBasisPoints: 1800,
      );
      final draft = SalesController().createNewSale()..addProduct(product);

      expect(draft.subtotal, 25);
      expect(draft.tax, 5);
      expect(draft.total, 30);

      final violations = SaleRules.validateCompletion(
        draft: draft,
        isOnline: false,
        offlinePolicy: OfflineSalesPolicy(
          enabled: true,
          transactionValueLimit: 29,
          remainingDailyValue: 100,
          offlineSalesValidUntil: DateTime.utc(2100),
          productAllocations: {'taxed-product': 1},
        ),
      ).map((value) => value.code);
      expect(violations, contains(SaleRuleCode.offlineTaxUnsupported));
      expect(violations, contains(SaleRuleCode.offlineValueLimit));
    });

    test('checked amount helpers fail closed at the API-safe maximum', () {
      expect(checkedApiProduct(maximumSafeApiIntegerValue, 2), isNull);
      expect(
        checkedTaxAmount(maximumSafeApiIntegerValue, 10000),
        maximumSafeApiIntegerValue,
      );
      expect(checkedApiSum([maximumSafeApiIntegerValue, 1]), isNull);
    });

    test('unsafe offline total is never persisted or queued', () async {
      const product = Product(
        id: 'unsafe-total-product',
        code: 'MAX-1',
        name: 'Unsafe total product',
        commonDescription: '',
        brand: '',
        category: '',
        unit: 'EA',
        sellingPrice: maximumSafeApiIntegerValue,
        availableQuantity: 2,
        taxBasisPoints: 0,
      );
      final store = InMemoryEncryptedLocalStore();
      final controller = SalesController(store: store)..setConnectivity(false);
      final draft = controller.createNewSale();
      draft.lines.add(CartLine(product: product, quantity: 2));

      expect(draft.checkedSubtotal, isNull);
      await expectLater(
        controller.completeSale(draft),
        throwsA(
          isA<SaleRuleException>().having(
            (error) => error.violations.map((value) => value.code),
            'violations',
            contains(SaleRuleCode.amountOutsideApiRange),
          ),
        ),
      );
      expect(await store.readSales(), isEmpty);
      expect(await store.readSyncQueue(), isEmpty);
    });

    test(
      'expired offline authorization cannot persist or queue a sale',
      () async {
        final store = InMemoryEncryptedLocalStore();
        final controller = SalesController(
          store: store,
          offlinePolicy: OfflineSalesPolicy(
            enabled: true,
            transactionValueLimit: 50000000,
            remainingDailyValue: 50000000,
            offlineSalesValidUntil: DateTime.now().toUtc().subtract(
              const Duration(seconds: 1),
            ),
            productAllocations: {demoProducts.first.id: 2},
          ),
        )..setConnectivity(false);
        final draft =
            controller.createNewSale()..addProduct(demoProducts.first);

        await expectLater(
          controller.completeSale(draft),
          throwsA(
            isA<SaleRuleException>().having(
              (error) => error.violations.map((value) => value.code),
              'violations',
              contains(SaleRuleCode.offlineAuthorizationExpired),
            ),
          ),
        );
        expect(await store.readSales(), isEmpty);
        expect(await store.readSyncQueue(), isEmpty);
      },
    );

    test('stale product versions cannot be relabeled and persisted', () async {
      const currentDevice = DeviceContext(
        deviceId: 'device',
        userId: 'user',
        tenantId: 'tenant',
        attendantName: 'Attendant',
        companyId: 'company',
        companyName: 'Company',
        branchId: 'branch',
        branchName: 'Branch',
        warehouseId: 'warehouse',
        warehouseName: 'Warehouse',
        appVersion: '1.0.0+1',
        catalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
        masterDataVersion: 2,
        priceVersion: 2,
        approved: true,
      );
      final store = InMemoryEncryptedLocalStore();
      final controller = SalesController(device: currentDevice, store: store);
      final draft = controller.createNewSale()..addProduct(demoProducts.first);

      await expectLater(
        controller.completeSale(draft),
        throwsA(
          isA<SaleRuleException>().having(
            (error) => error.violations.map((value) => value.code),
            'violations',
            contains(SaleRuleCode.staleProductVersion),
          ),
        ),
      );
      expect(await store.readSales(), isEmpty);
      expect(await store.readSyncQueue(), isEmpty);
    });

    test('v1 draft is rejected after controller advances to v2', () async {
      const currentDevice = DeviceContext(
        deviceId: 'ITZ-DAR-0017',
        userId: 'usr-0024',
        tenantId: 'tenant-itemba-group',
        attendantName: 'Asha Mushi',
        companyId: 'company-itemba-trading',
        companyName: 'Itemba Trading Co. Ltd',
        branchId: 'branch-dar',
        branchName: 'Dar es Salaam',
        warehouseId: 'warehouse-dar-main',
        warehouseName: 'Dar Main Store',
        appVersion: '1.0.0+1',
        catalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
        masterDataVersion: 2,
        priceVersion: 2,
        approved: true,
      );
      final store = InMemoryEncryptedLocalStore();
      final controller = SalesController(store: store);
      final v1Draft =
          controller.createNewSale()..addProduct(demoProducts.first);
      controller.device = currentDevice;

      await expectLater(
        controller.completeSale(v1Draft),
        throwsA(
          isA<SaleRuleException>().having(
            (error) => error.violations.map((value) => value.code),
            'violations',
            contains(SaleRuleCode.staleDraftContext),
          ),
        ),
      );
      expect(await store.readSales(), isEmpty);
      expect(await store.readSyncQueue(), isEmpty);
    });

    test('draft is rejected when its catalog token has been superseded', () async {
      final store = InMemoryEncryptedLocalStore();
      final controller = SalesController(store: store);
      final draft = controller.createNewSale()..addProduct(demoProducts.first);
      controller.device = controller.device.copyWithVersions(
        catalogSnapshotToken: '00000000-0000-4000-8000-000000000010',
        masterDataVersion: controller.device.masterDataVersion,
        priceVersion: controller.device.priceVersion,
      );

      await expectLater(
        controller.completeSale(draft),
        throwsA(
          isA<SaleRuleException>().having(
            (error) => error.violations.map((value) => value.code),
            'violations',
            contains(SaleRuleCode.staleDraftContext),
          ),
        ),
      );
      expect(await store.readSales(), isEmpty);
      expect(await store.readSyncQueue(), isEmpty);
    });

    test('connection loss rejects a previously selected bank card', () async {
      final store = InMemoryEncryptedLocalStore();
      final controller = SalesController(store: store);
      final draft = controller.createNewSale()..addProduct(demoProducts.first);
      draft.paymentMethod = PaymentMethod.card;
      controller.setConnectivity(false);

      await expectLater(
        controller.completeSale(draft),
        throwsA(
          isA<SaleRuleException>().having(
            (error) => error.violations.map((value) => value.code),
            'violations',
            contains(SaleRuleCode.offlinePhysicalCashRequired),
          ),
        ),
      );
      expect(await store.readSyncQueue(), isEmpty);
    });
  });

  test('money formats TZS minor units without floating point', () {
    expect(money(8500000), 'TZS 85,000.00');
    expect(money(-105), '-TZS 1.05');
    expect(money(-8500000), '-TZS 85,000.00');
  });
}
