# Advanced finance: budgets and fixed assets

This slice completes the first two remaining Phase 6 capabilities: governed legal-company budgets and a scoped fixed-asset subledger.

## Budget controls

- A budget is created once with account/month lines in the legal company's base currency.
- Only active revenue and expense accounts are accepted.
- Draft → submitted → approved/rejected is immutable and maker-checker controlled.
- Budget-versus-actual reads actual revenue and expense activity directly from immutable journals using the company's configured business timezone.
- Creation and transitions are idempotent and emit audit and transactional-outbox evidence.

## Fixed-asset controls

- Asset identity, acquisition facts, useful life, residual value, and all posting accounts become immutable at creation.
- Draft → submitted → active capitalization requires independent approval.
- Capitalization, straight-line depreciation, and disposal each produce a balanced journal in an open fiscal period.
- Depreciation is unique per asset/month, never exceeds depreciable value, and remains append-only.
- Disposal derecognizes cost and accumulated depreciation and posts proceeds plus the calculated gain or loss.
- Posted asset effects are corrected with governed finance reversals; asset facts and depreciation records cannot be deleted.

The capitalization offset must be a governed clearing or liability account appropriate to the source acquisition. An invoice already capitalized through another posting must not be capitalized a second time. A future purchase-to-asset link will automate clearing-account selection and duplicate-source prevention.

No tax depreciation rates, statutory thresholds, or Tanzania-specific values are hardcoded. Those remain effective-dated professional configuration.
