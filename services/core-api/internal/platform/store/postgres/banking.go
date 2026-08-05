package postgres

import (
	"context"
	"strconv"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/banking"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) ListBankAccounts(ctx context.Context, scope tenancy.Scope, actorID string) ([]banking.Account, error) {
	var result []banking.Account
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		allowed, err := tx.Authorize(ctx, scope, actorID, "finance.bank.read")
		if err != nil {
			return err
		}
		if !allowed {
			return sales.ErrForbidden
		}
		rows, err := tx.tx.Query(ctx, `SELECT id::text,code,name,account_type,currency,gl_account_id,active FROM bank_accounts WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 ORDER BY code`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID)
		if err != nil {
			return normalizeError(err)
		}
		defer rows.Close()
		for rows.Next() {
			var value banking.Account
			value.Scope = scope
			if err := rows.Scan(&value.ID, &value.Code, &value.Name, &value.Type, &value.Currency, &value.GLAccountID, &value.Active); err != nil {
				return normalizeError(err)
			}
			result = append(result, value)
		}
		return normalizeError(rows.Err())
	})
	return result, err
}

func (s *Store) ImportBankStatement(ctx context.Context, statement banking.Statement, event audit.Event, message outbox.Event, idem, requestHash string) (banking.Statement, error) {
	var result banking.Statement
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		allowed, err := tx.Authorize(ctx, statement.Scope, statement.ImportedBy, "finance.bank.import")
		if err != nil {
			return err
		}
		if !allowed {
			return sales.ErrForbidden
		}
		acquired, resultID, err := tx.ClaimIdempotency(ctx, statement.Scope, "banking.statement.import.v1", idem, requestHash)
		if err != nil {
			return err
		}
		if !acquired {
			result, err = tx.bankStatement(ctx, statement.Scope, resultID, true)
			return err
		}
		var currency string
		var active bool
		if err := tx.tx.QueryRow(ctx, `SELECT currency,active FROM bank_accounts WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND id=$5 FOR UPDATE`, statement.Scope.TenantID, statement.Scope.CompanyID, statement.Scope.BranchID, statement.Scope.WarehouseID, statement.AccountID).Scan(&currency, &active); err != nil {
			return normalizeError(err)
		}
		if !active {
			return banking.ErrInactiveAccount
		}
		if currency != statement.Currency {
			return banking.ErrStatementImbalance
		}
		var duplicate bool
		if err := tx.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bank_statements WHERE tenant_id=$1 AND company_id=$2 AND account_id=$3 AND external_reference=$4)`, statement.Scope.TenantID, statement.Scope.CompanyID, statement.AccountID, statement.ExternalReference).Scan(&duplicate); err != nil {
			return normalizeError(err)
		}
		if duplicate {
			return banking.ErrDuplicateStatement
		}
		_, err = tx.tx.Exec(ctx, `INSERT INTO bank_statements(id,tenant_id,company_id,branch_id,warehouse_id,account_id,external_reference,currency,period_start,period_end,opening_minor,closing_minor,imported_by,imported_at,correlation_id,idempotency_key,request_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`, statement.ID, statement.Scope.TenantID, statement.Scope.CompanyID, statement.Scope.BranchID, statement.Scope.WarehouseID, statement.AccountID, statement.ExternalReference, statement.Currency, statement.PeriodStart, statement.PeriodEnd, statement.OpeningMinor, statement.ClosingMinor, statement.ImportedBy, statement.ImportedAt, statement.CorrelationID, idem, requestHash)
		if err != nil {
			return normalizeError(err)
		}
		for _, line := range statement.Lines {
			if _, err := tx.tx.Exec(ctx, `INSERT INTO bank_statement_lines(id,tenant_id,company_id,statement_id,transaction_at,external_reference,description,amount_minor) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, line.ID, statement.Scope.TenantID, statement.Scope.CompanyID, statement.ID, line.TransactionAt, line.ExternalReference, line.Description, line.AmountMinor); err != nil {
				return normalizeError(err)
			}
		}
		if err := tx.AppendAuditEvent(ctx, event); err != nil {
			return err
		}
		if err := tx.AppendOutboxEvent(ctx, message); err != nil {
			return err
		}
		if err := tx.CompleteIdempotency(ctx, statement.Scope, "banking.statement.import.v1", idem, statement.ID); err != nil {
			return err
		}
		result = statement
		return nil
	})
	return result, err
}

