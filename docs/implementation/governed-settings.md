# Governed settings

Configuration is an immutable, effective-dated legal-company record rather than mutable application state. Categories cover sales, purchasing, inventory, finance, HR, numbering, templates, notifications, imports, and integrations. A maker creates and submits a version; a different authorized actor activates or rejects it. Active effective ranges for the same category and key cannot overlap, and superseded versions are retired rather than deleted.

Configuration values are JSON objects so specialized editors can evolve without weakening the shared approval and audit boundary. Credentials are never stored in values: integrations retain only an external secret-manager reference. Every create and transition is idempotent and emits audit/outbox evidence atomically.

Document number sequences allocate under a PostgreSQL row lock. Each allocation is unique, append-only, idempotent, and retained as evidence. The bilingual `/settings` workspace exposes the version register, approvals, retirement, sequence creation, and controlled allocation.
