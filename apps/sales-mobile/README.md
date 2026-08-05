# ITEMBA-Z Sales

Android-first Flutter sales-attendant POS for ITEMBA-Z. This app intentionally
contains one business module: Sales. It does not expose customer administration,
inventory administration, purchases, finance, HR, reports, or system settings.

## Implemented shell

- English and Swahili Sales Home, New Sale, My Sales, Sync Status, drafts, and
  profile/device experiences.
- New sales default to Cash + General Customer with company, branch, warehouse,
  attendant, and time assigned automatically.
- Registered-customer metadata and a server-enforced, fail-closed mobile credit
  boundary; protected product prices, stock quantities, payments, internal receipt
  references, fiscal status, and manager-approved correction requests.
- Controlled offline rules: physical cash only, authoritative zero-rated products
  only, device approval, tax-inclusive transaction/day limits, product allocation,
  a server-issued offline validity deadline, pending/review states, and
  authoritative resynchronization when leased product/master/price facts remain
  compatible. Detected drift fails closed for operator review and reconciliation;
  non-zero tax stays online until governed historical tax snapshots are available.
- Idempotency on `device_id + client_transaction_id`; repeating a sync command
  returns the original result.

Production boot never loads demo customers or products. It requires device
enrollment, refreshes scoped customer/product data from the live versioned API,
and uses the encrypted cache only when its installed master/price versions match
the server's available versions and acknowledged app version exactly.
Enrollment returns installed and available versions, the authoritative device
binding, timezone, value limits,
an exclusive offline-sales deadline that never crosses the next tax transition,
and remaining per-product offline allocations. A downloaded cache is persisted
atomically, then explicitly acknowledged to the server before sales are enabled;
a lost acknowledgement is safe to retry after restart. Offline completion
atomically persists the sale, exact durable sync command, decremented allocation,
and draft removal. Completion rechecks the lease at one captured completion
instant and rejects draft lines whose product or price versions are stale.

All money is stored as exact TZS minor units: `100` stored units equal
`TZS 1.00`. Tax is calculated with integer half-up rounding as
`(subtotal_minor * tax_basis_points + 5000) ~/ 10000`. Checked multiplication,
tax, and aggregation reject values outside the API's exact integer range. A
corrupt or truncated success response is treated as ambiguous/manual-review and never removes the
queued command. Exhausted transport retries move the controller to offline mode
before another sale can be attempted.

Credit remains unavailable in the current Flutter entry UI. The online mobile
synchronization API now applies authoritative effective policy, reconciled ageing,
overdue tolerance, risk state, and expected exposure; offline credit remains
unconditionally prohibited. The app never assumes a missing overdue amount is
zero. Completing the online credit-entry UX is the next mobile slice.

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
Bearer credentials use a separate Android Keystore-backed secure-storage
namespace; base URL, enrollment, and policy records remain inside SQLCipher.
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
5. encrypted API connection, device enrollment, and fail-closed allocation
   records, plus persisted internal receipt/fiscal sync results.
6. finalized enrollment timezone, value-limit, and stock-allocation fields.
7. nullable tax metadata for legacy caches; missing metadata fails offline closed.
8. durable candidate device identity, written before the first enrollment call.
9. separate installed and available enrollment-version fields for install-ack.
10. persisted enrollment/allocation offline-sales deadlines; legacy rows migrate
    to an expired deadline and therefore fail closed.

Repository tests use `sqflite_common_ffi` as a test-safe SQLite engine to verify
schema creation, v1→v10 and v9→v10 migrations, exact-number rollback, and data round trips.
This test adapter does not claim encryption; Android integration/release builds
use SQLCipher.

## OpenAPI generation

The checked-in Dart subset/client/models are deterministically generated from
`../../contracts/openapi/itemba-z.v1.yaml`:

```powershell
dart run tool/generate_openapi_subset.dart
dart run tool/generate_openapi_subset.dart --check
```

The `--check` command exits non-zero when the OpenAPI SHA-256 or generated output
drifts and is the CI `generate:api` gate.

## Run and verify

```powershell
flutter pub get
flutter analyze
flutter test
flutter build apk --debug
flutter run
```

Release/profile manifests disallow cleartext traffic. Debug permits cleartext
only so a developer can opt into a local API at `localhost`, `127.0.0.1`, or the
Android emulator host `10.0.2.2`. Development identity headers are unavailable
in release mode even if a Dart define is supplied. A local debug session must
opt in explicitly:

```powershell
flutter run --dart-define=ITEMBA_Z_ALLOW_DEV_IDENTITY=true
```

The repository's seeded offline policy is bound to the fixed development device
`00000000-0000-4000-8000-000000000011`. A clean debug install can opt into that
fixture without weakening production device identity:

```powershell
flutter run --dart-define=ITEMBA_Z_ALLOW_DEV_IDENTITY=true --dart-define=ITEMBA_DEV_DEVICE_ID=00000000-0000-4000-8000-000000000011
```

`ITEMBA_DEV_DEVICE_ID` is honored only for development-header identity in a
non-release build. Bearer/production and all release builds ignore it and use a
durably generated UUID instead.

Normal production enrollment requires an HTTPS API URL and an externally
provisioned bearer credential. The profile connection control is read-only in
the live runtime; the simulator remains available only to injected demo/tests.
