# ITEMBA-Z Sales

Android-first Flutter sales-attendant POS for ITEMBA-Z. This app intentionally
contains one business module: Sales. It does not expose customer administration,
inventory administration, purchases, finance, HR, reports, or system settings.

## Implemented shell

- English and Swahili Sales Home, New Sale, My Sales, Sync Status, drafts, and
  profile/device experiences.
- New sales default to Cash + General Customer with company, branch, warehouse,
  attendant, and time assigned automatically.
- Registered-customer eligibility, online-only credit validation, credit
  exposure review, protected product prices, stock quantities, payments,
  receipts, and manager-approved correction requests.
- Controlled offline cash rules: device approval, transaction/day limits,
  product allocation, pending/review states, and authoritative resynchronization.
- Idempotency on `device_id + client_transaction_id`; repeating a sync command
  returns the original result.

The current API gateway and seed data are intentionally local so the application
is independently runnable while the OpenAPI-generated client is built. Replace
`AuthoritativeSyncGateway` with the generated API adapter without changing the
domain rules, durable queue, or UI.

## Encrypted local persistence

Production boot uses `SqlCipherLocalStore` with `sqflite_sqlcipher`. A 256-bit
passphrase is generated with `Random.secure()` on first use and stored by
`flutter_secure_storage` in an ITEMBA-Z-specific Android Keystore namespace. The
key is never hardcoded, printed, or stored beside the database. Android backup is
disabled to prevent an encrypted database being restored without its Keystore
key. The minimum Android version is API 23, required by the secure RSA-OAEP and
AES-GCM defaults.

The app fails closed if Keystore or SQLCipher initialization fails; it never
falls back to plaintext SQLite or memory storage in a production boot. The
in-memory implementation remains injectable for fast domain and widget tests.
Release signing is deliberately absent from the repository and must be injected
by the protected delivery pipeline; production artifacts must never use debug
keys.

Schema migrations are ordered and additive:

1. `cached_customers` and `cached_products`, including master-data and price
   versions.
2. `sale_drafts` and normalized draft lines with captured protected prices.
3. durable local sales, the idempotent sync queue, and authoritative sync
   results.
4. exact integer TZS minor units and whole-item quantities. The transactional
   v3→v4 migration rejects unexpected fractional legacy values rather than
   silently rounding a financial or stock amount.

Repository tests use `sqflite_common_ffi` as a test-safe SQLite engine to verify
schema creation, v1→v4 migrations, exact-number rollback, and data round trips. This test adapter does
not claim encryption; Android integration/release builds use SQLCipher.

## Run and verify

```powershell
flutter pub get
flutter analyze
flutter test
flutter build apk --debug
flutter run
```

The profile screen includes a connection simulator for demonstrating the
online-credit and offline-cash controls. It is not intended for production.
