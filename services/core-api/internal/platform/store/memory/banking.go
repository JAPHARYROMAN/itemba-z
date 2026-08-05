package memory

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/banking"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func bankStatementKey(scope tenancy.Scope, id string) string { return scopeKey(scope) + ":" + id }

func (s *Store) SeedBankAccount(value banking.Account) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.bankAccounts[scopeKey(value.Scope)+":"+value.ID] = value
}

func (s *Store) ListBankAccounts(_ context.Context, scope tenancy.Scope, actorID string) ([]banking.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actorID, "finance.bank.read")] {
		return nil, sales.ErrForbidden
	}
	result := []banking.Account{}
	for _, value := range s.state.bankAccounts {
		if value.Scope == scope {
			result = append(result, value)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Code < result[j].Code })
	return result, nil
}

func (s *Store) ImportBankStatement(_ context.Context, statement banking.Statement, event audit.Event, message outbox.Event, idem, requestHash string) (banking.Statement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(statement.Scope, statement.ImportedBy, "finance.bank.import")] {
		return banking.Statement{}, sales.ErrForbidden
	}
	idemKey := idempotencyKey(statement.Scope, "banking.statement.import.v1", idem)
	if prior, ok := s.state.idempotencies[idemKey]; ok {
		if prior.RequestHash != requestHash {
			return banking.Statement{}, sales.ErrIdempotencyConflict
		}
		return cloneBankStatement(s.state.bankStatements[bankStatementKey(statement.Scope, prior.ResultID)]), nil
	}
	account, ok := s.state.bankAccounts[scopeKey(statement.Scope)+":"+statement.AccountID]
	if !ok {
		return banking.Statement{}, sales.ErrNotFound
	}
	if !account.Active {
		return banking.Statement{}, banking.ErrInactiveAccount
	}
	if account.Currency != statement.Currency {
		return banking.Statement{}, banking.ErrStatementImbalance
	}
	for _, existing := range s.state.bankStatements {
		if existing.Scope.TenantID == statement.Scope.TenantID && existing.Scope.CompanyID == statement.Scope.CompanyID && existing.AccountID == statement.AccountID && existing.ExternalReference == statement.ExternalReference {
			return banking.Statement{}, banking.ErrDuplicateStatement
		}
	}
	s.state.bankStatements[bankStatementKey(statement.Scope, statement.ID)] = cloneBankStatement(statement)
	s.state.audits = append(s.state.audits, event)
	s.state.outbox = append(s.state.outbox, message)
	s.state.idempotencies[idemKey] = idempotency{RequestHash: requestHash, ResultID: statement.ID}
	return cloneBankStatement(statement), nil
}

func (s *Store) ListBankStatements(_ context.Context, scope tenancy.Scope, actorID, cursor string, limit int) (banking.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actorID, "finance.bank.read")] {
		return banking.Page{}, sales.ErrForbidden
	}
	values := []banking.Statement{}
	for _, value := range s.state.bankStatements {
		if value.Scope == scope && (cursor == "" || value.ID < cursor) {
			copyValue := cloneBankStatement(value)
			copyValue.Lines = nil
			values = append(values, copyValue)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID > values[j].ID })
	page := banking.Page{Items: values}
	if len(values) > limit {
		next := values[limit-1].ID
		page.NextCursor = &next
		page.Items = values[:limit]
	}
	return page, nil
}

func (s *Store) BankStatement(_ context.Context, scope tenancy.Scope, actorID, statementID string) (banking.Statement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actorID, "finance.bank.read")] {
		return banking.Statement{}, sales.ErrForbidden
	}
	value, ok := s.state.bankStatements[bankStatementKey(scope, statementID)]
	if !ok {
		return banking.Statement{}, sales.ErrNotFound
	}
	return s.withBankCandidates(value), nil
}

