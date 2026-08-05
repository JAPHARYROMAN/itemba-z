package memory

import (
	"context"
	"sort"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/inventory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/operations"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func operationKey(scope tenancy.Scope, id string) string { return scopeKey(scope) + ":" + id }

func (s *Store) CreateOperationDocument(_ context.Context, document operations.Document, event audit.Event, message outbox.Event, idem, requestHash string) (operations.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(document.Scope, document.CreatedBy, memoryOperationPermission(document.Type))] {
		return operations.Document{}, sales.ErrForbidden
	}
	idemKey := idempotencyKey(document.Scope, "operations.document.create.v1", idem)
	if prior, ok := s.state.idempotencies[idemKey]; ok {
		if prior.RequestHash != requestHash {
			return operations.Document{}, sales.ErrIdempotencyConflict
		}
		return cloneOperation(s.state.operationDocuments[operationKey(document.Scope, prior.ResultID)]), nil
	}
	key := operationKey(document.Scope, document.ID)
	if _, exists := s.state.operationDocuments[key]; exists {
		return operations.Document{}, sales.ErrIdempotencyConflict
	}
	s.state.operationDocuments[key] = cloneOperation(document)
	s.state.audits = append(s.state.audits, event)
	s.state.outbox = append(s.state.outbox, message)
	s.state.idempotencies[idemKey] = idempotency{RequestHash: requestHash, ResultID: document.ID}
	return cloneOperation(document), nil
}

func (s *Store) TransitionOperationDocument(_ context.Context, scope tenancy.Scope, actorID, documentID string, target operations.Status, reason, _ string, _ string, effects operations.EffectIDs, event audit.Event, message outbox.Event, idem, requestHash string, at time.Time) (operations.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key,document, ok := memoryOperationAccessible(s.state,scope,documentID)
	if !ok {
		return operations.Document{}, sales.ErrNotFound
	}
	if !s.state.permissions[permissionKey(scope, actorID, memoryOperationPermission(document.Type))] {
		return operations.Document{}, sales.ErrForbidden
	}
	idemKey := idempotencyKey(scope, "operations.document.transition.v1", idem)
	if prior, found := s.state.idempotencies[idemKey]; found {
		if prior.RequestHash != requestHash {
			return operations.Document{}, sales.ErrIdempotencyConflict
		}
		return cloneOperation(s.state.operationDocuments[key]), nil
	}
	if !memoryAllowedTransition(document, target) {
		return operations.Document{}, operations.ErrInvalidTransition
	}
	if target == operations.Approved && document.CreatedBy == actorID {
		return operations.Document{}, operations.ErrSeparationOfDuties
	}
	if target == operations.Approved && document.Type == operations.SalesOrder {
		tx := &transaction{state: s.state}
		if len(effects.ReserveIDs) < len(document.Lines) {
			return operations.Document{}, operations.ErrInvalidCommand
		}
		for _, line := range document.Lines {
			available, err := tx.AvailableStock(context.Background(), scope, line.ProductID)
			if err != nil {
				return operations.Document{}, err
			}
			if available < line.Quantity {
				return operations.Document{}, operations.ErrInsufficientStock
			}
		}
		for index, line := range document.Lines {
			s.state.reservations = append(s.state.reservations, inventory.Movement{ID: effects.ReserveIDs[index], TenantID: scope.TenantID, CompanyID: scope.CompanyID, BranchID: scope.BranchID, WarehouseID: scope.WarehouseID, ProductID: line.ProductID, SourceType: "SALES_ORDER", SourceID: document.ID, Quantity: line.Quantity, OccurredAt: at})
		}
	}
	document.Status = target
	switch target {
	case operations.Submitted:
		document.SubmittedAt = &at
	case operations.Approved:
		document.ApprovedAt = &at
	case operations.Posted, operations.Dispatched, operations.Received, operations.Closed:
		document.PostedAt = &at
	}
	s.state.operationDocuments[key] = document
	s.state.audits = append(s.state.audits, event)
	s.state.outbox = append(s.state.outbox, message)
	s.state.idempotencies[idemKey] = idempotency{RequestHash: requestHash, ResultID: document.ID}
	return cloneOperation(document), nil
}

func (s *Store) OperationDocument(_ context.Context, scope tenancy.Scope, actorID, documentID string) (operations.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actorID, "operations.read")] {
		return operations.Document{}, sales.ErrForbidden
	}
	_,value, ok := memoryOperationAccessible(s.state,scope,documentID)
	if !ok {
		return operations.Document{}, sales.ErrNotFound
	}
	return cloneOperation(value), nil
}

