# Identity and access governance

## Control objective

Every production identity is personal, MFA-authenticated, mapped to one active
internal user, and authorized only through a time-valid role assignment in an
exact tenant, legal-company, branch and warehouse scope. Shared operational
accounts are prohibited. Provider administrators, database administrators and
cloud operators do not receive ERP business permissions implicitly.

## Joiner, mover and leaver procedure

1. The manager raises a ticket naming the person, employment/vendor status,
   required role, exact scope, start/end date and business reason.
2. The role owner checks the permission matrix and segregation-of-duties (SoD)
   conflicts. HR or vendor management verifies the relationship independently.
3. A different authorized approver approves the request. The requester cannot
   approve or execute their own access.
4. The controlled `accessctl` job records the assignment and immutable event.
   Production invokes it with `ITEMBA_ADMIN_DATABASE_URL` from managed secret
   custody; the value is never exposed to a user workstation or CI log.
5. Movers receive the approved new assignment only after incompatible old
   assignments are revoked. There is no silent scope expansion.
6. A leaver event disables the user and revokes all active assignments in one
   serializable transaction. IdP revocation and managed-device suspension are
   separate mandatory ticket tasks.
7. The ticket retains the command correlation ID. Security samples completed
   requests monthly against identity-provider and database audit evidence.

## Delegation and emergency access

- Delegation names the source user, delegate, permissions embodied by the
  delegated role, exact scope and expiry. It is never open-ended and cannot
  exceed 30 days. The delegator cannot be the approver.
- Break-glass access requires a recorded incident/change ticket, a different
  approver, forced reauthentication and the approved MFA assurance class. The
  database assignment cannot exceed two hours.
- Break-glass secret retrieval is performed only through the managed emergency
  group, alerts the security owner, and is reviewed by the next business day.
- Emergency access never permits deleting audit, ledgers, stock movements,
  posted documents, legal holds or security evidence.

## Periodic access review

Quarterly campaigns include every active assignment; payroll, treasury,
security administration, database administration and break-glass membership
are reviewed monthly. Reviewers choose `RETAIN` or `REVOKE` with a reason.
Unreviewed assignments are escalated at the due date and sensitive access is
suspended rather than automatically retained. `last_reviewed_at` is not an
authorization substitute: expiry and revocation remain independently enforced.

## Segregation-of-duties baseline

The following combinations require separate people unless a signed, dated
small-team exception defines compensating review:

| Process | Maker | Checker / incompatible authority |
| --- | --- | --- |
| Access | requester or executor | access approver/reviewer |
| Supplier payment | supplier/bill preparer | payment approver/releaser |
| Bank reconciliation | statement importer/matcher | reconciliation approver |
| Payroll | payroll preparer | payroll approver/payment releaser |
| Inventory variance | count/adjustment creator | material-variance approver |
| Period close/reopen | close preparer | reopen approver |
| Intercompany | source approver | counterparty confirmer |
| Secret rotation | key custodian | rotation approver/evidence reviewer |

Named assignments, amount thresholds, exceptions and reviewers require the
accountable business and security owners. Repository seed roles are never
production approval evidence.

## Verification

- PostgreSQL constraints enforce maker-checker, duration, reason and ticket
  evidence for governed assignments.
- API authorization rejects inactive users and revoked, not-yet-valid or
  expired assignments.
- The cross-module PostgreSQL acceptance test proves that revocation removes
  access immediately.
- OIDC access requires configured `acr`, `amr` and bounded `auth_time` claims;
  spoofed client scope headers are ignored.
