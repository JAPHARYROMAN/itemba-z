package memory

import (
	"context"
	"sort"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/reporting"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) AccountActivity(_ context.Context, scope tenancy.Scope, actor string, from, to time.Time) ([]reporting.AccountActivity, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "reports.financial.read")] {
		return nil, "", sales.ErrForbidden
	}
	from, to = memoryReportBounds(from, to)
	values := map[string]reporting.AccountActivity{}
	for _, a := range s.state.glAccounts {
		if a.TenantID == scope.TenantID && a.CompanyID == scope.CompanyID && (a.Status == "ACTIVE" || a.Status == "INACTIVE") {
			values[a.ID] = reporting.AccountActivity{AccountID: a.ID, Code: a.Code, Name: a.Name, Type: a.Type, ParentAccountID: a.ParentAccountID}
		}
	}
	for _, j := range s.state.journals {
		if j.TenantID != scope.TenantID || j.CompanyID != scope.CompanyID || j.Currency != "TZS" || !j.OccurredAt.Before(to) {
			continue
		}
		for _, line := range j.Entries {
			v, ok := values[line.AccountID]
			if !ok {
				continue
			}
			if !from.IsZero() && j.OccurredAt.Before(from) {
				v.OpeningMinor += line.DebitMinor - line.CreditMinor
			} else {
				v.DebitMinor += line.DebitMinor
				v.CreditMinor += line.CreditMinor
			}
			values[line.AccountID] = v
		}
	}
	result := make([]reporting.AccountActivity, 0, len(values))
	for _, v := range values {
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Code < result[j].Code })
	return result, "TZS", nil
}

func (s *Store) LedgerEntries(_ context.Context, scope tenancy.Scope, actor, accountID string, from, to time.Time) (reporting.GeneralLedger, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "reports.financial.read")] {
		return reporting.GeneralLedger{}, sales.ErrForbidden
	}
	from, to = memoryReportBounds(from, to)
	account, ok := s.state.glAccounts[accountKey(scope, accountID)]
	if !ok {
		return reporting.GeneralLedger{}, sales.ErrNotFound
	}
	r := reporting.GeneralLedger{AccountID: accountID, AccountCode: account.Code, AccountName: account.Name, Currency: "TZS", Entries: []reporting.LedgerEntry{}}
	type candidate struct{ entry reporting.LedgerEntry }
	items := []candidate{}
	var numeric int64
	for _, j := range s.state.journals {
		if j.TenantID != scope.TenantID || j.CompanyID != scope.CompanyID || j.Currency != "TZS" {
			continue
		}
		for _, line := range j.Entries {
			if line.AccountID != accountID {
				continue
			}
			if j.OccurredAt.Before(from) {
				r.OpeningBalanceMinor += line.DebitMinor - line.CreditMinor
				continue
			}
			if !j.OccurredAt.Before(to) {
				continue
			}
			numeric++
			items = append(items, candidate{reporting.LedgerEntry{JournalLineID: numeric, JournalID: j.ID, OccurredAt: j.OccurredAt, SourceType: j.SourceType, SourceID: j.SourceID, Memo: line.Memo, DebitMinor: line.DebitMinor, CreditMinor: line.CreditMinor}})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].entry.OccurredAt.Before(items[j].entry.OccurredAt) })
	running := r.OpeningBalanceMinor
	for _, item := range items {
		v := item.entry
		running += v.DebitMinor - v.CreditMinor
		v.RunningBalanceMinor = running
		r.TotalDebitMinor += v.DebitMinor
		r.TotalCreditMinor += v.CreditMinor
		r.Entries = append(r.Entries, v)
	}
	r.ClosingBalanceMinor = running
	return r, nil
}

func (s *Store) CashActivity(_ context.Context, scope tenancy.Scope, actor string, from, to time.Time) (int64, int64, []reporting.CashMovement, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "reports.financial.read")] {
		return 0, 0, nil, "", sales.ErrForbidden
	}
	from, to = memoryReportBounds(from, to)
	cash := map[string]bool{}
	for _, a := range s.state.bankAccounts {
		if a.Scope.TenantID == scope.TenantID && a.Scope.CompanyID == scope.CompanyID && a.Active {
			cash[a.GLAccountID] = true
		}
	}
	for _, m := range s.state.postingMappings {
		if m.TenantID == scope.TenantID && m.CompanyID == scope.CompanyID && m.Status == "ACTIVE" && m.EffectiveFrom.Before(to) && len(m.Key) >= 8 && string(m.Key[:8]) == "PAYMENT_" {
			cash[m.AccountID] = true
		}
	}
	opening := int64(0)
	closing := int64(0)
	result := []reporting.CashMovement{}
	for _, j := range s.state.journals {
		if j.TenantID != scope.TenantID || j.CompanyID != scope.CompanyID || j.Currency != "TZS" || !j.OccurredAt.Before(to) {
			continue
		}
		amount := int64(0)
		memo := ""
		for _, line := range j.Entries {
			if cash[line.AccountID] {
				amount += line.DebitMinor - line.CreditMinor
				if memo == "" {
					memo = line.Memo
				}
			}
		}
		if j.OccurredAt.Before(from) {
			opening += amount
		} else if amount != 0 {
			result = append(result, reporting.CashMovement{JournalID: j.ID, OccurredAt: j.OccurredAt, SourceType: j.SourceType, SourceID: j.SourceID, Memo: memo, AmountMinor: amount})
		}
		closing += amount
	}
	sort.Slice(result, func(i, j int) bool { return result[i].OccurredAt.Before(result[j].OccurredAt) })
	return opening, closing, result, "TZS", nil
}

func (s *Store) DashboardControls(_ context.Context, scope tenancy.Scope, actor string) (int64, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "dashboard.read")] {
		return 0, "", sales.ErrForbidden
	}
	var count int64
	for _, document := range s.state.operationDocuments {
		if document.Scope == scope && string(document.Status) == "SUBMITTED" {
			count++
		}
	}
	for _, document := range s.state.financialDocuments {
		if document.Scope == scope && string(document.Status) == "SUBMITTED" {
			count++
		}
	}
	return count, "Africa/Dar_es_Salaam", nil
}

func memoryReportBounds(from, to time.Time) (time.Time, time.Time) {
	zone, _ := time.LoadLocation("Africa/Dar_es_Salaam")
	local := func(value time.Time) time.Time {
		if value.IsZero() {
			return value
		}
		year, month, date := value.Date()
		return time.Date(year, month, date, 0, 0, 0, 0, zone).UTC()
	}
	return local(from), local(to)
}

func (s *Store) SaveReportExport(_ context.Context, scope tenancy.Scope, actor string, artifact reporting.ExportArtifact, idem, hash string) (reporting.ExportArtifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "reports.financial.export")] {
		return reporting.ExportArtifact{}, sales.ErrForbidden
	}
	ik := idempotencyKey(scope, "report.export.v1", idem)
	if prior, ok := s.state.idempotencies[ik]; ok {
		if prior.RequestHash != hash {
			return reporting.ExportArtifact{}, sales.ErrIdempotencyConflict
		}
		return s.state.reportExports[mappingKey(scope, prior.ResultID)], nil
	}
	s.state.reportExports[mappingKey(scope, artifact.ID)] = artifact
	s.state.idempotencies[ik] = idempotency{RequestHash: hash, ResultID: artifact.ID}
	appendFinancialEvidence(s.state, scope, actor, "report.exported", "report_export", artifact.ID, artifact.GeneratedAt)
	return artifact, nil
}
