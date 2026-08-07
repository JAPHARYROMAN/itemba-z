import 'package:flutter_test/flutter_test.dart';
import 'package:sales_mobile/application/sales_controller.dart';
import 'package:sales_mobile/data/demo_data.dart';
import 'package:sales_mobile/data/local_store.dart';
import 'package:sales_mobile/domain/connection_models.dart';
import 'package:sales_mobile/domain/models.dart';
import 'package:sales_mobile/domain/sale_rules.dart';

void main() {
  test(
    'offline completion consumes and persists remaining allocation',
    () async {
      final store = InMemoryEncryptedLocalStore();
      final allocation = DeviceAllocation(
        offlineEnabled: true,
        transactionLimitMinor: 10000000,
        dailyValueLimitMinor: 10000000,
        remainingDailyMinor: 10000000,
        offlineSalesValidUntil: DateTime.utc(2100),
        productQuantities: {demoProducts.first.id: 2},
        updatedAt: DateTime.utc(2026, 8, 4),
      );
      await store.saveAllocation(allocation);
      final controller = SalesController(
        store: store,
        deviceAllocation: allocation,
        offlinePolicy: OfflineSalesPolicy(
          enabled: true,
          transactionValueLimit: allocation.transactionLimitMinor,
          remainingDailyValue: allocation.remainingDailyMinor,
          offlineSalesValidUntil: allocation.offlineSalesValidUntil,
          productAllocations: allocation.productQuantities,
        ),
      );
      controller.setConnectivity(false);
      final first = controller.createNewSale(now: DateTime.utc(2026, 8, 4, 8));
      first.addProduct(demoProducts.first);

      await controller.completeSale(first);

      expect(controller.offlinePolicy.remainingDailyValue, 8150000);
      expect(controller.offlinePolicy.allocationFor(demoProducts.first.id), 1);
      final persisted = await store.readAllocation();
      expect(persisted?.remainingDailyMinor, 8150000);
      expect(persisted?.productQuantities[demoProducts.first.id], 1);

      final second = controller.createNewSale(now: DateTime.utc(2026, 8, 4, 9));
      second
        ..addProduct(demoProducts.first)
        ..addProduct(demoProducts.first);
      await expectLater(
        controller.completeSale(second),
        throwsA(
          isA<SaleRuleException>().having(
            (error) => error.violations.map((value) => value.code),
            'violations',
            contains(SaleRuleCode.offlineAllocationExceeded),
          ),
        ),
      );
    },
  );
}
