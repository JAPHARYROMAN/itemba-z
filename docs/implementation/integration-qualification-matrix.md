# Integration qualification matrix

Updated: 2026-08-06

This matrix separates shared delivery-engine qualification from provider
certification. The shared engine is executable; each provider row remains
blocked until its versioned contract and protected sandbox evidence exist.

| Scenario | Shared-engine evidence | Provider evidence required |
| --- | --- | --- |
| Normal acceptance | Processor success and PostgreSQL fiscal acceptance tests | Signed sandbox response and reconciliation |
| Duplicate request/event | Source-event uniqueness, request hash and provider idempotency key | Provider duplicate-window response |
| Delayed response | Bounded timeout, retry schedule and leased worker | Provider latency/rate-limit evidence |
| Malformed response | JSON-object validation and fail-closed dead letter | Contract-specific schema rejection evidence |
| Provider unavailable | Circuit health, capped backoff and finite attempt budget | Approved outage exercise |
| Provider rejection | Classified failure, retained error and exception register | Provider reason-code mapping and business procedure |
| Worker termination | Expired `IN_FLIGHT` lease recovery | Protected recovery exercise |
| Manual replay | Permission-separated replay, immutable reason and expanded bounded budget | Named reviewer and reconciliation outcome |
| Cancellation/return | Versioned `SALES_RETURN` projection | Provider-specific cancellation/credit-note certification |

The current public TRA page exposes a request-signing test simulator but does
not provide enough approved contract detail in the repository to safely infer
the production authentication, payload, receipt, cancellation or certification
contract. The TRA adapter therefore remains fail-closed and unregistered.
