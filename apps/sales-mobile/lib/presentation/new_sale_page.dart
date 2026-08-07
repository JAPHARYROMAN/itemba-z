import 'package:flutter/material.dart';

import '../application/sales_controller.dart';
import '../core/app_strings.dart';
import '../core/theme.dart';
import '../domain/models.dart';
import '../domain/sale_rules.dart';
import 'widgets/common.dart';

class NewSalePage extends StatefulWidget {
  const NewSalePage({super.key, required this.controller, this.existingDraft});

  final SalesController controller;
  final SaleDraft? existingDraft;

  @override
  State<NewSalePage> createState() => _NewSalePageState();
}

class _NewSalePageState extends State<NewSalePage> {
  late final SaleDraft draft;
  final searchController = TextEditingController();
  int step = 0;
  bool completing = false;

  @override
  void initState() {
    super.initState();
    draft = widget.existingDraft ?? widget.controller.createNewSale();
  }

  @override
  void dispose() {
    searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      appBar: AppBar(
        title: Text(
          strings.t('newSale'),
          style: const TextStyle(fontWeight: FontWeight.w900),
        ),
        actions: [
          ConnectionPill(online: widget.controller.isOnline),
          if (draft.lines.isNotEmpty)
            TextButton(
              onPressed: _saveDraft,
              child: Text(strings.t('saveDraft')),
            ),
          const SizedBox(width: 8),
        ],
      ),
      body: Column(
        children: [
          _ProgressHeader(step: step),
          Expanded(
            child: AnimatedSwitcher(
              duration: const Duration(milliseconds: 180),
              child: KeyedSubtree(
                key: ValueKey(step),
                child: switch (step) {
                  0 => _buildCustomerStep(context),
                  1 => _buildProductsStep(context),
                  2 => _buildCartStep(context),
                  _ => _buildPaymentStep(context),
                },
              ),
            ),
          ),
          _buildFooter(context),
        ],
      ),
    );
  }

  Widget _buildCustomerStep(BuildContext context) {
    final strings = AppStrings.of(context);
    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 14, 20, 24),
      children: [
        Text(
          strings.t('saleType'),
          style: Theme.of(
            context,
          ).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w900),
        ),
        const SizedBox(height: 6),
        Text(
          draft.saleType == SaleType.cash
              ? strings.t('cashCustomerHelp')
              : strings.t('creditCustomerHelp'),
          style: const TextStyle(color: Colors.blueGrey, height: 1.4),
        ),
        const SizedBox(height: 18),
        Row(
          children: [
            Expanded(
              child: _SaleTypeCard(
                key: const Key('sale-type-cash'),
                icon: Icons.payments_outlined,
                label: strings.t('cash'),
                selected: draft.saleType == SaleType.cash,
                onTap: () => _changeSaleType(SaleType.cash),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: _SaleTypeCard(
                key: const Key('sale-type-credit'),
                icon: Icons.calendar_month_outlined,
                label: strings.t('credit'),
                selected: draft.saleType == SaleType.credit,
                onTap: () => _changeSaleType(SaleType.credit),
              ),
            ),
          ],
        ),
        const SizedBox(height: 24),
        SectionTitle(strings.t('customer')),
        const SizedBox(height: 10),
        Card(
          clipBehavior: Clip.antiAlias,
          child: InkWell(
            key: const Key('customer-selector'),
            onTap: _pickCustomer,
            child: Padding(
              padding: const EdgeInsets.all(17),
              child: Row(
                children: [
                  Container(
                    width: 48,
                    height: 48,
                    decoration: BoxDecoration(
                      color: AppTheme.mint,
                      borderRadius: BorderRadius.circular(14),
                    ),
                    child: Icon(
                      draft.customer.isGeneral
                          ? Icons.people_outline
                          : Icons.business_outlined,
                      color: AppTheme.teal,
                    ),
                  ),
                  const SizedBox(width: 13),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          draft.customer.isGeneral &&
                                  widget.controller.language ==
                                      AppLanguage.swahili
                              ? strings.t('generalCustomer')
                              : draft.customer.name,
                          style: const TextStyle(
                            fontWeight: FontWeight.w900,
                            fontSize: 16,
                          ),
                        ),
                        const SizedBox(height: 3),
                        Text(
                          draft.customer.code,
                          style: const TextStyle(color: Colors.blueGrey),
                        ),
                      ],
                    ),
                  ),
                  Text(
                    strings.t('change'),
                    style: const TextStyle(
                      color: AppTheme.teal,
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
        const SizedBox(height: 14),
        _AssignmentStrip(device: widget.controller.device),
        if (draft.saleType == SaleType.credit) ...[
          const SizedBox(height: 14),
          Container(
            padding: const EdgeInsets.all(14),
            decoration: BoxDecoration(
              color: const Color(0xFFFFF3DF),
              borderRadius: BorderRadius.circular(14),
            ),
            child: Row(
              children: [
                const Icon(Icons.wifi, color: Color(0xFFB54708)),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(
                    strings.t('creditOnlineOnly'),
                    style: const TextStyle(
                      color: Color(0xFF7A2E0E),
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ],
    );
  }

  Widget _buildProductsStep(BuildContext context) {
    final strings = AppStrings.of(context);
    final products = widget.controller.searchProducts(searchController.text);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(20, 14, 20, 12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      strings.t('products'),
                      style: Theme.of(context).textTheme.titleLarge?.copyWith(
                        fontWeight: FontWeight.w900,
                      ),
                    ),
                  ),
                  if (draft.lines.isNotEmpty)
                    Badge(
                      label: Text(
                        '${draft.lines.fold<int>(0, (sum, line) => sum + line.quantity)}',
                      ),
                      child: const Icon(
                        Icons.shopping_bag_outlined,
                        color: AppTheme.teal,
                      ),
                    ),
                ],
              ),
              const SizedBox(height: 13),
              TextField(
                key: const Key('product-search'),
                controller: searchController,
                onChanged: (_) => setState(() {}),
                decoration: InputDecoration(
                  hintText: strings.t('searchProducts'),
                  prefixIcon: const Icon(Icons.search),
                  suffixIcon:
                      searchController.text.isEmpty
                          ? null
                          : IconButton(
                            onPressed: () {
                              searchController.clear();
                              setState(() {});
                            },
                            icon: const Icon(Icons.close),
                          ),
                ),
              ),
            ],
          ),
        ),
        Expanded(
          child: ListView.separated(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 20),
            itemCount: products.length,
            separatorBuilder: (_, __) => const SizedBox(height: 10),
            itemBuilder: (_, index) {
              final product = products[index];
              final matching = draft.lines.where(
                (line) => line.product.id == product.id,
              );
              final count = matching.isEmpty ? 0 : matching.first.quantity;
              return Card(
                child: Padding(
                  padding: const EdgeInsets.all(14),
                  child: Row(
                    children: [
                      Container(
                        width: 50,
                        height: 56,
                        decoration: BoxDecoration(
                          color: const Color(0xFFEAF0F2),
                          borderRadius: BorderRadius.circular(13),
                        ),
                        child: const Icon(
                          Icons.inventory_2_outlined,
                          color: AppTheme.teal,
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              product.name,
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                              style: const TextStyle(
                                fontWeight: FontWeight.w800,
                              ),
                            ),
                            const SizedBox(height: 3),
                            Text(
                              '${product.code} · ${product.unit}',
                              style: const TextStyle(
                                color: Colors.blueGrey,
                                fontSize: 11,
                              ),
                            ),
                            const SizedBox(height: 6),
                            Row(
                              children: [
                                Text(
                                  money(product.sellingPrice),
                                  style: const TextStyle(
                                    fontWeight: FontWeight.w900,
                                    color: AppTheme.navy,
                                  ),
                                ),
                                const SizedBox(width: 8),
                                Text(
                                  strings.t('available', {
                                    'qty': product.availableQuantity,
                                  }),
                                  style: const TextStyle(
                                    color: Color(0xFF07825F),
                                    fontSize: 11,
                                  ),
                                ),
                              ],
                            ),
                          ],
                        ),
                      ),
                      const SizedBox(width: 8),
                      count == 0
                          ? FilledButton.tonal(
                            key: Key('add-${product.id}'),
                            onPressed:
                                product.availableQuantity <= 0
                                    ? null
                                    : () => setState(
                                      () => draft.addProduct(product),
                                    ),
                            style: FilledButton.styleFrom(
                              minimumSize: const Size(64, 42),
                              padding: const EdgeInsets.symmetric(
                                horizontal: 14,
                              ),
                            ),
                            child: Text(strings.t('add')),
                          )
                          : Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 12,
                              vertical: 9,
                            ),
                            decoration: BoxDecoration(
                              color: AppTheme.mint,
                              borderRadius: BorderRadius.circular(12),
                            ),
                            child: Text(
                              '$count ✓',
                              style: const TextStyle(
                                color: AppTheme.teal,
                                fontWeight: FontWeight.w900,
                              ),
                            ),
                          ),
                    ],
                  ),
                ),
              );
            },
          ),
        ),
      ],
    );
  }

  Widget _buildCartStep(BuildContext context) {
    final strings = AppStrings.of(context);
    if (draft.lines.isEmpty) {
      return EmptyState(
        icon: Icons.remove_shopping_cart_outlined,
        title: strings.t('cartEmpty'),
        message: strings.t('cartEmptyHelp'),
      );
    }
    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 14, 20, 24),
      children: [
        Text(
          strings.t('cart'),
          style: Theme.of(
            context,
          ).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w900),
        ),
        const SizedBox(height: 4),
        Text(
          strings.t('unitPrice'),
          style: const TextStyle(color: Colors.blueGrey),
        ),
        const SizedBox(height: 14),
        ...draft.lines.map(
          (line) => Padding(
            padding: const EdgeInsets.only(bottom: 10),
            child: Card(
              child: Padding(
                padding: const EdgeInsets.all(15),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            line.product.name,
                            style: const TextStyle(fontWeight: FontWeight.w800),
                          ),
                        ),
                        IconButton(
                          onPressed:
                              () => setState(
                                () => draft.removeProduct(line.product.id),
                              ),
                          icon: const Icon(
                            Icons.delete_outline,
                            color: Colors.blueGrey,
                          ),
                        ),
                      ],
                    ),
                    Text(
                      '${money(line.unitPrice)} · ${line.product.unit}',
                      style: const TextStyle(
                        color: Colors.blueGrey,
                        fontSize: 12,
                      ),
                    ),
                    const SizedBox(height: 13),
                    Row(
                      children: [
                        _QuantityButton(
                          icon: Icons.remove,
                          onTap:
                              () => setState(() {
                                if (line.quantity <= 1) {
                                  draft.removeProduct(line.product.id);
                                } else {
                                  line.setQuantity(line.quantity - 1);
                                }
                              }),
                        ),
                        SizedBox(
                          width: 44,
                          child: Text(
                            '${line.quantity}',
                            textAlign: TextAlign.center,
                            style: const TextStyle(
                              fontWeight: FontWeight.w900,
                              fontSize: 16,
                            ),
                          ),
                        ),
                        _QuantityButton(
                          icon: Icons.add,
                          onTap:
                              line.quantity < line.product.availableQuantity
                                  ? () => setState(
                                    () => line.setQuantity(line.quantity + 1),
                                  )
                                  : null,
                        ),
                        const Spacer(),
                        Text(
                          money(line.subtotal),
                          style: const TextStyle(
                            fontWeight: FontWeight.w900,
                            color: AppTheme.navy,
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
        const SizedBox(height: 4),
        Container(
          padding: const EdgeInsets.all(18),
          decoration: BoxDecoration(
            color: AppTheme.navy,
            borderRadius: BorderRadius.circular(18),
          ),
          child: Row(
            children: [
              Text(
                strings.t('total'),
                style: const TextStyle(
                  color: Color(0xFFC5D7DC),
                  fontWeight: FontWeight.w700,
                ),
              ),
              const Spacer(),
              Text(
                money(draft.total),
                style: const TextStyle(
                  color: Colors.white,
                  fontWeight: FontWeight.w900,
                  fontSize: 21,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildPaymentStep(BuildContext context) {
    final strings = AppStrings.of(context);
    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 14, 20, 24),
      children: [
        Text(
          draft.saleType == SaleType.cash
              ? strings.t('payment')
              : strings.t('creditReview'),
          style: Theme.of(
            context,
          ).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w900),
        ),
        const SizedBox(height: 14),
        if (draft.saleType == SaleType.cash)
          _buildCashPayment(context)
        else
          _buildCreditReview(context),
        const SizedBox(height: 16),
        Card(
          child: Padding(
            padding: const EdgeInsets.all(17),
            child: Column(
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        draft.customer.isGeneral &&
                                widget.controller.language ==
                                    AppLanguage.swahili
                            ? strings.t('generalCustomer')
                            : draft.customer.name,
                        style: const TextStyle(fontWeight: FontWeight.w800),
                      ),
                    ),
                    Text(
                      draft.saleType == SaleType.cash
                          ? strings.t('cash')
                          : strings.t('credit'),
                      style: const TextStyle(
                        color: AppTheme.teal,
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                  ],
                ),
                const Divider(height: 26),
                Row(
                  children: [
                    Text(
                      '${draft.lines.length} ${strings.t('products').toLowerCase()}',
                      style: const TextStyle(color: Colors.blueGrey),
                    ),
                    const Spacer(),
                    Text(
                      money(draft.total),
                      style: const TextStyle(
                        fontWeight: FontWeight.w900,
                        color: AppTheme.navy,
                        fontSize: 19,
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
        if (!widget.controller.isOnline && draft.saleType == SaleType.cash) ...[
          const SizedBox(height: 14),
          Container(
            padding: const EdgeInsets.all(14),
            decoration: BoxDecoration(
              color: const Color(0xFFFFF3DF),
              borderRadius: BorderRadius.circular(14),
            ),
            child: Row(
              children: [
                const Icon(Icons.cloud_off_outlined, color: Color(0xFFB54708)),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(
                    strings.t('queuedOffline'),
                    style: const TextStyle(
                      color: Color(0xFF7A2E0E),
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ],
    );
  }

  Widget _buildCashPayment(BuildContext context) {
    final strings = AppStrings.of(context);
    final methods =
        widget.controller.isOnline
            ? <PaymentMethod, String>{
              PaymentMethod.cash: strings.t('cash'),
              PaymentMethod.mobileMoney: strings.t('mobileMoney'),
              PaymentMethod.card: strings.t('card'),
              PaymentMethod.bankTransfer: strings.t('bankTransfer'),
            }
            : <PaymentMethod, String>{PaymentMethod.cash: strings.t('cash')};
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(17),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              strings.t('paymentMethod'),
              style: const TextStyle(fontWeight: FontWeight.w800),
            ),
            const SizedBox(height: 13),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children:
                  methods.entries
                      .map(
                        (entry) => ChoiceChip(
                          label: Text(entry.value),
                          selected: draft.paymentMethod == entry.key,
                          onSelected:
                              (_) => setState(
                                () => draft.paymentMethod = entry.key,
                              ),
                        ),
                      )
                      .toList(),
            ),
            const SizedBox(height: 18),
            Row(
              children: [
                Text(
                  strings.t('total'),
                  style: const TextStyle(color: Colors.blueGrey),
                ),
                const Spacer(),
                Text(
                  money(draft.total),
                  style: const TextStyle(
                    fontWeight: FontWeight.w900,
                    fontSize: 22,
                    color: AppTheme.navy,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildCreditReview(BuildContext context) {
    final strings = AppStrings.of(context);
    final credit = draft.customer.credit;
    if (credit == null) return const SizedBox.shrink();
    final allowed =
        widget.controller.isOnline &&
        credit.overdueAmount <= 0 &&
        draft.total <= credit.availableCredit;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(17),
        child: Column(
          children: [
            Row(
              children: [
                Container(
                  width: 38,
                  height: 38,
                  decoration: BoxDecoration(
                    color: allowed ? AppTheme.mint : const Color(0xFFFFE3E0),
                    shape: BoxShape.circle,
                  ),
                  child: Icon(
                    allowed ? Icons.verified_outlined : Icons.block_outlined,
                    color:
                        allowed
                            ? const Color(0xFF07825F)
                            : Theme.of(context).colorScheme.error,
                  ),
                ),
                const SizedBox(width: 11),
                Expanded(
                  child: Text(
                    allowed ? strings.t('allowed') : strings.t('blocked'),
                    style: TextStyle(
                      fontWeight: FontWeight.w900,
                      fontSize: 17,
                      color:
                          allowed
                              ? const Color(0xFF07825F)
                              : Theme.of(context).colorScheme.error,
                    ),
                  ),
                ),
                ConnectionPill(online: widget.controller.isOnline),
              ],
            ),
            const Divider(height: 28),
            _CreditRow(
              label: strings.t('creditLimit'),
              value: money(credit.limit),
            ),
            _CreditRow(
              label: strings.t('currentExposure'),
              value: money(credit.currentExposure),
            ),
            _CreditRow(
              label: strings.t('availableCredit'),
              value: money(credit.availableCredit),
              emphasized: true,
            ),
            _CreditRow(
              label: strings.t('overdue'),
              value: money(credit.overdueAmount),
              warning: credit.overdueAmount > 0,
            ),
            _CreditRow(
              label: strings.t('proposedSale'),
              value: money(draft.total),
            ),
            _CreditRow(
              label: strings.t('expectedBalance'),
              value: money(credit.expectedBalance(draft.total)),
              emphasized: true,
            ),
            _CreditRow(
              label: strings.t('dueDate'),
              value:
                  credit.dueDate == null
                      ? '—'
                      : '${credit.dueDate!.day}/${credit.dueDate!.month}/${credit.dueDate!.year}',
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildFooter(BuildContext context) {
    final strings = AppStrings.of(context);
    final canContinue = switch (step) {
      0 => SaleRules.isCustomerEligible(draft.customer, draft.saleType),
      1 || 2 => draft.lines.isNotEmpty,
      _ => !completing,
    };
    return Container(
      padding: EdgeInsets.fromLTRB(
        20,
        12,
        20,
        12 + MediaQuery.paddingOf(context).bottom,
      ),
      decoration: const BoxDecoration(
        color: Colors.white,
        border: Border(top: BorderSide(color: Color(0xFFE3E8EC))),
      ),
      child: Row(
        children: [
          if (step > 0) ...[
            OutlinedButton(
              onPressed: () => setState(() => step -= 1),
              child: Text(strings.t('back')),
            ),
            const SizedBox(width: 10),
          ],
          Expanded(
            child: FilledButton(
              key: Key(step == 3 ? 'complete-sale' : 'continue-sale'),
              onPressed:
                  canContinue
                      ? (step == 3
                          ? _completeSale
                          : () => setState(() => step += 1))
                      : null,
              child: Text(
                step == 3
                    ? (completing
                        ? strings.t('completing')
                        : strings.t('completeSale'))
                    : strings.t('continue'),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _changeSaleType(SaleType type) async {
    if (draft.saleType == type) return;
    if (type == SaleType.cash) {
      setState(() {
        draft.saleType = type;
        draft.customer = widget.controller.generalCustomer;
      });
      return;
    }
    final customer = await _showCustomerPicker(type);
    if (!mounted || customer == null) return;
    setState(() {
      draft.saleType = type;
      draft.customer = customer;
    });
  }

  Future<void> _pickCustomer() async {
    final customer = await _showCustomerPicker(draft.saleType);
    if (!mounted || customer == null) return;
    setState(() => draft.customer = customer);
  }

  Future<Customer?> _showCustomerPicker(SaleType type) {
    final strings = AppStrings.of(context);
    final customers = widget.controller.eligibleCustomers(type).toList();
    return showModalBottomSheet<Customer>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      builder:
          (sheetContext) => Padding(
            padding: const EdgeInsets.fromLTRB(20, 12, 20, 24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Center(
                  child: Container(
                    width: 44,
                    height: 4,
                    decoration: BoxDecoration(
                      color: Colors.black12,
                      borderRadius: BorderRadius.circular(99),
                    ),
                  ),
                ),
                const SizedBox(height: 18),
                Text(
                  strings.t('selectCustomer'),
                  style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                    fontWeight: FontWeight.w900,
                  ),
                ),
                const SizedBox(height: 5),
                Text(
                  type == SaleType.credit
                      ? strings.t('creditCustomerHelp')
                      : strings.t('cashCustomerHelp'),
                  style: const TextStyle(color: Colors.blueGrey),
                ),
                const SizedBox(height: 14),
                if (customers.isEmpty)
                  Padding(
                    padding: const EdgeInsets.symmetric(vertical: 30),
                    child: Center(
                      child: Text(
                        type == SaleType.credit
                            ? strings.t('creditPolicyUnavailable')
                            : strings.t('noEligibleCustomers'),
                        textAlign: TextAlign.center,
                      ),
                    ),
                  )
                else
                  ConstrainedBox(
                    constraints: BoxConstraints(
                      maxHeight: MediaQuery.sizeOf(context).height * .55,
                    ),
                    child: ListView.separated(
                      shrinkWrap: true,
                      itemCount: customers.length,
                      separatorBuilder: (_, __) => const Divider(height: 1),
                      itemBuilder: (_, index) {
                        final customer = customers[index];
                        return ListTile(
                          key: Key('customer-${customer.id}'),
                          contentPadding: EdgeInsets.zero,
                          onTap: () => Navigator.pop(sheetContext, customer),
                          leading: CircleAvatar(
                            backgroundColor: AppTheme.mint,
                            child: Icon(
                              customer.isGeneral
                                  ? Icons.people_outline
                                  : Icons.business_outlined,
                              color: AppTheme.teal,
                            ),
                          ),
                          title: Text(
                            customer.isGeneral &&
                                    widget.controller.language ==
                                        AppLanguage.swahili
                                ? strings.t('generalCustomer')
                                : customer.name,
                            style: const TextStyle(fontWeight: FontWeight.w800),
                          ),
                          subtitle: Text(customer.code),
                          trailing:
                              customer.credit == null
                                  ? null
                                  : Text(
                                    money(customer.credit!.availableCredit),
                                    style: const TextStyle(
                                      fontSize: 11,
                                      color: Colors.blueGrey,
                                    ),
                                  ),
                        );
                      },
                    ),
                  ),
              ],
            ),
          ),
    );
  }

  Future<void> _saveDraft() async {
    final strings = AppStrings.of(context);
    await widget.controller.saveDraft(draft);
    if (!mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(strings.t('saved'))));
    Navigator.pop(context);
  }

  Future<void> _completeSale() async {
    final strings = AppStrings.of(context);
    setState(() => completing = true);
    try {
      final sale = await widget.controller.completeSale(draft);
      if (!mounted) return;
      await Navigator.of(context).pushReplacement(
        MaterialPageRoute(
          builder:
              (_) => ReceiptPage(sale: sale, controller: widget.controller),
        ),
      );
    } on SaleRuleException catch (error) {
      if (!mounted) return;
      final messages = error.violations
          .map((violation) => strings.t('rule_${violation.code.name}'))
          .toSet()
          .join('\n');
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(messages),
          backgroundColor: Theme.of(context).colorScheme.error,
        ),
      );
    } finally {
      if (mounted) setState(() => completing = false);
    }
  }
}

class _ProgressHeader extends StatelessWidget {
  const _ProgressHeader({required this.step});
  final int step;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    final labels = [
      strings.t('customer'),
      strings.t('products'),
      strings.t('cart'),
      strings.t('payment'),
    ];
    return Container(
      color: Colors.white,
      padding: const EdgeInsets.fromLTRB(20, 8, 20, 12),
      child: Column(
        children: [
          Row(
            children: List.generate(
              4,
              (index) => Expanded(
                child: Container(
                  height: 4,
                  margin: EdgeInsets.only(right: index == 3 ? 0 : 5),
                  decoration: BoxDecoration(
                    color:
                        index <= step ? AppTheme.teal : const Color(0xFFE0E6E9),
                    borderRadius: BorderRadius.circular(99),
                  ),
                ),
              ),
            ),
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              Text(
                '${step + 1}/4',
                style: const TextStyle(
                  color: AppTheme.teal,
                  fontWeight: FontWeight.w800,
                  fontSize: 12,
                ),
              ),
              const SizedBox(width: 8),
              Text(
                labels[step],
                style: const TextStyle(
                  color: Colors.blueGrey,
                  fontWeight: FontWeight.w600,
                  fontSize: 12,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _SaleTypeCard extends StatelessWidget {
  const _SaleTypeCard({
    super.key,
    required this.icon,
    required this.label,
    required this.selected,
    required this.onTap,
  });
  final IconData icon;
  final String label;
  final bool selected;
  final VoidCallback onTap;
  @override
  Widget build(BuildContext context) => Material(
    color: selected ? AppTheme.mint : Colors.white,
    shape: RoundedRectangleBorder(
      borderRadius: BorderRadius.circular(17),
      side: BorderSide(
        color: selected ? AppTheme.teal : const Color(0xFFDDE3E8),
        width: selected ? 2 : 1,
      ),
    ),
    child: InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(17),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 19),
        child: Column(
          children: [
            Icon(
              icon,
              color: selected ? AppTheme.teal : Colors.blueGrey,
              size: 28,
            ),
            const SizedBox(height: 8),
            Text(
              label,
              style: TextStyle(
                fontWeight: FontWeight.w900,
                color: selected ? AppTheme.navy : Colors.blueGrey,
              ),
            ),
          ],
        ),
      ),
    ),
  );
}

class _AssignmentStrip extends StatelessWidget {
  const _AssignmentStrip({required this.device});
  final DeviceContext device;
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(13),
    decoration: BoxDecoration(
      color: const Color(0xFFEAF0F2),
      borderRadius: BorderRadius.circular(14),
    ),
    child: Row(
      children: [
        const Icon(Icons.lock_outline, size: 18, color: AppTheme.teal),
        const SizedBox(width: 8),
        Expanded(
          child: Text(
            '${device.companyName} · ${device.branchName} · ${device.warehouseName}',
            style: const TextStyle(
              fontSize: 11,
              color: AppTheme.navy,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
      ],
    ),
  );
}

class _QuantityButton extends StatelessWidget {
  const _QuantityButton({required this.icon, required this.onTap});
  final IconData icon;
  final VoidCallback? onTap;
  @override
  Widget build(BuildContext context) => InkWell(
    onTap: onTap,
    borderRadius: BorderRadius.circular(10),
    child: Container(
      width: 36,
      height: 36,
      decoration: BoxDecoration(
        color: const Color(0xFFEAF0F2),
        borderRadius: BorderRadius.circular(10),
      ),
      child: Icon(
        icon,
        size: 18,
        color: onTap == null ? Colors.black26 : AppTheme.navy,
      ),
    ),
  );
}

class _CreditRow extends StatelessWidget {
  const _CreditRow({
    required this.label,
    required this.value,
    this.emphasized = false,
    this.warning = false,
  });
  final String label;
  final String value;
  final bool emphasized;
  final bool warning;
  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(vertical: 6),
    child: Row(
      children: [
        Expanded(
          child: Text(label, style: const TextStyle(color: Colors.blueGrey)),
        ),
        Text(
          value,
          style: TextStyle(
            fontWeight:
                emphasized || warning ? FontWeight.w900 : FontWeight.w700,
            color:
                warning ? Theme.of(context).colorScheme.error : AppTheme.navy,
          ),
        ),
      ],
    ),
  );
}

class ReceiptPage extends StatelessWidget {
  const ReceiptPage({super.key, required this.sale, required this.controller});
  final CompletedSale sale;
  final SalesController controller;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    final synced = sale.syncStatus == SyncStatus.synced;
    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false,
        title: Text(strings.t('receipt')),
      ),
      body: ListView(
        padding: const EdgeInsets.fromLTRB(20, 14, 20, 30),
        children: [
          Center(
            child: Container(
              width: 76,
              height: 76,
              decoration: const BoxDecoration(
                color: AppTheme.mint,
                shape: BoxShape.circle,
              ),
              child: const Icon(
                Icons.check_rounded,
                color: Color(0xFF07825F),
                size: 42,
              ),
            ),
          ),
          const SizedBox(height: 16),
          Text(
            strings.t('saleComplete'),
            textAlign: TextAlign.center,
            style: Theme.of(context).textTheme.headlineSmall?.copyWith(
              fontWeight: FontWeight.w900,
              color: AppTheme.navy,
            ),
          ),
          const SizedBox(height: 5),
          Text(
            synced ? strings.t('postedOnline') : strings.t('queuedOffline'),
            textAlign: TextAlign.center,
            style: const TextStyle(color: Colors.blueGrey),
          ),
          const SizedBox(height: 22),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(20),
              child: Column(
                children: [
                  const Text(
                    'ITEMBA-Z',
                    style: TextStyle(
                      letterSpacing: 2,
                      fontWeight: FontWeight.w900,
                      color: AppTheme.navy,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    controller.device.companyName,
                    style: const TextStyle(color: Colors.blueGrey),
                  ),
                  const Divider(height: 28),
                  Row(
                    children: [
                      Text(
                        strings.t('receiptNumber'),
                        style: const TextStyle(color: Colors.blueGrey),
                      ),
                      const Spacer(),
                      Flexible(
                        child: Text(
                          synced
                              ? sale.receiptReference
                              : sale.clientTransactionId,
                          textAlign: TextAlign.end,
                          style: const TextStyle(
                            fontWeight: FontWeight.w800,
                            fontSize: 12,
                          ),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 10),
                  Row(
                    children: [
                      Text(
                        strings.t('fiscalStatus'),
                        style: const TextStyle(color: Colors.blueGrey),
                      ),
                      const Spacer(),
                      Flexible(
                        child: Text(
                          strings.t('fiscal_${sale.fiscalStatus.name}'),
                          textAlign: TextAlign.end,
                          style: TextStyle(
                            fontWeight: FontWeight.w700,
                            color:
                                sale.fiscalStatus == FiscalStatus.fiscalized
                                    ? AppTheme.teal
                                    : Colors.blueGrey,
                          ),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 10),
                  Row(
                    children: [
                      Text(
                        strings.t('customer'),
                        style: const TextStyle(color: Colors.blueGrey),
                      ),
                      const Spacer(),
                      Text(
                        sale.customer.isGeneral &&
                                controller.language == AppLanguage.swahili
                            ? strings.t('generalCustomer')
                            : sale.customer.name,
                        style: const TextStyle(fontWeight: FontWeight.w700),
                      ),
                    ],
                  ),
                  const Divider(height: 28),
                  ...sale.lines.map(
                    (line) => Padding(
                      padding: const EdgeInsets.only(bottom: 12),
                      child: Row(
                        children: [
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  line.product.name,
                                  style: const TextStyle(
                                    fontWeight: FontWeight.w700,
                                  ),
                                ),
                                Text(
                                  '${line.quantity} × ${money(line.unitPrice)}',
                                  style: const TextStyle(
                                    fontSize: 11,
                                    color: Colors.blueGrey,
                                  ),
                                ),
                              ],
                            ),
                          ),
                          Text(
                            money(line.subtotal),
                            style: const TextStyle(fontWeight: FontWeight.w700),
                          ),
                        ],
                      ),
                    ),
                  ),
                  const Divider(),
                  if (sale.subtotalMinor != null) ...[
                    _ReceiptAmountRow(
                      label: strings.t('subtotal'),
                      amount: sale.subtotalMinor!,
                    ),
                    _ReceiptAmountRow(
                      label: strings.t('tax'),
                      amount: sale.taxMinor ?? 0,
                    ),
                    const SizedBox(height: 8),
                  ],
                  Row(
                    children: [
                      Text(
                        strings.t('total'),
                        style: const TextStyle(fontWeight: FontWeight.w800),
                      ),
                      const Spacer(),
                      Text(
                        money(sale.total),
                        style: const TextStyle(
                          fontWeight: FontWeight.w900,
                          color: AppTheme.navy,
                          fontSize: 21,
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 14),
                  StatusPill(status: sale.syncStatus),
                ],
              ),
            ),
          ),
          const SizedBox(height: 18),
          OutlinedButton.icon(
            onPressed: synced ? () {} : null,
            icon: const Icon(Icons.print_outlined),
            label: Text(strings.t('reprint')),
          ),
          const SizedBox(height: 10),
          FilledButton(
            onPressed: () => Navigator.pop(context),
            child: Text(strings.t('done')),
          ),
        ],
      ),
    );
  }
}

class _ReceiptAmountRow extends StatelessWidget {
  const _ReceiptAmountRow({required this.label, required this.amount});

  final String label;
  final int amount;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: 4),
    child: Row(
      children: [
        Text(label, style: const TextStyle(color: Colors.blueGrey)),
        const Spacer(),
        Text(
          money(amount),
          style: const TextStyle(fontWeight: FontWeight.w700),
        ),
      ],
    ),
  );
}