func (s *Store) ListBankStatements(ctx context.Context, scope tenancy.Scope, actorID, cursor string, limit int) (banking.Page, error) {
	result := banking.Page{Items: []banking.Statement{}}
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		allowed, err := tx.Authorize(ctx, scope, actorID, "finance.bank.read")
		if err != nil {
			return err
		}
		if !allowed {
			return sales.ErrForbidden
		}
		rows, err := tx.tx.Query(ctx, `SELECT s.id::text,s.account_id::text,s.external_reference,s.currency,s.period_start,s.period_end,s.opening_minor,s.closing_minor,s.imported_by::text,s.imported_at,s.correlation_id::text,COALESCE(r.actor_id::text,''),r.occurred_at FROM bank_statements s LEFT JOIN bank_statement_reconciliations r ON r.tenant_id=s.tenant_id AND r.company_id=s.company_id AND r.statement_id=s.id WHERE s.tenant_id=$1 AND s.company_id=$2 AND s.branch_id=$3 AND s.warehouse_id=$4 AND ($5='' OR s.id::text<$5) ORDER BY s.id DESC LIMIT $6`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, cursor, limit+1)
		if err != nil {
			return normalizeError(err)
		}
		defer rows.Close()
		for rows.Next() {
			value, err := scanBankStatement(rows, scope)
			if err != nil {
				return err
			}
			result.Items = append(result.Items, value)
		}
		if err := rows.Err(); err != nil {
			return normalizeError(err)
		}
		if len(result.Items) > limit {
			next := result.Items[limit-1].ID
			result.NextCursor = &next
			result.Items = result.Items[:limit]
		}
		return nil
	})
	return result, err
}

func (s *Store) BankStatement(ctx context.Context, scope tenancy.Scope, actorID, statementID string) (banking.Statement, error) {
	var result banking.Statement
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		allowed, err := tx.Authorize(ctx, scope, actorID, "finance.bank.read")
		if err != nil {
			return err
		}
		if !allowed {
			return sales.ErrForbidden
		}
		result, err = tx.bankStatement(ctx, scope, statementID, true)
		return err
	})
	return result, err
}

func (s *Store) MatchBankStatementLine(ctx context.Context, scope tenancy.Scope, actorID, statementID, lineID, journalLineID string, match banking.Match, event audit.Event, message outbox.Event, idem, requestHash string) (banking.Statement, error) {
	var result banking.Statement
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		allowed, err := tx.Authorize(ctx, scope, actorID, "finance.bank.match")
		if err != nil {
			return err
		}
		if !allowed {
			return sales.ErrForbidden
		}
		acquired, resultID, err := tx.ClaimIdempotency(ctx, scope, "banking.statement.match.v1", idem, requestHash)
		if err != nil {
			return err
		}
		if !acquired {
			result, err = tx.bankStatement(ctx, scope, resultID, true)
			return err
		}
		if err := tx.lockBankStatement(ctx, scope, statementID); err != nil {
			return err
		}
		statement, err := tx.bankStatement(ctx, scope, statementID, false)
		if err != nil {
			return err
		}
		if statement.Status == banking.Reconciled {
			return banking.ErrAlreadyReconciled
		}
		var amount int64
		found := false
		for _, line := range statement.Lines {
			if line.ID == lineID {
				amount = line.AmountMinor
				found = true
				break
			}
		}
		if !found {
			return sales.ErrNotFound
		}
		journalNumeric, err := strconv.ParseInt(journalLineID, 10, 64)
		if err != nil || journalNumeric < 1 {
			return banking.ErrInvalidCommand
		}
		var ledgerAmount int64
		var currency, glAccount string
		if err := tx.tx.QueryRow(ctx, `SELECT jl.debit_minor-jl.credit_minor,j.currency,jl.account_id FROM journal_lines jl JOIN journals j ON j.id=jl.journal_id JOIN bank_accounts a ON a.tenant_id=jl.tenant_id AND a.company_id=jl.company_id AND a.gl_account_id=jl.account_id WHERE jl.tenant_id=$1 AND jl.company_id=$2 AND jl.id=$3 AND a.id=$4 FOR UPDATE OF jl`, scope.TenantID, scope.CompanyID, journalNumeric, statement.AccountID).Scan(&ledgerAmount, &currency, &glAccount); err != nil {
			return normalizeError(err)
		}
		if amount != ledgerAmount || currency != statement.Currency || glAccount == "" {
			return banking.ErrMatchMismatch
		}
		var exists bool
		if err := tx.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bank_statement_matches WHERE tenant_id=$1 AND company_id=$2 AND (statement_line_id=$3 OR journal_line_id=$4))`, scope.TenantID, scope.CompanyID, lineID, journalNumeric).Scan(&exists); err != nil {
			return normalizeError(err)
		}
		if exists {
			return banking.ErrAlreadyMatched
		}
		if _, err := tx.tx.Exec(ctx, `INSERT INTO bank_statement_matches(id,tenant_id,company_id,statement_id,statement_line_id,journal_line_id,actor_id,reason,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, match.ID, scope.TenantID, scope.CompanyID, statementID, lineID, journalNumeric, actorID, match.Reason, match.OccurredAt); err != nil {
			return normalizeError(err)
		}
		if err := tx.AppendAuditEvent(ctx, event); err != nil {
			return err
		}
		if err := tx.AppendOutboxEvent(ctx, message); err != nil {
			return err
		}
		if err := tx.CompleteIdempotency(ctx, scope, "banking.statement.match.v1", idem, statementID); err != nil {
			return err
		}
		result, err = tx.bankStatement(ctx, scope, statementID, true)
		return err
	})
	return result, err
}

