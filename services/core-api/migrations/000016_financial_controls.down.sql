SET search_path TO itembaz, public;
DROP TABLE fiscal_period_action_requests;
DROP TABLE financial_document_transitions;
DROP TABLE financial_document_lines;
DROP TABLE financial_documents;
ALTER TABLE fiscal_periods DROP CONSTRAINT fiscal_periods_scope_id_unique;
DELETE FROM permissions WHERE code IN ('finance.journals.read','finance.journals.manage','finance.journals.post','finance.cash.transfer','finance.bank.adjust','finance.periods.read','finance.periods.close','finance.periods.reopen');
