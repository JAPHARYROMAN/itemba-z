import 'package:flutter/material.dart';

import '../application/sales_controller.dart';
import '../core/app_strings.dart';
import '../core/theme.dart';
import '../domain/models.dart';
import 'new_sale_page.dart';
import 'widgets/common.dart';

class SalesShell extends StatefulWidget {
  const SalesShell({super.key, required this.controller});

  final SalesController controller;

  @override
  State<SalesShell> createState() => _SalesShellState();
}

class _SalesShellState extends State<SalesShell> {
  int selectedIndex = 0;

  void openNewSale([SaleDraft? draft]) {
    Navigator.of(context).push(
      MaterialPageRoute(
        builder:
            (_) => NewSalePage(
              controller: widget.controller,
              existingDraft: draft,
            ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    final pages = [
      SalesHomePage(
        controller: widget.controller,
        onNewSale: openNewSale,
        onSelectTab: (value) => setState(() => selectedIndex = value),
        onDrafts:
            () => Navigator.of(context).push(
              MaterialPageRoute(
                builder:
                    (_) => DraftSalesPage(
                      controller: widget.controller,
                      onOpenDraft: openNewSale,
                    ),
              ),
            ),
      ),
      MySalesPage(controller: widget.controller),
      SyncStatusPage(controller: widget.controller),
      ProfilePage(controller: widget.controller),
    ];

    return Scaffold(
      body: SafeArea(
        child: IndexedStack(index: selectedIndex, children: pages),
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: selectedIndex,
        onDestinationSelected: (value) => setState(() => selectedIndex = value),
        destinations: [
          NavigationDestination(
            icon: const Icon(Icons.storefront_outlined),
            selectedIcon: const Icon(Icons.storefront),
            label: strings.t('home'),
          ),
          NavigationDestination(
            icon: const Icon(Icons.receipt_long_outlined),
            selectedIcon: const Icon(Icons.receipt_long),
            label: strings.t('mySales'),
          ),
          NavigationDestination(
            icon: Badge(
              isLabelVisible: widget.controller.pendingCount > 0,
              label: Text('${widget.controller.pendingCount}'),
              child: const Icon(Icons.sync_outlined),
            ),
            selectedIcon: const Icon(Icons.sync),
            label: strings.t('sync'),
          ),
          NavigationDestination(
            icon: const Icon(Icons.person_outline),
            selectedIcon: const Icon(Icons.person),
            label: strings.t('profile'),
          ),
        ],
      ),
    );
  }
}

class PageHeader extends StatelessWidget {
  const PageHeader({
    super.key,
    required this.controller,
    required this.title,
    this.subtitle,
  });

  final SalesController controller;
  final String title;
  final String? subtitle;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 16, 20, 14),
      child: Row(
        children: [
          Container(
            width: 42,
            height: 42,
            decoration: BoxDecoration(
              color: AppTheme.navy,
              borderRadius: BorderRadius.circular(13),
            ),
            child: const Center(
              child: Text(
                'Z',
                style: TextStyle(
                  color: Colors.white,
                  fontWeight: FontWeight.w900,
                  fontSize: 21,
                ),
              ),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: Theme.of(context).textTheme.titleLarge?.copyWith(
                    fontWeight: FontWeight.w900,
                    color: AppTheme.navy,
                  ),
                ),
                if (subtitle != null)
                  Text(
                    subtitle!,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      color: Colors.blueGrey,
                      fontSize: 12,
                    ),
                  ),
              ],
            ),
          ),
          ConnectionPill(online: controller.isOnline),
          const SizedBox(width: 4),
          IconButton(
            tooltip: AppStrings.of(context).t('language'),
            onPressed: controller.toggleLanguage,
            icon: Text(
              controller.language == AppLanguage.english ? 'SW' : 'EN',
              style: const TextStyle(
                fontWeight: FontWeight.w900,
                color: AppTheme.teal,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class SalesHomePage extends StatelessWidget {
  const SalesHomePage({
    super.key,
    required this.controller,
    required this.onNewSale,
    required this.onSelectTab,
    required this.onDrafts,
  });

  final SalesController controller;
  final ValueChanged<SaleDraft?> onNewSale;
  final ValueChanged<int> onSelectTab;
  final VoidCallback onDrafts;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Column(
      children: [
        PageHeader(
          controller: controller,
          title: strings.t('appName'),
          subtitle: strings.t('salesOnly'),
        ),
        Expanded(
          child: ListView(
            padding: const EdgeInsets.fromLTRB(20, 4, 20, 28),
            children: [
              Text(
                strings.t('goodMorning', {
                  'name': controller.device.attendantName.split(' ').first,
                }),
                style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                  fontWeight: FontWeight.w900,
                  color: AppTheme.navy,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                strings.t('assignedTo', {
                  'branch': controller.device.branchName,
                  'warehouse': controller.device.warehouseName,
                }),
                style: const TextStyle(color: Colors.blueGrey),
              ),
              const SizedBox(height: 20),
              Container(
                padding: const EdgeInsets.all(22),
                decoration: BoxDecoration(
                  gradient: const LinearGradient(
                    colors: [AppTheme.navy, Color(0xFF124D5B)],
                    begin: Alignment.topLeft,
                    end: Alignment.bottomRight,
                  ),
                  borderRadius: BorderRadius.circular(24),
                ),
                child: Row(
                  children: [
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            strings.t('todaySales'),
                            style: const TextStyle(
                              color: Color(0xFFB9CED5),
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                          const SizedBox(height: 8),
                          Text(
                            money(controller.todayTotal),
                            style: const TextStyle(
                              color: Colors.white,
                              fontWeight: FontWeight.w900,
                              fontSize: 27,
                            ),
                          ),
                          const SizedBox(height: 8),
                          Text(
                            '${controller.sales.length} ${strings.t('transactions').toLowerCase()}',
                            style: const TextStyle(color: Color(0xFFD4E4E7)),
                          ),
                        ],
                      ),
                    ),
                    Container(
                      width: 62,
                      height: 62,
                      decoration: BoxDecoration(
                        color: Colors.white.withValues(alpha: 0.12),
                        shape: BoxShape.circle,
                      ),
                      child: const Icon(
                        Icons.trending_up_rounded,
                        color: Colors.white,
                        size: 32,
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 18),
              FilledButton.icon(
                key: const Key('home-new-sale'),
                onPressed: () => onNewSale(null),
                icon: const Icon(Icons.add_shopping_cart),
                label: Text(strings.t('newSale')),
              ),
              const SizedBox(height: 22),
              GridView.count(
                crossAxisCount: 2,
                shrinkWrap: true,
                physics: const NeverScrollableScrollPhysics(),
                crossAxisSpacing: 12,
                mainAxisSpacing: 12,
                childAspectRatio: 1.08,
                children: [
                  _HomeAction(
                    icon: Icons.receipt_long_outlined,
                    color: const Color(0xFF2962A8),
                    title: strings.t('mySales'),
                    subtitle: strings.t('viewSalesSubtitle'),
                    onTap: () => onSelectTab(1),
                  ),
                  _HomeAction(
                    icon: Icons.sync_problem_outlined,
                    color: const Color(0xFFB54708),
                    title: strings.t('pendingSync'),
                    subtitle: strings.t('syncSubtitle'),
                    badge: controller.pendingCount,
                    onTap: () => onSelectTab(2),
                  ),
                  _HomeAction(
                    icon: Icons.edit_note_rounded,
                    color: AppTheme.teal,
                    title: strings.t('draftSales'),
                    subtitle: strings.t('draftSubtitle'),
                    badge: controller.drafts.length,
                    onTap: onDrafts,
                  ),
                  _HomeAction(
                    icon: Icons.verified_user_outlined,
                    color: const Color(0xFF07825F),
                    title: strings.t('protected'),
                    subtitle: strings.t('serverAuthoritative'),
                    onTap: () => onSelectTab(3),
                  ),
                ],
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _HomeAction extends StatelessWidget {
  const _HomeAction({
    required this.icon,
    required this.color,
    required this.title,
    required this.subtitle,
    required this.onTap,
    this.badge = 0,
  });

  final IconData icon;
  final Color color;
  final String title;
  final String subtitle;
  final VoidCallback onTap;
  final int badge;

  @override
  Widget build(BuildContext context) {
    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Container(
                    width: 38,
                    height: 38,
                    decoration: BoxDecoration(
                      color: color.withValues(alpha: .1),
                      borderRadius: BorderRadius.circular(11),
                    ),
                    child: Icon(icon, color: color),
                  ),
                  const Spacer(),
                  if (badge > 0)
                    CircleAvatar(
                      radius: 13,
                      backgroundColor: color,
                      child: Text(
                        '$badge',
                        style: const TextStyle(
                          color: Colors.white,
                          fontWeight: FontWeight.w800,
                          fontSize: 11,
                        ),
                      ),
                    ),
                ],
              ),
              const Spacer(),
              Text(
                title,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  fontWeight: FontWeight.w800,
                  color: AppTheme.navy,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                subtitle,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  fontSize: 11,
                  color: Colors.blueGrey,
                  height: 1.25,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class MySalesPage extends StatefulWidget {
  const MySalesPage({super.key, required this.controller});

  final SalesController controller;

  @override
  State<MySalesPage> createState() => _MySalesPageState();
}

class _MySalesPageState extends State<MySalesPage> {
  SaleType? filter;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    final sales =
        widget.controller.sales
            .where((sale) => filter == null || sale.saleType == filter)
            .toList();
    return Column(
      children: [
        PageHeader(
          controller: widget.controller,
          title: strings.t('mySales'),
          subtitle: strings.t('ownSalesOnly', {
            'name': widget.controller.device.attendantName,
          }),
        ),
        Padding(
          padding: const EdgeInsets.fromLTRB(20, 0, 20, 10),
          child: Row(
            children: [
              _FilterChip(
                label: strings.t('all'),
                selected: filter == null,
                onTap: () => setState(() => filter = null),
              ),
              const SizedBox(width: 8),
              _FilterChip(
                label: strings.t('cash'),
                selected: filter == SaleType.cash,
                onTap: () => setState(() => filter = SaleType.cash),
              ),
              const SizedBox(width: 8),
              _FilterChip(
                label: strings.t('credit'),
                selected: filter == SaleType.credit,
                onTap: () => setState(() => filter = SaleType.credit),
              ),
            ],
          ),
        ),
        Expanded(
          child:
              sales.isEmpty
                  ? EmptyState(
                    icon: Icons.receipt_long_outlined,
                    title: strings.t('noSales'),
                    message: strings.t('noSalesHelp'),
                  )
                  : ListView.separated(
                    padding: const EdgeInsets.fromLTRB(20, 8, 20, 24),
                    itemCount: sales.length,
                    separatorBuilder: (_, __) => const SizedBox(height: 10),
                    itemBuilder:
                        (context, index) => _SaleTile(
                          sale: sales[index],
                          controller: widget.controller,
                        ),
                  ),
        ),
      ],
    );
  }
}

class _FilterChip extends StatelessWidget {
  const _FilterChip({
    required this.label,
    required this.selected,
    required this.onTap,
  });
  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return ChoiceChip(
      label: Text(label),
      selected: selected,
      onSelected: (_) => onTap(),
    );
  }
}

class _SaleTile extends StatelessWidget {
  const _SaleTile({required this.sale, required this.controller});
  final CompletedSale sale;
  final SalesController controller;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap:
            () => showModalBottomSheet<void>(
              context: context,
              isScrollControlled: true,
              useSafeArea: true,
              builder:
                  (_) => SaleDetailsSheet(sale: sale, controller: controller),
            ),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: AppTheme.mint,
                  borderRadius: BorderRadius.circular(13),
                ),
                child: Icon(
                  sale.saleType == SaleType.cash
                      ? Icons.payments_outlined
                      : Icons.schedule_outlined,
                  color: AppTheme.teal,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      sale.customer.isGeneral &&
                              controller.language == AppLanguage.swahili
                          ? strings.t('generalCustomer')
                          : sale.customer.name,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontWeight: FontWeight.w800),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      '${compactDate(sale.createdAt)} · ${sale.saleType == SaleType.cash ? strings.t('cash') : strings.t('credit')}',
                      style: const TextStyle(
                        color: Colors.blueGrey,
                        fontSize: 12,
                      ),
                    ),
                    const SizedBox(height: 7),
                    StatusPill(status: sale.syncStatus),
                  ],
                ),
              ),
              const SizedBox(width: 8),
              Column(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Text(
                    money(sale.total),
                    style: const TextStyle(
                      fontWeight: FontWeight.w900,
                      color: AppTheme.navy,
                    ),
                  ),
                  const SizedBox(height: 5),
                  Text(
                    sale.receiptNumber,
                    style: const TextStyle(
                      color: Colors.blueGrey,
                      fontSize: 10,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class SaleDetailsSheet extends StatelessWidget {
  const SaleDetailsSheet({
    super.key,
    required this.sale,
    required this.controller,
  });
  final CompletedSale sale;
  final SalesController controller;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Padding(
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
          const SizedBox(height: 20),
          Row(
            children: [
              Expanded(
                child: Text(
                  strings.t('receipt'),
                  style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                    fontWeight: FontWeight.w900,
                  ),
                ),
              ),
              StatusPill(status: sale.syncStatus),
            ],
          ),
          const SizedBox(height: 5),
          Text(
            sale.receiptNumber,
            style: const TextStyle(color: Colors.blueGrey),
          ),
          const Divider(height: 30),
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
                          style: const TextStyle(fontWeight: FontWeight.w700),
                        ),
                        Text(
                          '${line.quantity} × ${money(line.unitPrice)}',
                          style: const TextStyle(
                            color: Colors.blueGrey,
                            fontSize: 12,
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
          Row(
            children: [
              Text(
                strings.t('total'),
                style: const TextStyle(fontWeight: FontWeight.w700),
              ),
              const Spacer(),
              Text(
                money(sale.total),
                style: const TextStyle(
                  fontWeight: FontWeight.w900,
                  fontSize: 20,
                  color: AppTheme.navy,
                ),
              ),
            ],
          ),
          const SizedBox(height: 20),
          Row(
            children: [
              Expanded(
                child: OutlinedButton.icon(
                  onPressed:
                      sale.syncStatus == SyncStatus.synced ? () {} : null,
                  icon: const Icon(Icons.print_outlined),
                  label: Text(strings.t('reprint')),
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: FilledButton.tonalIcon(
                  onPressed: () => _showCorrection(context),
                  icon: const Icon(Icons.assignment_return_outlined),
                  label: Text(strings.t('requestCorrection')),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  void _showCorrection(BuildContext context) {
    final strings = AppStrings.of(context);
    showDialog<void>(
      context: context,
      builder:
          (dialogContext) => AlertDialog(
            title: Text(strings.t('correctionTitle')),
            content: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(strings.t('correctionHelp')),
                const SizedBox(height: 14),
                TextField(
                  maxLines: 3,
                  decoration: InputDecoration(labelText: strings.t('reason')),
                ),
              ],
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(dialogContext),
                child: Text(strings.t('cancel')),
              ),
              FilledButton(
                onPressed: () {
                  Navigator.pop(dialogContext);
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text(strings.t('requestSent'))),
                  );
                },
                child: Text(strings.t('submitRequest')),
              ),
            ],
          ),
    );
  }
}

class SyncStatusPage extends StatelessWidget {
  const SyncStatusPage({super.key, required this.controller});
  final SalesController controller;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    final queued =
        controller.sales
            .where((sale) => sale.syncStatus != SyncStatus.synced)
            .toList();
    return Column(
      children: [
        PageHeader(
          controller: controller,
          title: strings.t('sync'),
          subtitle: strings.t('serverAuthoritative'),
        ),
        Padding(
          padding: const EdgeInsets.fromLTRB(20, 2, 20, 14),
          child: Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  Container(
                    width: 44,
                    height: 44,
                    decoration: BoxDecoration(
                      color:
                          controller.pendingCount == 0
                              ? AppTheme.mint
                              : const Color(0xFFFFEED8),
                      borderRadius: BorderRadius.circular(13),
                    ),
                    child: Icon(
                      controller.pendingCount == 0
                          ? Icons.cloud_done_outlined
                          : Icons.cloud_upload_outlined,
                      color:
                          controller.pendingCount == 0
                              ? AppTheme.teal
                              : const Color(0xFFB54708),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          '${controller.pendingCount} ${strings.t('pendingSync').toLowerCase()}',
                          style: const TextStyle(fontWeight: FontWeight.w800),
                        ),
                        Text(
                          '${strings.t('lastSync')}: ${controller.lastSyncAt == null ? strings.t('never') : compactDate(controller.lastSyncAt!)}',
                          style: const TextStyle(
                            color: Colors.blueGrey,
                            fontSize: 12,
                          ),
                        ),
                      ],
                    ),
                  ),
                  FilledButton.tonal(
                    onPressed:
                        controller.isOnline &&
                                !controller.isSyncing &&
                                controller.pendingCount > 0
                            ? controller.synchronizePending
                            : null,
                    child: Text(
                      controller.isSyncing
                          ? strings.t('syncing')
                          : strings.t('syncNow'),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
        Expanded(
          child:
              queued.isEmpty
                  ? EmptyState(
                    icon: Icons.cloud_done_outlined,
                    title: strings.t('allSynced'),
                    message: strings.t('allSyncedHelp'),
                  )
                  : ListView.separated(
                    padding: const EdgeInsets.fromLTRB(20, 0, 20, 24),
                    itemCount: queued.length,
                    separatorBuilder: (_, __) => const SizedBox(height: 10),
                    itemBuilder:
                        (_, index) => _SaleTile(
                          sale: queued[index],
                          controller: controller,
                        ),
                  ),
        ),
      ],
    );
  }
}

class ProfilePage extends StatelessWidget {
  const ProfilePage({super.key, required this.controller});
  final SalesController controller;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    final device = controller.device;
    return Column(
      children: [
        PageHeader(
          controller: controller,
          title: strings.t('profile'),
          subtitle: device.attendantName,
        ),
        Expanded(
          child: ListView(
            padding: const EdgeInsets.fromLTRB(20, 2, 20, 28),
            children: [
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(18),
                  child: Row(
                    children: [
                      const CircleAvatar(
                        radius: 28,
                        backgroundColor: AppTheme.navy,
                        child: Text(
                          'AM',
                          style: TextStyle(
                            color: Colors.white,
                            fontWeight: FontWeight.w800,
                          ),
                        ),
                      ),
                      const SizedBox(width: 14),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              device.attendantName,
                              style: const TextStyle(
                                fontWeight: FontWeight.w900,
                                fontSize: 17,
                              ),
                            ),
                            Text(
                              strings.t('salesOnly'),
                              style: const TextStyle(color: Colors.blueGrey),
                            ),
                            const SizedBox(height: 5),
                            const StatusPill(status: SyncStatus.synced),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 18),
              SectionTitle(strings.t('device')),
              const SizedBox(height: 10),
              Card(
                child: Column(
                  children: [
                    _InfoRow(
                      label: strings.t('deviceId'),
                      value: device.deviceId,
                      icon: Icons.phone_android_outlined,
                    ),
                    _InfoRow(
                      label: strings.t('company'),
                      value: device.companyName,
                      icon: Icons.business_outlined,
                    ),
                    _InfoRow(
                      label: strings.t('branch'),
                      value: device.branchName,
                      icon: Icons.store_outlined,
                    ),
                    _InfoRow(
                      label: strings.t('warehouse'),
                      value: device.warehouseName,
                      icon: Icons.warehouse_outlined,
                      showDivider: false,
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 18),
              SectionTitle(strings.t('offlineControls')),
              const SizedBox(height: 10),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    children: [
                      SwitchListTile(
                        contentPadding: EdgeInsets.zero,
                        value: controller.isOnline,
                        onChanged: controller.setConnectivity,
                        title: Text(
                          strings.t('connectionDemo'),
                          style: const TextStyle(fontWeight: FontWeight.w800),
                        ),
                        subtitle: Text(strings.t('connectionDemoHelp')),
                        secondary: Icon(
                          controller.isOnline ? Icons.wifi : Icons.wifi_off,
                          color:
                              controller.isOnline
                                  ? AppTheme.teal
                                  : const Color(0xFFB54708),
                        ),
                      ),
                      const Divider(),
                      _CompactFact(
                        label: strings.t('offlineEnabled'),
                        value: controller.offlinePolicy.enabled ? '✓' : '—',
                      ),
                      _CompactFact(
                        label: strings.t('transactionLimit'),
                        value: money(
                          controller.offlinePolicy.transactionValueLimit,
                        ),
                      ),
                      _CompactFact(
                        label: strings.t('dailyRemaining'),
                        value: money(
                          controller.offlinePolicy.remainingDailyValue,
                        ),
                      ),
                      _CompactFact(
                        label: strings.t('creditDisabledOffline'),
                        value: '✓',
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 18),
              SectionTitle(strings.t('language')),
              const SizedBox(height: 10),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(10),
                  child: SegmentedButton<AppLanguage>(
                    expandedInsets: EdgeInsets.zero,
                    segments: [
                      ButtonSegment(
                        value: AppLanguage.english,
                        label: Text(strings.t('english')),
                      ),
                      ButtonSegment(
                        value: AppLanguage.swahili,
                        label: Text(strings.t('swahili')),
                      ),
                    ],
                    selected: {controller.language},
                    onSelectionChanged:
                        (value) => controller.setLanguage(value.first),
                  ),
                ),
              ),
              const SizedBox(height: 18),
              Card(
                child: ListTile(
                  leading: const Icon(Icons.lock_outline, color: AppTheme.teal),
                  title: Text(
                    strings.t('securedCache'),
                    style: const TextStyle(fontWeight: FontWeight.w800),
                  ),
                  subtitle: Text(strings.t('securedCacheHelp')),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _InfoRow extends StatelessWidget {
  const _InfoRow({
    required this.label,
    required this.value,
    required this.icon,
    this.showDivider = true,
  });
  final String label;
  final String value;
  final IconData icon;
  final bool showDivider;
  @override
  Widget build(BuildContext context) => Column(
    children: [
      ListTile(
        leading: Icon(icon, color: AppTheme.teal),
        title: Text(
          label,
          style: const TextStyle(color: Colors.blueGrey, fontSize: 12),
        ),
        subtitle: Text(
          value,
          style: const TextStyle(
            color: AppTheme.navy,
            fontWeight: FontWeight.w700,
          ),
        ),
      ),
      if (showDivider) const Divider(height: 1, indent: 56),
    ],
  );
}

class _CompactFact extends StatelessWidget {
  const _CompactFact({required this.label, required this.value});
  final String label;
  final String value;
  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(vertical: 5),
    child: Row(
      children: [
        Expanded(
          child: Text(label, style: const TextStyle(color: Colors.blueGrey)),
        ),
        Text(
          value,
          style: const TextStyle(
            fontWeight: FontWeight.w800,
            color: AppTheme.navy,
          ),
        ),
      ],
    ),
  );
}

class DraftSalesPage extends StatelessWidget {
  const DraftSalesPage({
    super.key,
    required this.controller,
    required this.onOpenDraft,
  });
  final SalesController controller;
  final ValueChanged<SaleDraft> onOpenDraft;
  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      appBar: AppBar(title: Text(strings.t('draftSales'))),
      body:
          controller.drafts.isEmpty
              ? EmptyState(
                icon: Icons.edit_note,
                title: strings.t('noDrafts'),
                message: strings.t('noDraftsHelp'),
              )
              : ListView.separated(
                padding: const EdgeInsets.all(20),
                itemCount: controller.drafts.length,
                separatorBuilder: (_, __) => const SizedBox(height: 10),
                itemBuilder: (_, index) {
                  final draft = controller.drafts[index];
                  return Card(
                    child: ListTile(
                      onTap: () {
                        Navigator.pop(context);
                        onOpenDraft(draft);
                      },
                      leading: const Icon(
                        Icons.edit_note,
                        color: AppTheme.teal,
                      ),
                      title: Text(
                        draft.customer.name,
                        style: const TextStyle(fontWeight: FontWeight.w800),
                      ),
                      subtitle: Text(
                        '${draft.lines.length} · ${compactDate(draft.createdAt)}',
                      ),
                      trailing: Text(
                        money(draft.total),
                        style: const TextStyle(fontWeight: FontWeight.w800),
                      ),
                    ),
                  );
                },
              ),
    );
  }
}