func (s *Store) ReconcileBankStatement(ctx context.Context, scope tenancy.Scope, actorID, statementID, reason string, at time.Time, event audit.Event, message outbox.Event, idem, requestHash string) (banking.Statement, error) {
	var result banking.Statement
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		allowed, err := tx.Authorize(ctx, scope, actorID, "finance.bank.reconcile")
		if err != nil {
			return err
		}
		if !allowed {
			return sales.ErrForbidden
		}
		acquired, resultID, err := tx.ClaimIdempotency(ctx, scope, "banking.statement.reconcile.v1", idem, requestHash)
		if err != nil {
			return err
		}
		if !acquired {
			result, err = tx.bankStatement(ctx, scope, resultID, true)
			return err
		}
		if err := tx.lockBankStatement(ctx, scope, statementID); err != nil {
			return err
		}
		statement, err := tx.bankStatement(ctx, scope, statementID, false)
		if err != nil {
			return err
		}
		if statement.Status == banking.Reconciled {
			return banking.ErrAlreadyReconciled
		}
		if statement.ImportedBy == actorID {
			return banking.ErrSeparationOfDuties
		}
		for _, line := range statement.Lines {
			if line.Match == nil {
				return banking.ErrUnmatchedLines
			}
		}
		if _, err := tx.tx.Exec(ctx, `INSERT INTO bank_statement_reconciliations(id,tenant_id,company_id,statement_id,actor_id,reason,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, event.ID, scope.TenantID, scope.CompanyID, statementID, actorID, reason, at); err != nil {
			return normalizeError(err)
		}
		if err := tx.AppendAuditEvent(ctx, event); err != nil {
			return err
		}
		if err := tx.AppendOutboxEvent(ctx, message); err != nil {
			return err
		}
		if err := tx.CompleteIdempotency(ctx, scope, "banking.statement.reconcile.v1", idem, statementID); err != nil {
			return err
		}
		result, err = tx.bankStatement(ctx, scope, statementID, true)
		return err
	})
	return result, err
}

type rowScanner interface{ Scan(...any) error }

func scanBankStatement(row rowScanner, scope tenancy.Scope) (banking.Statement, error) {
	var value banking.Statement
	var reconciledBy string
	var reconciledAt *time.Time
	value.Scope = scope
	err := row.Scan(&value.ID, &value.AccountID, &value.ExternalReference, &value.Currency, &value.PeriodStart, &value.PeriodEnd, &value.OpeningMinor, &value.ClosingMinor, &value.ImportedBy, &value.ImportedAt, &value.CorrelationID, &reconciledBy, &reconciledAt)
	if err != nil {
		return banking.Statement{}, normalizeError(err)
	}
	value.Status = banking.Imported
	if reconciledBy != "" {
		value.Status = banking.Reconciled
		value.ReconciledBy = reconciledBy
		value.ReconciledAt = reconciledAt
	}
	return value, nil
}

func (t *transaction) bankStatement(ctx context.Context, scope tenancy.Scope, statementID string, withCandidates bool) (banking.Statement, error) {
	row := t.tx.QueryRow(ctx, `SELECT s.id::text,s.account_id::text,s.external_reference,s.currency,s.period_start,s.period_end,s.opening_minor,s.closing_minor,s.imported_by::text,s.imported_at,s.correlation_id::text,COALESCE(r.actor_id::text,''),r.occurred_at FROM bank_statements s LEFT JOIN bank_statement_reconciliations r ON r.tenant_id=s.tenant_id AND r.company_id=s.company_id AND r.statement_id=s.id WHERE s.tenant_id=$1 AND s.company_id=$2 AND s.branch_id=$3 AND s.warehouse_id=$4 AND s.id=$5`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, statementID)
	value, err := scanBankStatement(row, scope)
	if err != nil {
		return banking.Statement{}, err
	}
	rows, err := t.tx.Query(ctx, `SELECT l.id::text,l.transaction_at,l.external_reference,l.description,l.amount_minor,COALESCE(m.id::text,''),COALESCE(m.journal_line_id::text,''),COALESCE(m.actor_id::text,''),COALESCE(m.reason,''),m.occurred_at FROM bank_statement_lines l LEFT JOIN bank_statement_matches m ON m.tenant_id=l.tenant_id AND m.company_id=l.company_id AND m.statement_line_id=l.id WHERE l.tenant_id=$1 AND l.company_id=$2 AND l.statement_id=$3 ORDER BY l.transaction_at,l.id`, scope.TenantID, scope.CompanyID, statementID)
	if err != nil {
		return banking.Statement{}, normalizeError(err)
	}
	for rows.Next() {
		var line banking.StatementLine
		var matchID, journalLineID, matchActor, matchReason string
		var matchAt *time.Time
		if err := rows.Scan(&line.ID, &line.TransactionAt, &line.ExternalReference, &line.Description, &line.AmountMinor, &matchID, &journalLineID, &matchActor, &matchReason, &matchAt); err != nil {
			return banking.Statement{}, normalizeError(err)
		}
		line.Candidates = []banking.Candidate{}
		if matchID != "" {
			line.Match = &banking.Match{ID: matchID, JournalLineID: journalLineID, ActorID: matchActor, Reason: matchReason, OccurredAt: *matchAt}
		}
		value.Lines = append(value.Lines, line)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return banking.Statement{}, normalizeError(err)
	}
	rows.Close()
	if withCandidates {
		for index := range value.Lines {
			if value.Lines[index].Match != nil {
				continue
			}
			candidates, err := t.bankCandidates(ctx, value, value.Lines[index].AmountMinor)
			if err != nil {
				return banking.Statement{}, err
			}
			value.Lines[index].Candidates = candidates
		}
	}
	return value, nil
}

func (t *transaction) lockBankStatement(ctx context.Context, scope tenancy.Scope, statementID string) error {
	var id string
	err := t.tx.QueryRow(ctx, `SELECT id::text FROM bank_statements WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND id=$5 FOR UPDATE`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, statementID).Scan(&id)
	return normalizeError(err)
}

func (t *transaction) bankCandidates(ctx context.Context, statement banking.Statement, amount int64) ([]banking.Candidate, error) {
	rows, err := t.tx.Query(ctx, `SELECT jl.id::text,j.id::text,j.source_type,j.source_id::text,jl.debit_minor-jl.credit_minor,j.occurred_at,jl.memo FROM journal_lines jl JOIN journals j ON j.id=jl.journal_id JOIN bank_accounts a ON a.tenant_id=jl.tenant_id AND a.company_id=jl.company_id AND a.gl_account_id=jl.account_id LEFT JOIN bank_statement_matches m ON m.tenant_id=jl.tenant_id AND m.company_id=jl.company_id AND m.journal_line_id=jl.id WHERE jl.tenant_id=$1 AND jl.company_id=$2 AND a.id=$3 AND j.currency=$4 AND jl.debit_minor-jl.credit_minor=$5 AND j.occurred_at BETWEEN $6::timestamptz-interval '7 days' AND $7::timestamptz+interval '7 days' AND m.id IS NULL ORDER BY j.occurred_at,jl.id LIMIT 20`, statement.Scope.TenantID, statement.Scope.CompanyID, statement.AccountID, statement.Currency, amount, statement.PeriodStart, statement.PeriodEnd)
	if err != nil {
		return nil, normalizeError(err)
	}
	defer rows.Close()
	result := []banking.Candidate{}
	for rows.Next() {
		var value banking.Candidate
		if err := rows.Scan(&value.JournalLineID, &value.JournalID, &value.SourceType, &value.SourceID, &value.AmountMinor, &value.OccurredAt, &value.Memo); err != nil {
			return nil, normalizeError(err)
		}
		result = append(result, value)
	}
	return result, normalizeError(rows.Err())
}
