# Security and personal-data incident response

## Severity and activation

Anyone may raise an incident. Suspected credential exposure, unauthorized data
exposure between tenants, payroll or finance disclosure, malicious ledger activity, device theft,
ransomware, provider breach or unexplained audit gap activates the security
lead and incident commander immediately. Severity is based on confidentiality,
integrity, availability, subject harm, financial/statutory impact, scope and
ongoing attacker access—not reputational convenience.

## Procedure

1. Record detection time, reporter, affected environment and an incident ID.
2. Preserve logs, audit events, affected artifacts, hashes and time sources.
   Do not copy unnecessary personal data into the incident system.
3. Contain through identity/session revocation, device suspension, key rotation,
   egress restriction or workload isolation. Never delete evidence or edit
   posted ERP records.
4. Establish affected tenants/companies, data classes, subjects, records,
   providers, countries, time window and attacker actions.
5. Privacy/legal assesses notification duties and deadlines from verified facts;
   only authorized people notify regulators, subjects, customers or providers.
6. Eradicate the cause, restore from verified sources, reconcile ledgers, stock,
   audit and outbox, and monitor for recurrence.
7. Close only after independent evidence review, required communications,
   corrective actions, owner acceptance and a blameless lessons review.

The incident register retains decisions, timestamps, approvers, evidence links,
communications, recovery validation and corrective-action owners. Tabletop
exercises cover identity compromise, cross-tenant exposure, stolen POS,
database exfiltration, malicious dependency and processor breach before pilot.
