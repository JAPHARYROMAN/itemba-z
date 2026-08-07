import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';

import '../domain/models.dart';

class AppStrings {
  const AppStrings(this.language);

  final AppLanguage language;

  static AppStrings of(BuildContext context) =>
      Localizations.of<AppStrings>(context, AppStrings) ??
      const AppStrings(AppLanguage.english);

  static const delegate = _AppStringsDelegate();

  String t(String key, [Map<String, Object> values = const {}]) {
    var value =
        (_values[language] ?? _values[AppLanguage.english])![key] ?? key;
    for (final entry in values.entries) {
      value = value.replaceAll('{${entry.key}}', '${entry.value}');
    }
    return value;
  }

  static const _values = <AppLanguage, Map<String, String>>{
    AppLanguage.english: {
      'appName': 'ITEMBA-Z Sales',
      'salesOnly': 'Sales-only workspace',
      'home': 'Sales Home',
      'newSale': 'New Sale',
      'mySales': 'My Sales',
      'sync': 'Sync Status',
      'profile': 'Profile',
      'draftSales': 'Draft Sales',
      'pendingSync': 'Pending sync',
      'goodMorning': 'Good morning, {name}',
      'assignedTo': '{branch} · {warehouse}',
      'todaySales': "Today's sales",
      'transactions': 'Transactions',
      'startSale': 'Start a sale',
      'startSaleSubtitle': 'Cash by default · protected prices',
      'viewSalesSubtitle': 'Receipts and your transaction history',
      'syncSubtitle': 'Review queued and rejected sales',
      'draftSubtitle': 'Continue a saved cart',
      'online': 'Online',
      'offline': 'Offline',
      'protected': 'Protected',
      'serverAuthoritative': 'Server remains authoritative',
      'cash': 'Cash',
      'credit': 'Credit',
      'saleType': 'Sale type',
      'customer': 'Customer',
      'generalCustomer': 'General Customer',
      'change': 'Change',
      'continue': 'Continue',
      'back': 'Back',
      'cancel': 'Cancel',
      'saveDraft': 'Save draft',
      'selectCustomer': 'Select customer',
      'cashCustomerHelp':
          'Cash sales may use General Customer or a registered customer.',
      'creditCustomerHelp':
          'Only active, registered, credit-enabled customers are shown.',
      'creditPolicyUnavailable':
          'Credit is blocked until the server supplies authoritative overdue-policy data.',
      'creditOnlineOnly': 'Credit sales require a live server check.',
      'noEligibleCustomers': 'No eligible customers found.',
      'products': 'Products',
      'searchProducts': 'Search name, code, brand or category',
      'available': '{qty} available',
      'add': 'Add',
      'added': 'Added',
      'cart': 'Review cart',
      'cartEmpty': 'Your cart is empty',
      'cartEmptyHelp': 'Add at least one product to continue.',
      'unitPrice': 'Locked unit price',
      'total': 'Total',
      'subtotal': 'Subtotal',
      'tax': 'Tax',
      'payment': 'Payment',
      'paymentMethod': 'Payment method',
      'mobileMoney': 'Mobile money',
      'card': 'Card',
      'bankTransfer': 'Bank transfer',
      'completeSale': 'Complete sale',
      'completing': 'Completing…',
      'creditReview': 'Credit review',
      'creditLimit': 'Credit limit',
      'currentExposure': 'Current exposure',
      'availableCredit': 'Available credit',
      'overdue': 'Overdue',
      'proposedSale': 'Proposed sale',
      'expectedBalance': 'Expected balance',
      'dueDate': 'Due date',
      'allowed': 'Allowed',
      'blocked': 'Blocked',
      'receipt': 'Sale confirmation',
      'saleComplete': 'Sale completed',
      'queuedOffline': 'Saved securely and queued for sync',
      'postedOnline': 'Posted once to ITEMBA-Z',
      'receiptNumber': 'Internal receipt reference',
      'done': 'Done',
      'reprint': 'View confirmation',
      'printerStatus': 'Receipt output',
      'printerUnavailable': 'Printer not configured',
      'printerUnavailableHelp':
          'The internal confirmation remains available on this device. Physical printing is unavailable until an approved printer adapter is configured. Fiscal status is shown separately and this reference must not be represented as a TRA receipt.',
      'requestCorrection': 'Request correction',
      'ownSalesOnly': 'Only sales authorized for {name}',
      'all': 'All',
      'noSales': 'No sales yet',
      'noSalesHelp': 'Completed transactions will appear here.',
      'paid': 'Paid',
      'receivable': 'Receivable',
      'syncQueue': 'Synchronization queue',
      'syncNow': 'Sync now',
      'syncing': 'Syncing…',
      'allSynced': 'Everything is synchronized',
      'allSyncedHelp': 'There are no pending mobile transactions.',
      'pending': 'Pending Sync',
      'synced': 'Synced',
      'rejected': 'Rejected',
      'requiresReview': 'Requires Review',
      'reconciliationRequired': 'Reconciliation Required',
      'draft': 'Draft',
      'lastSync': 'Last sync',
      'never': 'Never',
      'device': 'Authorized device',
      'deviceId': 'Device ID',
      'company': 'Company',
      'branch': 'Branch',
      'warehouse': 'Warehouse',
      'masterData': 'Master data',
      'priceVersion': 'Price version',
      'offlineControls': 'Offline controls',
      'offlineEnabled': 'Offline cash enabled',
      'transactionLimit': 'Transaction limit',
      'dailyRemaining': 'Daily remaining',
      'creditDisabledOffline': 'Credit disabled offline',
      'connectionDemo': 'Connection simulator',
      'connectionDemoHelp':
          'Use this switch to verify controlled offline behaviour.',
      'connectionStatus': 'Server connection',
      'refreshMasterData': 'Refresh customers and products',
      'connection_ready': 'Connected and current',
      'connection_offlineCache': 'Offline · encrypted cache in use',
      'connection_refreshing': 'Refreshing master data…',
      'connection_error': 'Connection requires attention',
      'connection_authenticationRequired': 'Sign-in required',
      'connection_suspended': 'Device suspended',
      'connection_notConfigured': 'Not configured',
      'connection_enrolling': 'Enrolling device…',
      'fiscalStatus': 'Fiscal status',
      'fiscal_notConfigured': 'Not configured · non-fiscal reference',
      'fiscal_pending': 'Fiscalization pending',
      'fiscal_fiscalized': 'TRA fiscalized',
      'fiscal_failed': 'Fiscalization failed',
      'language': 'Language',
      'english': 'English',
      'swahili': 'Swahili',
      'securedCache': 'Encrypted cache boundary',
      'securedCacheHelp': 'Production adapter: SQLCipher + Android Keystore',
      'noDrafts': 'No saved drafts',
      'noDraftsHelp': 'Partially prepared sales can be saved here.',
      'saved': 'Draft saved',
      'correctionTitle': 'Correction request',
      'correctionHelp':
          'Completed sales cannot be edited. A manager must approve any return or cancellation.',
      'correctionControlCenter':
          'Correction submission is not available in this POS release. Give the sale and client transaction references below to a manager, who must create the governed reversal in Control Center.',
      'close': 'Close',
      'submitRequest': 'Submit request',
      'reason': 'Reason and notes',
      'requestSent': 'Request sent for manager approval.',
      'rule_generalCustomerCredit': 'General Customer cannot buy on credit.',
      'rule_inactiveCreditCustomer': 'This customer is inactive.',
      'rule_creditNotEnabled': 'This customer is not credit-enabled.',
      'rule_customerOutOfScope':
          'This customer is outside your assigned scope.',
      'rule_creditRequiresOnline':
          'Connect to ITEMBA-Z before completing a credit sale.',
      'rule_emptyCart': 'Add at least one product.',
      'rule_amountOutsideApiRange':
          'This sale amount is too large to process safely.',
      'rule_staleDraftContext':
          'This draft was opened before the app or catalog changed. Start a new sale.',
      'rule_staleProductVersion':
          'A product or price changed. Refresh the cart before completing this sale.',
      'rule_stockUnavailable': 'Requested stock is unavailable.',
      'rule_offlineCashDisabled':
          'Offline cash sales are disabled for this device.',
      'rule_offlinePhysicalCashRequired':
          'Offline sales require physical cash payment.',
      'rule_offlineTaxUnsupported':
          'Offline sales are limited to products with an authoritative zero tax rate.',
      'rule_offlineAuthorizationExpired':
          'Reconnect to ITEMBA-Z to renew offline sales authorization.',
      'rule_offlineValueLimit':
          'This sale exceeds the device offline transaction limit.',
      'rule_offlineDailyLimit':
          'This sale exceeds today’s remaining offline limit.',
      'rule_offlineAllocationExceeded':
          'This quantity exceeds the device offline stock allocation.',
      'rule_deviceNotApproved': 'This device is not approved.',
      'rule_creditLimitExceeded': 'The proposed sale exceeds available credit.',
      'rule_overdueCredit': 'This account has overdue credit and is blocked.',
    },
    AppLanguage.swahili: {
      'appName': 'ITEMBA-Z Mauzo',
      'salesOnly': 'Eneo la mauzo pekee',
      'home': 'Mwanzo wa Mauzo',
      'newSale': 'Mauzo Mapya',
      'mySales': 'Mauzo Yangu',
      'sync': 'Hali ya Usawazishaji',
      'profile': 'Wasifu',
      'draftSales': 'Rasimu za Mauzo',
      'pendingSync': 'Yanasubiri kusawazishwa',
      'goodMorning': 'Habari, {name}',
      'assignedTo': '{branch} · {warehouse}',
      'todaySales': 'Mauzo ya leo',
      'transactions': 'Miamala',
      'startSale': 'Anza mauzo',
      'startSaleSubtitle': 'Taslimu kwa chaguo-msingi · bei zinalindwa',
      'viewSalesSubtitle': 'Risiti na historia ya miamala yako',
      'syncSubtitle': 'Kagua mauzo yanayosubiri au yaliyokataliwa',
      'draftSubtitle': 'Endelea na kikapu kilichohifadhiwa',
      'online': 'Mtandaoni',
      'offline': 'Nje ya mtandao',
      'protected': 'Imelindwa',
      'serverAuthoritative': 'Seva ndiyo chanzo kikuu',
      'cash': 'Taslimu',
      'credit': 'Mkopo',
      'saleType': 'Aina ya mauzo',
      'customer': 'Mteja',
      'generalCustomer': 'Mteja wa Jumla',
      'change': 'Badilisha',
      'continue': 'Endelea',
      'back': 'Rudi',
      'cancel': 'Ghairi',
      'saveDraft': 'Hifadhi rasimu',
      'selectCustomer': 'Chagua mteja',
      'cashCustomerHelp':
          'Mauzo ya taslimu yanaweza kutumia Mteja wa Jumla au mteja aliyesajiliwa.',
      'creditCustomerHelp':
          'Wateja hai, waliosajiliwa na kuruhusiwa mkopo pekee ndio wanaoonyeshwa.',
      'creditPolicyUnavailable':
          'Mkopo umezuiwa hadi seva itoe data rasmi ya sera ya madeni yaliyochelewa.',
      'creditOnlineOnly':
          'Mauzo ya mkopo yanahitaji uhakiki wa moja kwa moja wa seva.',
      'noEligibleCustomers': 'Hakuna wateja wanaokidhi vigezo.',
      'products': 'Bidhaa',
      'searchProducts': 'Tafuta jina, msimbo, chapa au kundi',
      'available': '{qty} zinapatikana',
      'add': 'Ongeza',
      'added': 'Imeongezwa',
      'cart': 'Kagua kikapu',
      'cartEmpty': 'Kikapu chako ni tupu',
      'cartEmptyHelp': 'Ongeza angalau bidhaa moja ili kuendelea.',
      'unitPrice': 'Bei iliyofungwa',
      'total': 'Jumla',
      'subtotal': 'Jumla ndogo',
      'tax': 'Kodi',
      'payment': 'Malipo',
      'paymentMethod': 'Njia ya malipo',
      'mobileMoney': 'Pesa ya simu',
      'card': 'Kadi',
      'bankTransfer': 'Uhamisho wa benki',
      'completeSale': 'Kamilisha mauzo',
      'completing': 'Inakamilisha…',
      'creditReview': 'Mapitio ya mkopo',
      'creditLimit': 'Kikomo cha mkopo',
      'currentExposure': 'Deni la sasa',
      'availableCredit': 'Mkopo unaopatikana',
      'overdue': 'Deni lililochelewa',
      'proposedSale': 'Mauzo yanayopendekezwa',
      'expectedBalance': 'Salio linalotarajiwa',
      'dueDate': 'Tarehe ya mwisho',
      'allowed': 'Imeruhusiwa',
      'blocked': 'Imezuiwa',
      'receipt': 'Uthibitisho wa mauzo',
      'saleComplete': 'Mauzo yamekamilika',
      'queuedOffline': 'Yamehifadhiwa salama na yanasubiri usawazishaji',
      'postedOnline': 'Yamewasilishwa mara moja ITEMBA-Z',
      'receiptNumber': 'Rejea ya risiti ya ndani',
      'done': 'Imekamilika',
      'reprint': 'Angalia uthibitisho',
      'printerStatus': 'Utoaji wa risiti',
      'printerUnavailable': 'Printa haijasanidiwa',
      'printerUnavailableHelp':
          'Uthibitisho wa ndani unabaki kwenye kifaa hiki. Uchapishaji haupatikani hadi kiunganishi cha printa kilichoidhinishwa kisanidiwe. Hali ya fiskali inaonyeshwa kando na rejea hii haipaswi kuwakilishwa kama risiti ya TRA.',
      'requestCorrection': 'Omba marekebisho',
      'ownSalesOnly': 'Mauzo yaliyoidhinishwa kwa {name} pekee',
      'all': 'Yote',
      'noSales': 'Hakuna mauzo bado',
      'noSalesHelp': 'Miamala iliyokamilika itaonekana hapa.',
      'paid': 'Imelipwa',
      'receivable': 'Inadaiwa',
      'syncQueue': 'Foleni ya usawazishaji',
      'syncNow': 'Sawazisha sasa',
      'syncing': 'Inasawazisha…',
      'allSynced': 'Kila kitu kimesawazishwa',
      'allSyncedHelp': 'Hakuna miamala ya simu inayosubiri.',
      'pending': 'Inasubiri',
      'synced': 'Imesawazishwa',
      'rejected': 'Imekataliwa',
      'requiresReview': 'Inahitaji mapitio',
      'reconciliationRequired': 'Inahitaji upatanisho',
      'draft': 'Rasimu',
      'lastSync': 'Usawazishaji wa mwisho',
      'never': 'Kamwe',
      'device': 'Kifaa kilichoidhinishwa',
      'deviceId': 'Kitambulisho cha kifaa',
      'company': 'Kampuni',
      'branch': 'Tawi',
      'warehouse': 'Ghala',
      'masterData': 'Data kuu',
      'priceVersion': 'Toleo la bei',
      'offlineControls': 'Udhibiti nje ya mtandao',
      'offlineEnabled': 'Mauzo ya taslimu nje ya mtandao yameruhusiwa',
      'transactionLimit': 'Kikomo cha muamala',
      'dailyRemaining': 'Kikomo kilichobaki leo',
      'creditDisabledOffline': 'Mkopo umezuiwa nje ya mtandao',
      'connectionDemo': 'Kiigaji cha muunganisho',
      'connectionDemoHelp':
          'Tumia swichi hii kuhakiki tabia salama nje ya mtandao.',
      'connectionStatus': 'Muunganisho wa seva',
      'refreshMasterData': 'Sasisha wateja na bidhaa',
      'connection_ready': 'Imeunganishwa na imesasishwa',
      'connection_offlineCache': 'Nje ya mtandao · hifadhi salama inatumika',
      'connection_refreshing': 'Inasasisha data kuu…',
      'connection_error': 'Muunganisho unahitaji ukaguzi',
      'connection_authenticationRequired': 'Kuingia kunahitajika',
      'connection_suspended': 'Kifaa kimesimamishwa',
      'connection_notConfigured': 'Haijasanidiwa',
      'connection_enrolling': 'Inasajili kifaa…',
      'fiscalStatus': 'Hali ya fiskali',
      'fiscal_notConfigured': 'Haijasanidiwa · rejea si ya fiskali',
      'fiscal_pending': 'Ufiskalishaji unasubiri',
      'fiscal_fiscalized': 'Imefiskalishwa na TRA',
      'fiscal_failed': 'Ufiskalishaji umeshindwa',
      'language': 'Lugha',
      'english': 'Kiingereza',
      'swahili': 'Kiswahili',
      'securedCache': 'Mpaka wa hifadhi iliyosimbwa',
      'securedCacheHelp':
          'Utekelezaji wa uzalishaji: SQLCipher + Android Keystore',
      'noDrafts': 'Hakuna rasimu zilizohifadhiwa',
      'noDraftsHelp': 'Mauzo ambayo hayajakamilika yanaweza kuhifadhiwa hapa.',
      'saved': 'Rasimu imehifadhiwa',
      'correctionTitle': 'Ombi la marekebisho',
      'correctionHelp':
          'Mauzo yaliyokamilika hayawezi kuhaririwa. Meneja lazima aidhinishe marejesho au kufuta.',
      'correctionControlCenter':
          'Kutuma ombi la marekebisho hakupatikani katika toleo hili la POS. Mpe meneja rejea ya mauzo na ya muamala hapa chini; lazima atengeneze ubatilisho unaodhibitiwa katika Control Center.',
      'close': 'Funga',
      'submitRequest': 'Tuma ombi',
      'reason': 'Sababu na maelezo',
      'requestSent': 'Ombi limetumwa kwa idhini ya meneja.',
      'rule_generalCustomerCredit': 'Mteja wa Jumla hawezi kununua kwa mkopo.',
      'rule_inactiveCreditCustomer': 'Mteja huyu si hai.',
      'rule_creditNotEnabled': 'Mteja huyu hajaruhusiwa mkopo.',
      'rule_customerOutOfScope': 'Mteja huyu yuko nje ya eneo lako.',
      'rule_creditRequiresOnline':
          'Unganisha ITEMBA-Z kabla ya kukamilisha mauzo ya mkopo.',
      'rule_emptyCart': 'Ongeza angalau bidhaa moja.',
      'rule_amountOutsideApiRange':
          'Kiasi cha mauzo haya ni kikubwa mno kuchakatwa kwa usalama.',
      'rule_staleDraftContext':
          'Rasimu hii ilifunguliwa kabla programu au katalogi kubadilika. Anza mauzo mapya.',
      'rule_staleProductVersion':
          'Bidhaa au bei imebadilika. Onyesha upya kikapu kabla ya kukamilisha mauzo.',
      'rule_stockUnavailable': 'Kiasi hiki cha bidhaa hakipatikani.',
      'rule_offlineCashDisabled':
          'Mauzo ya taslimu nje ya mtandao yamezuiwa kwa kifaa hiki.',
      'rule_offlinePhysicalCashRequired':
          'Mauzo nje ya mtandao yanahitaji malipo ya fedha taslimu.',
      'rule_offlineTaxUnsupported':
          'Mauzo nje ya mtandao yanaruhusiwa kwa bidhaa zenye kiwango halali cha kodi sifuri pekee.',
      'rule_offlineAuthorizationExpired':
          'Unganisha ITEMBA-Z ili kuhuisha ruhusa ya mauzo nje ya mtandao.',
      'rule_offlineValueLimit':
          'Mauzo haya yamezidi kikomo cha muamala wa kifaa.',
      'rule_offlineDailyLimit': 'Mauzo haya yamezidi kikomo kilichobaki leo.',
      'rule_offlineAllocationExceeded':
          'Kiasi hiki kimezidi mgao wa bidhaa wa kifaa.',
      'rule_deviceNotApproved': 'Kifaa hiki hakijaidhinishwa.',
      'rule_creditLimitExceeded': 'Mauzo yamezidi mkopo unaopatikana.',
      'rule_overdueCredit': 'Akaunti ina deni lililochelewa na imezuiwa.',
    },
  };
}

class _AppStringsDelegate extends LocalizationsDelegate<AppStrings> {
  const _AppStringsDelegate();

  @override
  bool isSupported(Locale locale) =>
      const ['en', 'sw'].contains(locale.languageCode);

  @override
  Future<AppStrings> load(Locale locale) => SynchronousFuture(
    AppStrings(
      locale.languageCode == 'sw' ? AppLanguage.swahili : AppLanguage.english,
    ),
  );

  @override
  bool shouldReload(_AppStringsDelegate old) => false;
}