func (s *Store) MatchBankStatementLine(_ context.Context, scope tenancy.Scope, actorID, statementID, lineID, journalLineID string, match banking.Match, event audit.Event, message outbox.Event, idem, requestHash string) (banking.Statement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actorID, "finance.bank.match")] {
		return banking.Statement{}, sales.ErrForbidden
	}
	idemKey := idempotencyKey(scope, "banking.statement.match.v1", idem)
	if prior, ok := s.state.idempotencies[idemKey]; ok {
		if prior.RequestHash != requestHash {
			return banking.Statement{}, sales.ErrIdempotencyConflict
		}
		return s.withBankCandidates(s.state.bankStatements[bankStatementKey(scope, prior.ResultID)]), nil
	}
	key := bankStatementKey(scope, statementID)
	statement, ok := s.state.bankStatements[key]
	if !ok {
		return banking.Statement{}, sales.ErrNotFound
	}
	if statement.Status == banking.Reconciled {
		return banking.Statement{}, banking.ErrAlreadyReconciled
	}
	lineIndex := -1
	for index, line := range statement.Lines {
		if line.ID == lineID {
			lineIndex = index
		}
		if line.Match != nil && line.Match.JournalLineID == journalLineID {
			return banking.Statement{}, banking.ErrAlreadyMatched
		}
	}
	if lineIndex < 0 {
		return banking.Statement{}, sales.ErrNotFound
	}
	if statement.Lines[lineIndex].Match != nil {
		return banking.Statement{}, banking.ErrAlreadyMatched
	}
	candidates := s.candidates(statement, statement.Lines[lineIndex].AmountMinor)
	found := false
	for _, candidate := range candidates {
		if candidate.JournalLineID == journalLineID {
			found = true
			break
		}
	}
	if !found {
		return banking.Statement{}, banking.ErrMatchMismatch
	}
	statement.Lines[lineIndex].Match = &match
	s.state.bankStatements[key] = statement
	s.state.audits = append(s.state.audits, event)
	s.state.outbox = append(s.state.outbox, message)
	s.state.idempotencies[idemKey] = idempotency{RequestHash: requestHash, ResultID: statementID}
	return s.withBankCandidates(statement), nil
}

func (s *Store) ReconcileBankStatement(_ context.Context, scope tenancy.Scope, actorID, statementID, reason string, at time.Time, event audit.Event, message outbox.Event, idem, requestHash string) (banking.Statement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actorID, "finance.bank.reconcile")] {
		return banking.Statement{}, sales.ErrForbidden
	}
	idemKey := idempotencyKey(scope, "banking.statement.reconcile.v1", idem)
	if prior, ok := s.state.idempotencies[idemKey]; ok {
		if prior.RequestHash != requestHash {
			return banking.Statement{}, sales.ErrIdempotencyConflict
		}
		return cloneBankStatement(s.state.bankStatements[bankStatementKey(scope, prior.ResultID)]), nil
	}
	key := bankStatementKey(scope, statementID)
	statement, ok := s.state.bankStatements[key]
	if !ok {
		return banking.Statement{}, sales.ErrNotFound
	}
	if statement.Status == banking.Reconciled {
		return banking.Statement{}, banking.ErrAlreadyReconciled
	}
	if statement.ImportedBy == actorID {
		return banking.Statement{}, banking.ErrSeparationOfDuties
	}
	for _, line := range statement.Lines {
		if line.Match == nil {
			return banking.Statement{}, banking.ErrUnmatchedLines
		}
	}
	statement.Status = banking.Reconciled
	statement.ReconciledBy = actorID
	statement.ReconciledAt = &at
	s.state.bankStatements[key] = statement
	s.state.audits = append(s.state.audits, event)
	s.state.outbox = append(s.state.outbox, message)
	s.state.idempotencies[idemKey] = idempotency{RequestHash: requestHash, ResultID: statementID}
	_ = reason
	return cloneBankStatement(statement), nil
}

func (s *Store) withBankCandidates(value banking.Statement) banking.Statement {
	value = cloneBankStatement(value)
	for index := range value.Lines {
		if value.Lines[index].Match == nil {
			value.Lines[index].Candidates = s.candidates(value, value.Lines[index].AmountMinor)
		}
	}
	return value
}
func (s *Store) candidates(statement banking.Statement, amount int64) []banking.Candidate {
	account, ok := s.state.bankAccounts[scopeKey(statement.Scope)+":"+statement.AccountID]
	if !ok {
		return []banking.Candidate{}
	}
	used := map[string]bool{}
	for _, other := range s.state.bankStatements {
		for _, line := range other.Lines {
			if line.Match != nil {
				used[line.Match.JournalLineID] = true
			}
		}
	}
	result := []banking.Candidate{}
	for _, journal := range s.state.journals {
		if journal.TenantID != statement.Scope.TenantID || journal.CompanyID != statement.Scope.CompanyID || journal.Currency != statement.Currency {
			continue
		}
		for index, entry := range journal.Entries {
			candidateID := fmt.Sprintf("%s:%d", journal.ID, index)
			if entry.AccountID == account.GLAccountID && entry.DebitMinor-entry.CreditMinor == amount && !used[candidateID] {
				result = append(result, banking.Candidate{JournalLineID: candidateID, JournalID: journal.ID, SourceType: journal.SourceType, SourceID: journal.SourceID, AmountMinor: amount, OccurredAt: journal.OccurredAt, Memo: entry.Memo})
			}
		}
	}
	return result
}
