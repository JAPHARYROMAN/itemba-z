# Release 1 product specification

## Control Center shell

Every authenticated screen presents the current tenant, legal company, branch, financial period, language, global search, approval inbox, notifications, help, and profile. Navigation is limited to three levels and is permission-shaped.

Primary modules are Dashboard, Customers, Suppliers, Sales, Purchases, Inventory, Finance, Human Resources, Reports, and Settings. Every transactional record exposes status, totals, scope, source links, approval history, audit timeline, and permitted actions.

## Mobile Sales flow

```text
Sales Home
  -> New Sale (Cash + General Customer by default)
  -> Select an eligible registered customer when needed
  -> Add authorized products at server-controlled prices
  -> Validate stock and price
  -> Record payment or perform online credit validation
  -> Complete once
  -> Issue or print receipt
  -> View immutable result in My Sales
```

Credit sales require an online, active, registered, credit-enabled customer. General Customer is cash-only. Offline cash sales require an enabled policy, encrypted cache, device and attendant limits, stock allocation, and idempotent synchronization.

## Shared action language

| English | Swahili | Meaning |
| --- | --- | --- |
| Create | Unda | Save a draft business document |
| Submit | Wasilisha | Request validation or approval |
| Approve | Idhinisha | Record authorized approval |
| Post | Chapisha | Commit immutable ledger effects |
| Reverse | Batilisha kwa marekebisho | Create a linked correcting transaction |

Translations are message keys, not duplicated business logic. Financial terminology must receive professional bilingual review before release.
