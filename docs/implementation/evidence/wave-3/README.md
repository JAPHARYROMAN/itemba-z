# Wave 3 evidence

The repository-controlled Wave 3 implementation is complete. The production
exit remains blocked by provider selection, protected credentials, approved
contracts, sandbox/certification results, statutory values and accountable
professional/business signatures.

`control-set.json` is deliberately fail-closed: every mandatory external
capability remains `BLOCKED_EXTERNAL` until real evidence replaces the stated
requirement. Engineering tests and a public TRA request-signing simulator do
not constitute TRA certification.

Run `node scripts/validate-wave3-controls.mjs` to verify that the durable
delivery spine, governed operator API, bilingual UI, replay controls and
evidence classification remain present and do not claim provider approval.