func (s *Store) ListOperationDocuments(_ context.Context, scope tenancy.Scope, actorID string, kind operations.DocumentType, cursor string, limit int) (operations.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actorID, "operations.read")] {
		return operations.Page{}, sales.ErrForbidden
	}
	values := make([]operations.Document, 0)
	for _, value := range s.state.operationDocuments {
		accessible:=value.Scope==scope||(value.Type==operations.StockTransfer&&value.Scope.TenantID==scope.TenantID&&value.Scope.CompanyID==scope.CompanyID&&value.DestinationWarehouseID==scope.WarehouseID)
		if accessible && (kind == "" || value.Type == kind) && (cursor == "" || value.ID < cursor) {
			values = append(values, cloneOperation(value))
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID > values[j].ID })
	page := operations.Page{Items: values}
	if len(values) > limit {
		next := values[limit-1].ID
		page.NextCursor = &next
		page.Items = values[:limit]
	}
	return page, nil
}

func (s *Store) ListSuppliers(_ context.Context, scope tenancy.Scope, actorID, cursor string, limit int) (operations.SupplierPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actorID, "operations.read")] {
		return operations.SupplierPage{}, sales.ErrForbidden
	}
	result := operations.SupplierPage{Items: make([]operations.Supplier, 0)}
	for _, value := range s.state.suppliers {
		if value.TenantID == scope.TenantID && value.CompanyID == scope.CompanyID && (cursor == "" || value.ID < cursor) {
			result.Items = append(result.Items, value)
		}
	}
	sort.Slice(result.Items, func(i, j int) bool { return result.Items[i].ID > result.Items[j].ID })
	if len(result.Items) > limit {
		next := result.Items[limit-1].ID
		result.NextCursor = &next
		result.Items = result.Items[:limit]
	}
	return result, nil
}

func (s *Store) SeedSupplier(value operations.Supplier) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.suppliers[companyEntityKey(value.TenantID, value.CompanyID, value.ID)] = value
}

func cloneOperation(value operations.Document) operations.Document {
	value.Lines = append([]operations.DocumentLine(nil), value.Lines...)
	return value
}

func memoryOperationAccessible(current *state,scope tenancy.Scope,id string)(string,operations.Document,bool){key:=operationKey(scope,id);if value,ok:=current.operationDocuments[key];ok{return key,value,true};for candidateKey,value:=range current.operationDocuments{if value.ID==id&&value.Type==operations.StockTransfer&&value.Scope.TenantID==scope.TenantID&&value.Scope.CompanyID==scope.CompanyID&&value.DestinationWarehouseID==scope.WarehouseID{return candidateKey,value,true}};return "",operations.Document{},false}

func memoryOperationPermission(kind operations.DocumentType) string {
	switch kind {
	case operations.Quotation, operations.SalesOrder:
		return "sales.orders.manage"
	case operations.PurchaseRequest:
		return "purchases.requests.manage"
	case operations.PurchaseOrder:
		return "purchases.orders.manage"
	case operations.GoodsReceipt, operations.PurchaseReturn:
		return "purchases.receive"
	case operations.SupplierInvoice:
		return "purchases.invoices.post"
	case operations.SupplierPayment:
		return "purchases.payments.post"
	case operations.StockTransfer:
		return "inventory.transfers.manage"
	case operations.StockCount:
		return "inventory.counts.manage"
	case operations.StockAdjustment:
		return "inventory.adjustments.post"
	default:
		return ""
	}
}

func memoryAllowedTransition(document operations.Document, target operations.Status) bool {
	switch document.Status {
	case operations.Draft:
		return target == operations.Submitted
	case operations.Submitted:
		return target == operations.Approved || target == operations.Rejected
	case operations.Approved:
		switch document.Type {
		case operations.Quotation, operations.PurchaseRequest, operations.PurchaseOrder:
			return target == operations.Closed
		case operations.GoodsReceipt, operations.SupplierInvoice, operations.SupplierPayment, operations.PurchaseReturn, operations.StockCount, operations.StockAdjustment:
			return target == operations.Posted
		case operations.StockTransfer:
			return target == operations.Dispatched
		}
	case operations.Dispatched:
		return document.Type == operations.StockTransfer && target == operations.Received
	}
	return false
}
