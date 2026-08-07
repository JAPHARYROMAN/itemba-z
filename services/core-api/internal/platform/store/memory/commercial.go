package memory

import (
	"context"
	"sort"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/catalog"
	"github.com/itemba-z/itemba-z/services/core-api/internal/commercial"
	"github.com/itemba-z/itemba-z/services/core-api/internal/operations"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func commercialKey(scope tenancy.Scope, id string) string {
	return companyEntityKey(scope.TenantID, scope.CompanyID, id)
}

func (s *Store) CommercialWorkspace(_ context.Context, scope tenancy.Scope, actor string) (commercial.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "purchases.sourcing.read")] || !s.state.permissions[permissionKey(scope, actor, "masterdata.read")] {
		return commercial.Workspace{}, sales.ErrForbidden
	}
	w := commercial.Workspace{Revisions: []commercial.Revision{}, RFQs: []commercial.RFQ{}, Quotes: []commercial.SupplierQuote{}, Awards: []commercial.Award{}}
	for _, v := range s.state.masterRevisions {
		if v.Scope.TenantID == scope.TenantID && v.Scope.CompanyID == scope.CompanyID {
			w.Revisions = append(w.Revisions, v)
		}
	}
	for _, v := range s.state.rfqs {
		if v.Scope == scope {
			w.RFQs = append(w.RFQs, v)
		}
	}
	for _, v := range s.state.supplierQuotes {
		if v.Scope == scope {
			w.Quotes = append(w.Quotes, v)
		}
	}
	for _, v := range s.state.sourcingAwards {
		if v.Scope == scope {
			w.Awards = append(w.Awards, v)
		}
	}
	sort.Slice(w.RFQs, func(i, j int) bool { return w.RFQs[i].CreatedAt.After(w.RFQs[j].CreatedAt) })
	return w, nil
}

func (s *Store) CreateMasterRevision(_ context.Context, v commercial.Revision, idem, hash string) (commercial.Revision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	permission := "masterdata.products.manage"
	if v.EntityType == commercial.SupplierEntity {
		permission = "masterdata.suppliers.manage"
	}
	if !s.state.permissions[permissionKey(v.Scope, v.CreatedBy, permission)] {
		return v, sales.ErrForbidden
	}
	if prior, ok, err := s.memoryIdem(v.Scope, "commercial.master.create.v1", idem, hash); err != nil {
		return v, err
	} else if ok {
		return s.state.masterRevisions[commercialKey(v.Scope, prior)], nil
	}
	s.state.masterRevisions[commercialKey(v.Scope, v.ID)] = v
	s.putIdem(v.Scope, "commercial.master.create.v1", idem, hash, v.ID)
	return v, nil
}

func (s *Store) TransitionMasterRevision(_ context.Context, scope tenancy.Scope, actor, id string, to commercial.Status, _ string, idem, hash string, _ time.Time) (commercial.Revision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	permission := "masterdata.products.manage"
	if to == commercial.Active || to == commercial.Rejected {
		permission = "masterdata.approve"
	}
	if !s.state.permissions[permissionKey(scope, actor, permission)] && !s.state.permissions[permissionKey(scope, actor, "masterdata.suppliers.manage")] {
		return commercial.Revision{}, sales.ErrForbidden
	}
	k := commercialKey(scope, id)
	v, ok := s.state.masterRevisions[k]
	if !ok {
		return v, sales.ErrNotFound
	}
	op := "commercial.master.transition." + string(to) + ".v1"
	if _, hit, err := s.memoryIdem(scope, op, idem, hash); err != nil {
		return v, err
	} else if hit {
		return v, nil
	}
	valid := v.Status == commercial.Draft && to == commercial.Submitted || v.Status == commercial.Submitted && (to == commercial.Active || to == commercial.Rejected)
	if !valid {
		return v, commercial.ErrInvalidTransition
	}
	if (to == commercial.Active || to == commercial.Rejected) && v.CreatedBy == actor {
		return v, commercial.ErrSeparationOfDuties
	}
	if to == commercial.Active {
		for key, x := range s.state.masterRevisions {
			if key != k && x.EntityType == v.EntityType && x.EntityID == v.EntityID && x.Status == commercial.Active {
				x.Status = commercial.Rejected
				s.state.masterRevisions[key] = x
			}
		}
		v.ApprovedBy = actor
		if v.Supplier != nil {
			s.state.suppliers[commercialKey(scope, v.EntityID)] = operations.Supplier{ID: v.EntityID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, Code: v.Supplier.Code, Name: v.Supplier.Name, Active: v.Supplier.Active, PaymentTermsDays: v.Supplier.PaymentTermsDays}
		}
		if v.Product != nil {
			s.state.products[commercialKey(scope, v.EntityID)] = catalog.Product{ID: v.EntityID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SKU: v.Product.SKU, Name: v.Product.Name, BaseUnitCode: v.Product.BaseUnitCode, Active: v.Product.Active, Currency: v.Product.Currency, ListPriceMinor: v.Product.ListPriceMinor, StandardCostMinor: v.Product.StandardCostMinor, TaxCode: v.Product.TaxCode, RevenueAccountID: v.Product.RevenueAccountID, COGSAccountID: v.Product.COGSAccountID, InventoryAccountID: v.Product.InventoryAccountID, PriceVersion: 1, MasterDataVersion: 1}
		}
	}
	v.Status = to
	s.state.masterRevisions[k] = v
	s.putIdem(scope, op, idem, hash, id)
	return v, nil
}

