# Purchase-to-asset clearing

A posted, three-way-matched supplier invoice line can now become a governed fixed-asset draft without re-entering its cost or acquisition date. The server derives the amount from the posted invoice, subtracts posted purchase returns for the same product, and rejects fully returned or previously capitalized sources.

The selected capitalization offset must be the purchased product's governed inventory account. Independent fixed-asset activation therefore posts debit fixed asset and credit purchased inventory, reclassifying the verified procurement cost without duplicating accounts payable, GRNI, cash, or supplier-ledger effects.

The fixed asset retains its source invoice and product identifiers. A database uniqueness constraint prevents the same invoice/product balance from being capitalized twice. Existing maker-checker activation, open-period validation, depreciation, disposal, tenant isolation, idempotency, audit, and outbox controls remain authoritative.