func (s *Store) CreateRFQ(_ context.Context, v commercial.RFQ, idem, hash string) (commercial.RFQ, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, v.CreatedBy, "purchases.sourcing.manage")] {
		return v, sales.ErrForbidden
	}
	if p, ok, e := s.memoryIdem(v.Scope, "commercial.rfq.create.v1", idem, hash); e != nil {
		return v, e
	} else if ok {
		return s.state.rfqs[commercialKey(v.Scope, p)], nil
	}
	for _, l := range v.Lines {
		if _, ok := s.state.products[commercialKey(v.Scope, l.ProductID)]; !ok {
			return v, sales.ErrNotFound
		}
	}
	s.state.rfqs[commercialKey(v.Scope, v.ID)] = v
	s.putIdem(v.Scope, "commercial.rfq.create.v1", idem, hash, v.ID)
	return v, nil
}
func (s *Store) TransitionRFQ(_ context.Context, scope tenancy.Scope, actor, id string, to commercial.RFQStatus, _ string, idem, hash string, _ time.Time) (commercial.RFQ, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	perm := "purchases.sourcing.manage"
	if to == commercial.RFQApproved {
		perm = "purchases.sourcing.approve"
	}
	if !s.state.permissions[permissionKey(scope, actor, perm)] {
		return commercial.RFQ{}, sales.ErrForbidden
	}
	k := commercialKey(scope, id)
	v, ok := s.state.rfqs[k]
	if !ok {
		return v, sales.ErrNotFound
	}
	op := "commercial.rfq.transition." + string(to) + ".v1"
	if _, hit, e := s.memoryIdem(scope, op, idem, hash); e != nil {
		return v, e
	} else if hit {
		return v, nil
	}
	valid := v.Status == commercial.RFQDraft && (to == commercial.RFQSubmitted || to == commercial.RFQCancelled) || v.Status == commercial.RFQSubmitted && (to == commercial.RFQApproved || to == commercial.RFQCancelled)
	if !valid {
		return v, commercial.ErrInvalidTransition
	}
	if to == commercial.RFQApproved && v.CreatedBy == actor {
		return v, commercial.ErrSeparationOfDuties
	}
	v.Status = to
	if to == commercial.RFQApproved {
		v.ApprovedBy = actor
	}
	s.state.rfqs[k] = v
	s.putIdem(scope, op, idem, hash, id)
	return v, nil
}
func (s *Store) CreateSupplierQuote(_ context.Context, v commercial.SupplierQuote, idem, hash string) (commercial.SupplierQuote, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, v.CreatedBy, "purchases.sourcing.manage")] {
		return v, sales.ErrForbidden
	}
	rfq, ok := s.state.rfqs[commercialKey(v.Scope, v.RFQID)]
	if !ok {
		return v, sales.ErrNotFound
	}
	if rfq.Status != commercial.RFQApproved || rfq.Currency != v.Currency || !quoteMatches(rfq, v) {
		return v, commercial.ErrSourceMismatch
	}
	if _, ok = s.state.suppliers[commercialKey(v.Scope, v.SupplierID)]; !ok {
		return v, sales.ErrNotFound
	}
	if p, hit, e := s.memoryIdem(v.Scope, "commercial.quote.create.v1", idem, hash); e != nil {
		return v, e
	} else if hit {
		return s.state.supplierQuotes[commercialKey(v.Scope, p)], nil
	}
	s.state.supplierQuotes[commercialKey(v.Scope, v.ID)] = v
	s.putIdem(v.Scope, "commercial.quote.create.v1", idem, hash, v.ID)
	return v, nil
}
func (s *Store) SubmitSupplierQuote(_ context.Context, scope tenancy.Scope, actor, id, _ string, idem, hash string, _ time.Time) (commercial.SupplierQuote, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "purchases.sourcing.manage")] {
		return commercial.SupplierQuote{}, sales.ErrForbidden
	}
	k := commercialKey(scope, id)
	v, ok := s.state.supplierQuotes[k]
	if !ok {
		return v, sales.ErrNotFound
	}
	if v.Status != commercial.QuoteDraft {
		return v, commercial.ErrInvalidTransition
	}
	if _, hit, e := s.memoryIdem(scope, "commercial.quote.submit.v1", idem, hash); e != nil {
		return v, e
	} else if hit {
		return v, nil
	}
	v.Status = commercial.QuoteSubmitted
	s.state.supplierQuotes[k] = v
	s.putIdem(scope, "commercial.quote.submit.v1", idem, hash, id)
	return v, nil
}
func (s *Store) RFQComparison(_ context.Context, scope tenancy.Scope, actor, rfqID string) (commercial.Comparison, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "purchases.sourcing.read")] {
		return commercial.Comparison{}, sales.ErrForbidden
	}
	rfq, ok := s.state.rfqs[commercialKey(scope, rfqID)]
	if !ok {
		return commercial.Comparison{}, sales.ErrNotFound
	}
	c := commercial.Comparison{RFQ: rfq, Quotes: []commercial.ComparisonItem{}}
	for _, q := range s.state.supplierQuotes {
		if q.RFQID == rfqID && q.Scope == scope && q.Status != commercial.QuoteDraft {
			c.Quotes = append(c.Quotes, commercial.ComparisonItem{QuoteID: q.ID, SupplierID: q.SupplierID, Reference: q.Reference, TotalMinor: q.TotalMinor, DeliveryDays: q.DeliveryDays, PaymentTermsDays: q.PaymentTermsDays, ValidUntil: q.ValidUntil, Comparable: quoteMatches(rfq, q), Selected: q.Status == commercial.QuoteSelected})
		}
	}
	sort.Slice(c.Quotes, func(i, j int) bool { return c.Quotes[i].TotalMinor < c.Quotes[j].TotalMinor })
	return c, nil
}
func (s *Store) AwardRFQ(_ context.Context, v commercial.Award, idem, hash string) (commercial.Award, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, v.SelectedBy, "purchases.sourcing.approve")] {
		return v, sales.ErrForbidden
	}
	rfq, ok := s.state.rfqs[commercialKey(v.Scope, v.RFQID)]
	if !ok {
		return v, sales.ErrNotFound
	}
	q, ok := s.state.supplierQuotes[commercialKey(v.Scope, v.QuoteID)]
	if !ok {
		return v, sales.ErrNotFound
	}
	if rfq.Status != commercial.RFQApproved || q.Status != commercial.QuoteSubmitted || q.RFQID != rfq.ID || !quoteMatches(rfq, q) {
		return v, commercial.ErrSourceMismatch
	}
	if rfq.CreatedBy == v.SelectedBy {
		return v, commercial.ErrSeparationOfDuties
	}
	for _, a := range s.state.sourcingAwards {
		if a.Scope == v.Scope && a.RFQID == v.RFQID {
			return a, commercial.ErrAwardExists
		}
	}
	if p, hit, e := s.memoryIdem(v.Scope, "commercial.rfq.award.v1", idem, hash); e != nil {
		return v, e
	} else if hit {
		return s.state.sourcingAwards[commercialKey(v.Scope, p)], nil
	}
	v.SupplierID = q.SupplierID
	lines := make([]operations.DocumentLine, 0, len(q.Lines))
	for _, l := range q.Lines {
		lines = append(lines, operations.DocumentLine{ID: l.ID, ProductID: l.ProductID, Quantity: l.Quantity, UnitPriceMinor: l.UnitPriceMinor, AmountMinor: l.AmountMinor})
	}
	po := operations.Document{ID: v.PurchaseOrderID, Scope: v.Scope, Number: "PO-" + v.PurchaseOrderID[:12], Type: operations.PurchaseOrder, Status: operations.Draft, PartyType: operations.SupplierParty, PartyID: q.SupplierID, Currency: q.Currency, SubtotalMinor: q.TotalMinor, TotalMinor: q.TotalMinor, Reason: v.Reason, CreatedBy: v.SelectedBy, CreatedAt: v.SelectedAt, CorrelationID: v.ID, Lines: lines}
	s.state.operationDocuments[operationKey(v.Scope, po.ID)] = po
	q.Status = commercial.QuoteSelected
	s.state.supplierQuotes[commercialKey(v.Scope, q.ID)] = q
	rfq.Status = commercial.RFQClosed
	s.state.rfqs[commercialKey(v.Scope, rfq.ID)] = rfq
	s.state.sourcingAwards[commercialKey(v.Scope, v.ID)] = v
	s.putIdem(v.Scope, "commercial.rfq.award.v1", idem, hash, v.ID)
	return v, nil
}
func quoteMatches(r commercial.RFQ, q commercial.SupplierQuote) bool {
	if len(r.Lines) != len(q.Lines) {
		return false
	}
	m := map[string]int64{}
	for _, l := range r.Lines {
		m[l.ProductID] = l.Quantity
	}
	for _, l := range q.Lines {
		if m[l.ProductID] != l.Quantity {
			return false
		}
	}
	return true
}
