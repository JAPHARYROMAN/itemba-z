package postgres

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/financialops"
	"github.com/itemba-z/itemba-z/services/core-api/internal/inventory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/operations"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) CreateOperationDocument(ctx context.Context, document operations.Document, documentAudit audit.Event, documentEvent outbox.Event, idempotencyKey, requestHash string) (operations.Document, error) {
	var result operations.Document
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		permission := createPermission(document.Type)
		authorized, err := tx.Authorize(ctx, document.Scope, document.CreatedBy, permission)
		if err != nil {
			return err
		}
		if !authorized {
			return sales.ErrForbidden
		}
		acquired, resultID, err := tx.ClaimIdempotency(ctx, document.Scope, "operations.document.create.v1", idempotencyKey, requestHash)
		if err != nil {
			return err
		}
		if !acquired {
			result, err = tx.operationDocument(ctx, document.Scope, resultID, false)
			return err
		}
		if err := tx.validateOperationReferences(ctx, document); err != nil {
			return err
		}
		_, err = tx.tx.Exec(ctx, `INSERT INTO operation_documents(
			id,tenant_id,company_id,branch_id,warehouse_id,number,document_type,status,party_type,party_id,
			source_document_id,destination_warehouse_id,currency,subtotal_minor,total_minor,reason,created_by,created_at,
			correlation_id,idempotency_key,request_hash
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,'')::uuid,NULLIF($11,'')::uuid,NULLIF($12,'')::uuid,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
			document.ID, document.Scope.TenantID, document.Scope.CompanyID, document.Scope.BranchID, document.Scope.WarehouseID,
			document.Number, document.Type, document.Status, document.PartyType, document.PartyID, document.SourceDocumentID,
			document.DestinationWarehouseID, document.Currency, document.SubtotalMinor, document.TotalMinor, document.Reason,
			document.CreatedBy, document.CreatedAt, document.CorrelationID, idempotencyKey, requestHash)
		if err != nil {
			return normalizeError(err)
		}
		for _, line := range document.Lines {
			if _, err := tx.tx.Exec(ctx, `INSERT INTO operation_document_lines(id,tenant_id,company_id,document_id,product_id,quantity,unit_price_minor,amount_minor)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, line.ID, document.Scope.TenantID, document.Scope.CompanyID, document.ID,
				line.ProductID, line.Quantity, line.UnitPriceMinor, line.AmountMinor); err != nil {
				return normalizeError(err)
			}
		}
		if err := tx.AppendAuditEvent(ctx, documentAudit); err != nil {
			return err
		}
		if err := tx.AppendOutboxEvent(ctx, documentEvent); err != nil {
			return err
		}
		if err := tx.CompleteIdempotency(ctx, document.Scope, "operations.document.create.v1", idempotencyKey, document.ID); err != nil {
			return err
		}
		result = document
		return nil
	})
	return result, err
}

func (s *Store) TransitionOperationDocument(ctx context.Context, scope tenancy.Scope, actorID, documentID string, toStatus operations.Status, reason, paymentMethod, correlationID string, effects operations.EffectIDs, transitionAudit audit.Event, transitionEvent outbox.Event, idempotencyKey, requestHash string, at time.Time) (operations.Document, error) {
	var result operations.Document
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		document, err := tx.operationDocumentAccessible(ctx, scope, documentID, true)
		if err != nil {
			return err
		}
		authorized, err := tx.Authorize(ctx, scope, actorID, transitionPermission(document.Type, toStatus))
		if err != nil {
			return err
		}
		if !authorized {
			return sales.ErrForbidden
		}
		acquired, resultID, err := tx.ClaimIdempotency(ctx, scope, "operations.document.transition.v1", idempotencyKey, requestHash)
		if err != nil {
			return err
		}
		if !acquired {
			result, err = tx.operationDocumentAccessible(ctx, scope, resultID, false)
			return err
		}
		if !allowedTransition(document, toStatus) {
			return operations.ErrInvalidTransition
		}
		if toStatus == operations.Approved && document.CreatedBy == actorID {
			return operations.ErrSeparationOfDuties
		}
		if err := tx.applyOperationEffects(ctx, document, toStatus, paymentMethod, effects, at); err != nil {
			return err
		}
		_, err = tx.tx.Exec(ctx, `UPDATE operation_documents SET status=$4,
			submitted_at=CASE WHEN $4='SUBMITTED' THEN $5 ELSE submitted_at END,
			approved_at=CASE WHEN $4='APPROVED' THEN $5 ELSE approved_at END,
			posted_at=CASE WHEN $4 IN ('POSTED','DISPATCHED','RECEIVED','CLOSED') THEN $5 ELSE posted_at END
			WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, scope.TenantID, scope.CompanyID, document.ID, toStatus, at)
		if err != nil {
			return normalizeError(err)
		}
		transition := operations.Transition{ID: effects.TransitionID, DocumentID: document.ID, FromStatus: document.Status, ToStatus: toStatus, Reason: reason, ActorID: actorID, OccurredAt: at, CorrelationID: correlationID}
		if _, err := tx.tx.Exec(ctx, `INSERT INTO operation_document_transitions(id,tenant_id,company_id,document_id,from_status,to_status,reason,actor_id,occurred_at,correlation_id)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, transition.ID, scope.TenantID, scope.CompanyID, document.ID,
			transition.FromStatus, transition.ToStatus, transition.Reason, transition.ActorID, transition.OccurredAt, transition.CorrelationID); err != nil {
			return normalizeError(err)
		}
		document.Status = toStatus
		switch toStatus {
		case operations.Submitted:
			document.SubmittedAt = &at
		case operations.Approved:
			document.ApprovedAt = &at
		case operations.Posted, operations.Dispatched, operations.Received, operations.Closed:
			document.PostedAt = &at
		}
		payload, _ := json.Marshal(document)
		transitionAudit.Data, transitionEvent.Payload = payload, payload
		if err := tx.AppendAuditEvent(ctx, transitionAudit); err != nil {
			return err
		}
		if err := tx.AppendOutboxEvent(ctx, transitionEvent); err != nil {
			return err
		}
		if err := tx.CompleteIdempotency(ctx, scope, "operations.document.transition.v1", idempotencyKey, document.ID); err != nil {
			return err
		}
		result = document
		return nil
	})
	return result, err
}

func (s *Store) OperationDocument(ctx context.Context, scope tenancy.Scope, actorID, documentID string) (operations.Document, error) {
	var result operations.Document
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		allowed, err := tx.Authorize(ctx, scope, actorID, "operations.read")
		if err != nil {
			return err
		}
		if !allowed {
			return sales.ErrForbidden
		}
		result, err = tx.operationDocumentAccessible(ctx, scope, documentID, false)
		return err
	})
	return result, err
}

func (s *Store) ListOperationDocuments(ctx context.Context, scope tenancy.Scope, actorID string, kind operations.DocumentType, cursor string, limit int) (operations.Page, error) {
	result := operations.Page{Items: make([]operations.Document, 0)}
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		allowed, err := tx.Authorize(ctx, scope, actorID, "operations.read")
		if err != nil {
			return err
		}
		if !allowed {
			return sales.ErrForbidden
		}
		rows, err := tx.tx.Query(ctx, `SELECT id FROM operation_documents WHERE tenant_id=$1 AND company_id=$2 AND
			((branch_id=$3 AND warehouse_id=$4) OR (document_type='STOCK_TRANSFER' AND destination_warehouse_id=$4))
			AND ($5='' OR document_type=$5) AND ($6='' OR id < NULLIF($6,'')::uuid) ORDER BY id DESC LIMIT $7`,
			scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, kind, cursor, limit+1)
		if err != nil {
			return normalizeError(err)
		}
		defer rows.Close()
		ids := make([]string, 0, limit+1)
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return normalizeError(err)
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			return normalizeError(err)
		}
		if len(ids) > limit {
			next := ids[limit-1]
			result.NextCursor = &next
			ids = ids[:limit]
		}
		for _, id := range ids {
			document, err := tx.operationDocumentAccessible(ctx, scope, id, false)
			if err != nil {
				return err
			}
			result.Items = append(result.Items, document)
		}
		return nil
	})
	return result, err
}

func (t *transaction) validateOperationReferences(ctx context.Context, document operations.Document) error {
	var baseCurrency string
	if err := t.tx.QueryRow(ctx, `SELECT base_currency FROM legal_companies WHERE tenant_id=$1 AND id=$2`, document.Scope.TenantID, document.Scope.CompanyID).Scan(&baseCurrency); err != nil {
		return normalizeError(err)
	}
	if document.Currency != baseCurrency {
		return operations.ErrSourceMismatch
	}
	if document.PartyType == operations.CustomerParty {
		var active bool
		if err := t.tx.QueryRow(ctx, `SELECT active FROM customer_accounts WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, document.Scope.TenantID, document.Scope.CompanyID, document.PartyID).Scan(&active); err != nil {
			return normalizeError(err)
		}
		if !active {
			return sales.ErrCustomerInactive
		}
	} else if document.PartyType == operations.SupplierParty {
		var active bool
		if err := t.tx.QueryRow(ctx, `SELECT active FROM suppliers WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, document.Scope.TenantID, document.Scope.CompanyID, document.PartyID).Scan(&active); err != nil {
			return normalizeError(err)
		}
		if !active {
			return operations.ErrSourceMismatch
		}
	}
	if document.SourceDocumentID != "" {
		var exists bool
		if err := t.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM operation_documents WHERE tenant_id=$1 AND company_id=$2 AND id=$3)`, document.Scope.TenantID, document.Scope.CompanyID, document.SourceDocumentID).Scan(&exists); err != nil || !exists {
			if err != nil {
				return normalizeError(err)
			}
			return sales.ErrNotFound
		}
	}
	if document.DestinationWarehouseID != "" {
		var exists bool
		if err := t.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE tenant_id=$1 AND company_id=$2 AND id=$3)`, document.Scope.TenantID, document.Scope.CompanyID, document.DestinationWarehouseID).Scan(&exists); err != nil || !exists {
			if err != nil {
				return normalizeError(err)
			}
			return sales.ErrNotFound
		}
	}
	for _, line := range document.Lines {
		var active bool
		var listPrice int64
		if err := t.tx.QueryRow(ctx, `SELECT active,list_price_minor FROM products WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, document.Scope.TenantID, document.Scope.CompanyID, line.ProductID).Scan(&active, &listPrice); err != nil {
			return normalizeError(err)
		}
		if !active {
			return sales.ErrProductInactive
		}
		if (document.Type == operations.Quotation || document.Type == operations.SalesOrder) && line.UnitPriceMinor != listPrice {
			return operations.ErrSourceMismatch
		}
	}
	return nil
}

func (t *transaction) operationDocument(ctx context.Context, scope tenancy.Scope, documentID string, lock bool) (operations.Document, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return operations.Document{}, err
	}
	query := `SELECT id,number,document_type,status,party_type,COALESCE(party_id::text,''),COALESCE(source_document_id::text,''),
		COALESCE(destination_warehouse_id::text,''),currency,subtotal_minor,total_minor,reason,created_by::text,created_at,submitted_at,approved_at,posted_at,correlation_id::text
		FROM operation_documents WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND id=$5`
	if lock {
		query += ` FOR UPDATE`
	}
	var value operations.Document
	value.Scope = scope
	err := t.tx.QueryRow(ctx, query, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, documentID).Scan(
		&value.ID, &value.Number, &value.Type, &value.Status, &value.PartyType, &value.PartyID, &value.SourceDocumentID,
		&value.DestinationWarehouseID, &value.Currency, &value.SubtotalMinor, &value.TotalMinor, &value.Reason, &value.CreatedBy,
		&value.CreatedAt, &value.SubmittedAt, &value.ApprovedAt, &value.PostedAt, &value.CorrelationID)
	if err != nil {
		return operations.Document{}, normalizeError(err)
	}
	rows, err := t.tx.Query(ctx, `SELECT id,product_id::text,quantity,unit_price_minor,amount_minor FROM operation_document_lines
		WHERE tenant_id=$1 AND company_id=$2 AND document_id=$3 ORDER BY id`, scope.TenantID, scope.CompanyID, documentID)
	if err != nil {
		return operations.Document{}, normalizeError(err)
	}
	defer rows.Close()
	value.Lines = make([]operations.DocumentLine, 0)
	for rows.Next() {
		var line operations.DocumentLine
		if err := rows.Scan(&line.ID, &line.ProductID, &line.Quantity, &line.UnitPriceMinor, &line.AmountMinor); err != nil {
			return operations.Document{}, normalizeError(err)
		}
		value.Lines = append(value.Lines, line)
	}
	return value, normalizeError(rows.Err())
}

func (t *transaction) operationDocumentAccessible(ctx context.Context, requestScope tenancy.Scope, documentID string, lock bool) (operations.Document, error) {
	if err := t.ensureScope(ctx, requestScope); err != nil {
		return operations.Document{}, err
	}
	var branchID, warehouseID, destination string
	query := `SELECT branch_id::text,warehouse_id::text,COALESCE(destination_warehouse_id::text,'') FROM operation_documents WHERE tenant_id=$1 AND company_id=$2 AND id=$3`
	if lock {
		query += ` FOR UPDATE`
	}
	if err := t.tx.QueryRow(ctx, query, requestScope.TenantID, requestScope.CompanyID, documentID).Scan(&branchID, &warehouseID, &destination); err != nil {
		return operations.Document{}, normalizeError(err)
	}
	if !(branchID == requestScope.BranchID && warehouseID == requestScope.WarehouseID) && destination != requestScope.WarehouseID {
		return operations.Document{}, sales.ErrNotFound
	}
	sourceScope := requestScope
	sourceScope.BranchID, sourceScope.WarehouseID = branchID, warehouseID
	return t.operationDocument(ctx, sourceScope, documentID, false)
}

func allowedTransition(document operations.Document, target operations.Status) bool {
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

func createPermission(kind operations.DocumentType) string {
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

func transitionPermission(kind operations.DocumentType, status operations.Status) string {
	if status == operations.Approved || status == operations.Rejected {
		return createPermission(kind)
	}
	return createPermission(kind)
}

func (t *transaction) applyOperationEffects(ctx context.Context, document operations.Document, target operations.Status, paymentMethod string, effects operations.EffectIDs, at time.Time) error {
	if target == operations.Submitted || target == operations.Rejected {
		return nil
	}
	if document.Type == operations.SalesOrder && target == operations.Approved {
		if document.SourceDocumentID != "" {
			source, err := t.operationDocumentByCompany(ctx, document.Scope, document.SourceDocumentID, true)
			if err != nil || source.Type != operations.Quotation || (source.Status != operations.Approved && source.Status != operations.Closed) || source.PartyID != document.PartyID || !sameLines(source.Lines, document.Lines) {
				return operations.ErrSourceMismatch
			}
		}
		return t.reserveSalesOrder(ctx, document, effects.ReserveIDs, at)
	}
	if document.Type == operations.PurchaseOrder && target == operations.Approved && document.SourceDocumentID != "" {
		source, err := t.operationDocumentByCompany(ctx, document.Scope, document.SourceDocumentID, true)
		if err != nil || source.Type != operations.PurchaseRequest || (source.Status != operations.Approved && source.Status != operations.Closed) || !sameProductQuantities(source.Lines, document.Lines) {
			return operations.ErrSourceMismatch
		}
	}
	if target == operations.Approved {
		return nil
	}
	if target == operations.Closed && (document.Type == operations.Quotation || document.Type == operations.PurchaseRequest || document.Type == operations.PurchaseOrder) {
		return nil
	}
	switch document.Type {
	case operations.GoodsReceipt:
		return t.postGoodsReceipt(ctx, document, effects, at)
	case operations.SupplierInvoice:
		return t.postSupplierInvoice(ctx, document, effects, at)
	case operations.SupplierPayment:
		return t.postSupplierPayment(ctx, document, paymentMethod, effects, at)
	case operations.PurchaseReturn:
		return t.postPurchaseReturn(ctx, document, effects, at)
	case operations.StockTransfer:
		if target == operations.Dispatched {
			return t.dispatchTransfer(ctx, document, effects, at)
		}
		return t.receiveTransfer(ctx, document, effects, at)
	case operations.StockCount, operations.StockAdjustment:
		return t.postStockVariance(ctx, document, effects, at)
	}
	return nil
}

func (t *transaction) operationPostingConfig(ctx context.Context, scope tenancy.Scope) (operations.PostingConfig, error) {
	var value operations.PostingConfig
	var cash []byte
	err := t.tx.QueryRow(ctx, `SELECT grni_account_id,payable_account_id,inventory_adjustment_account_id,stock_in_transit_account_id,cash_accounts
		FROM procurement_posting_config WHERE tenant_id=$1 AND company_id=$2`, scope.TenantID, scope.CompanyID).Scan(
		&value.GRNIAccountID, &value.PayableAccountID, &value.InventoryAdjustmentAccountID, &value.StockInTransitAccountID, &cash)
	if err != nil {
		return operations.PostingConfig{}, operations.ErrPostingConfiguration
	}
	if err := json.Unmarshal(cash, &value.CashAccounts); err != nil {
		return operations.PostingConfig{}, operations.ErrPostingConfiguration
	}
	mappings, err := t.activePostingAccounts(ctx, scope, time.Now().UTC())
	if err != nil {
		return operations.PostingConfig{}, operations.ErrPostingConfiguration
	}
	if account := mappings[financialops.MapProcurementGRNI]; account != "" {
		value.GRNIAccountID = account
	}
	if account := mappings[financialops.MapProcurementPayable]; account != "" {
		value.PayableAccountID = account
	}
	if account := mappings[financialops.MapInventoryAdjustment]; account != "" {
		value.InventoryAdjustmentAccountID = account
	}
	if account := mappings[financialops.MapStockInTransit]; account != "" {
		value.StockInTransitAccountID = account
	}
	for key, method := range map[financialops.MappingKey]string{
		financialops.MapPaymentCash: sales.PaymentCash, financialops.MapPaymentMobileMoney: sales.PaymentMobileMoney,
		financialops.MapPaymentBankCard: sales.PaymentBankCard, financialops.MapPaymentBankTransfer: sales.PaymentBankTransfer,
	} {
		if account := mappings[key]; account != "" {
			value.CashAccounts[method] = account
		}
	}
	return value, nil
}

func (t *transaction) reserveSalesOrder(ctx context.Context, document operations.Document, ids []string, at time.Time) error {
	for index, line := range document.Lines {
		available, err := t.AvailableStock(ctx, document.Scope, line.ProductID)
		if err != nil {
			return err
		}
		if available < line.Quantity {
			return operations.ErrInsufficientStock
		}
		if index >= len(ids) {
			return operations.ErrInvalidCommand
		}
		_, err = t.tx.Exec(ctx, `INSERT INTO inventory_reservation_ledger(id,tenant_id,company_id,branch_id,warehouse_id,product_id,source_type,source_id,quantity,occurred_at)
			VALUES($1,$2,$3,$4,$5,$6,'SALES_ORDER',$7,$8,$9)`, ids[index], document.Scope.TenantID, document.Scope.CompanyID,
			document.Scope.BranchID, document.Scope.WarehouseID, line.ProductID, document.ID, line.Quantity, at)
		if err != nil {
			return normalizeError(err)
		}
	}
	return nil
}

func (t *transaction) postGoodsReceipt(ctx context.Context, document operations.Document, effects operations.EffectIDs, at time.Time) error {
	source, err := t.operationDocumentByCompany(ctx, document.Scope, document.SourceDocumentID, true)
	if err != nil || source.Type != operations.PurchaseOrder || source.Status != operations.Approved || source.PartyID != document.PartyID || source.Currency != document.Currency {
		return operations.ErrSourceMismatch
	}
	config, err := t.operationPostingConfig(ctx, document.Scope)
	if err != nil {
		return err
	}
	journal := finance.Journal{ID: effects.JournalID, TenantID: document.Scope.TenantID, CompanyID: document.Scope.CompanyID, SourceType: string(document.Type), SourceID: document.ID, Currency: document.Currency, OccurredAt: at}
	for index, line := range document.Lines {
		ordered, ok := matchingLine(source.Lines, line.ProductID)
		if !ok || ordered.UnitPriceMinor != line.UnitPriceMinor || line.Quantity <= 0 {
			return operations.ErrSourceMismatch
		}
		var received int64
		if err := t.tx.QueryRow(ctx, `SELECT COALESCE(sum(l.quantity),0)::bigint FROM operation_documents d JOIN operation_document_lines l ON l.tenant_id=d.tenant_id AND l.company_id=d.company_id AND l.document_id=d.id
			WHERE d.tenant_id=$1 AND d.company_id=$2 AND d.source_document_id=$3 AND d.document_type='GOODS_RECEIPT' AND d.status='POSTED' AND l.product_id=$4`,
			document.Scope.TenantID, document.Scope.CompanyID, source.ID, line.ProductID).Scan(&received); err != nil {
			return normalizeError(err)
		}
		if received+line.Quantity > ordered.Quantity {
			return operations.ErrOverReceipt
		}
		product, err := t.Product(ctx, document.Scope, line.ProductID)
		if err != nil {
			return err
		}
		if index >= len(effects.MovementIDs) {
			return operations.ErrInvalidCommand
		}
		if err := t.AppendStockMovement(ctx, inventory.Movement{ID: effects.MovementIDs[index], TenantID: document.Scope.TenantID, CompanyID: document.Scope.CompanyID, BranchID: document.Scope.BranchID, WarehouseID: document.Scope.WarehouseID, ProductID: line.ProductID, SourceType: string(document.Type), SourceID: document.ID, Quantity: line.Quantity, OccurredAt: at}); err != nil {
			return err
		}
		if err := t.applyGoodsReceiptInventoryControls(ctx, document, line, effects.MovementIDs[index], product, at); err != nil {
			return err
		}
		journal.Entries = append(journal.Entries, finance.JournalEntry{AccountID: product.InventoryAccountID, DebitMinor: line.AmountMinor, Memo: "Goods received"})
	}
	journal.Entries = append(journal.Entries, finance.JournalEntry{AccountID: config.GRNIAccountID, CreditMinor: document.TotalMinor, Memo: "Goods received not invoiced"})
	return t.CreateJournal(ctx, journal)
}

func (t *transaction) postSupplierInvoice(ctx context.Context, document operations.Document, effects operations.EffectIDs, at time.Time) error {
	receipt, err := t.operationDocumentByCompany(ctx, document.Scope, document.SourceDocumentID, true)
	if err != nil || receipt.Type != operations.GoodsReceipt || receipt.Status != operations.Posted || receipt.PartyID != document.PartyID || receipt.Currency != document.Currency || !sameLines(receipt.Lines, document.Lines) {
		return operations.ErrSourceMismatch
	}
	var existing bool
	if err := t.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM operation_documents WHERE tenant_id=$1 AND company_id=$2 AND source_document_id=$3 AND document_type='SUPPLIER_INVOICE' AND status='POSTED')`, document.Scope.TenantID, document.Scope.CompanyID, receipt.ID).Scan(&existing); err != nil {
		return normalizeError(err)
	}
	if existing {
		return operations.ErrOverInvoice
	}
	config, err := t.operationPostingConfig(ctx, document.Scope)
	if err != nil {
		return err
	}
	var terms int64
	if err := t.tx.QueryRow(ctx, `SELECT payment_terms_days FROM suppliers WHERE tenant_id=$1 AND company_id=$2 AND id=$3 AND active`, document.Scope.TenantID, document.Scope.CompanyID, document.PartyID).Scan(&terms); err != nil {
		return normalizeError(err)
	}
	due := at.AddDate(0, 0, int(terms))
	if _, err := t.tx.Exec(ctx, `INSERT INTO supplier_ledger(id,tenant_id,company_id,supplier_id,source_type,source_id,amount_minor,currency,occurred_at)
		VALUES($1,$2,$3,$4,'SUPPLIER_INVOICE',$5,$6,$7,$8)`, effects.LedgerID, document.Scope.TenantID, document.Scope.CompanyID,
		document.PartyID, document.ID, document.TotalMinor, document.Currency, at); err != nil {
		return normalizeError(err)
	}
	if _, err := t.tx.Exec(ctx, `INSERT INTO supplier_payable_items(id,tenant_id,company_id,supplier_id,kind,source_type,source_id,amount_minor,currency,document_at,due_at)
		VALUES($1,$2,$3,$4,'INVOICE','SUPPLIER_INVOICE',$5,$6,$7,$8,$9)`, effects.PayableID, document.Scope.TenantID, document.Scope.CompanyID,
		document.PartyID, document.ID, document.TotalMinor, document.Currency, at, due); err != nil {
		return normalizeError(err)
	}
	return t.CreateJournal(ctx, finance.Journal{ID: effects.JournalID, TenantID: document.Scope.TenantID, CompanyID: document.Scope.CompanyID,
		SourceType: string(document.Type), SourceID: document.ID, Currency: document.Currency, OccurredAt: at, Entries: []finance.JournalEntry{
			{AccountID: config.GRNIAccountID, DebitMinor: document.TotalMinor, Memo: "Matched supplier invoice"},
			{AccountID: config.PayableAccountID, CreditMinor: document.TotalMinor, Memo: "Supplier payable"},
		}})
}

func (t *transaction) postSupplierPayment(ctx context.Context, document operations.Document, paymentMethod string, effects operations.EffectIDs, at time.Time) error {
	invoice, err := t.operationDocumentByCompany(ctx, document.Scope, document.SourceDocumentID, true)
	if err != nil || invoice.Type != operations.SupplierInvoice || invoice.Status != operations.Posted || invoice.PartyID != document.PartyID || invoice.Currency != document.Currency || paymentMethod == "" {
		return operations.ErrSourceMismatch
	}
	config, err := t.operationPostingConfig(ctx, document.Scope)
	if err != nil {
		return err
	}
	accountID := config.CashAccounts[paymentMethod]
	if accountID == "" {
		return operations.ErrPostingConfiguration
	}
	var payableID string
	var amount, allocated int64
	if err := t.tx.QueryRow(ctx, `SELECT id,amount_minor FROM supplier_payable_items WHERE tenant_id=$1 AND company_id=$2 AND source_type='SUPPLIER_INVOICE' AND source_id=$3 FOR UPDATE`, document.Scope.TenantID, document.Scope.CompanyID, invoice.ID).Scan(&payableID, &amount); err != nil {
		return normalizeError(err)
	}
	if err := t.tx.QueryRow(ctx, `SELECT COALESCE(sum(amount_minor),0)::bigint FROM supplier_payable_allocations WHERE tenant_id=$1 AND company_id=$2 AND debit_item_id=$3`, document.Scope.TenantID, document.Scope.CompanyID, payableID).Scan(&allocated); err != nil {
		return normalizeError(err)
	}
	if document.TotalMinor <= 0 || document.TotalMinor > amount-allocated {
		return operations.ErrPayableExceeded
	}
	if _, err := t.tx.Exec(ctx, `INSERT INTO supplier_ledger(id,tenant_id,company_id,supplier_id,source_type,source_id,amount_minor,currency,occurred_at)
		VALUES($1,$2,$3,$4,'SUPPLIER_PAYMENT',$5,$6,$7,$8)`, effects.LedgerID, document.Scope.TenantID, document.Scope.CompanyID, document.PartyID,
		document.ID, -document.TotalMinor, document.Currency, at); err != nil {
		return normalizeError(err)
	}
	if _, err := t.tx.Exec(ctx, `INSERT INTO supplier_payable_items(id,tenant_id,company_id,supplier_id,kind,source_type,source_id,amount_minor,currency,document_at,due_at)
		VALUES($1,$2,$3,$4,'PAYMENT','SUPPLIER_PAYMENT',$5,$6,$7,$8,NULL)`, effects.PayableID, document.Scope.TenantID, document.Scope.CompanyID,
		document.PartyID, document.ID, document.TotalMinor, document.Currency, at); err != nil {
		return normalizeError(err)
	}
	if _, err := t.tx.Exec(ctx, `INSERT INTO supplier_payable_allocations(id,tenant_id,company_id,supplier_id,debit_item_id,credit_item_id,amount_minor,occurred_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, effects.AllocationID, document.Scope.TenantID, document.Scope.CompanyID, document.PartyID, payableID, effects.PayableID, document.TotalMinor, at); err != nil {
		return normalizeError(err)
	}
	if _, err := t.tx.Exec(ctx, `INSERT INTO supplier_payments(id,tenant_id,company_id,supplier_id,supplier_invoice_id,account_id,method,amount_minor,currency,occurred_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, effects.PaymentID, document.Scope.TenantID, document.Scope.CompanyID, document.PartyID,
		invoice.ID, accountID, paymentMethod, document.TotalMinor, document.Currency, at); err != nil {
		return normalizeError(err)
	}
	return t.CreateJournal(ctx, finance.Journal{ID: effects.JournalID, TenantID: document.Scope.TenantID, CompanyID: document.Scope.CompanyID,
		SourceType: string(document.Type), SourceID: document.ID, Currency: document.Currency, OccurredAt: at, Entries: []finance.JournalEntry{
			{AccountID: config.PayableAccountID, DebitMinor: document.TotalMinor, Memo: "Supplier payment"},
			{AccountID: accountID, CreditMinor: document.TotalMinor, Memo: "Cash or bank payment"},
		}})
}

func (t *transaction) postPurchaseReturn(ctx context.Context, document operations.Document, effects operations.EffectIDs, at time.Time) error {
	invoice, err := t.operationDocumentByCompany(ctx, document.Scope, document.SourceDocumentID, true)
	if err != nil || invoice.Type != operations.SupplierInvoice || invoice.Status != operations.Posted || invoice.PartyID != document.PartyID || invoice.Currency != document.Currency {
		return operations.ErrSourceMismatch
	}
	config, err := t.operationPostingConfig(ctx, document.Scope)
	if err != nil {
		return err
	}
	var invoiceItemID string
	if err := t.tx.QueryRow(ctx, `SELECT id FROM supplier_payable_items WHERE tenant_id=$1 AND company_id=$2 AND source_type='SUPPLIER_INVOICE' AND source_id=$3 FOR UPDATE`, document.Scope.TenantID, document.Scope.CompanyID, invoice.ID).Scan(&invoiceItemID); err != nil {
		return normalizeError(err)
	}
	journal := finance.Journal{ID: effects.JournalID, TenantID: document.Scope.TenantID, CompanyID: document.Scope.CompanyID, SourceType: string(document.Type), SourceID: document.ID, Currency: document.Currency, OccurredAt: at}
	for index, line := range document.Lines {
		original, ok := matchingLine(invoice.Lines, line.ProductID)
		var returned int64
		if err := t.tx.QueryRow(ctx, `SELECT COALESCE(sum(l.quantity),0)::bigint FROM operation_documents d JOIN operation_document_lines l ON l.tenant_id=d.tenant_id AND l.company_id=d.company_id AND l.document_id=d.id WHERE d.tenant_id=$1 AND d.company_id=$2 AND d.source_document_id=$3 AND d.document_type='PURCHASE_RETURN' AND d.status='POSTED' AND l.product_id=$4`, document.Scope.TenantID, document.Scope.CompanyID, invoice.ID, line.ProductID).Scan(&returned); err != nil {
			return normalizeError(err)
		}
		if !ok || line.Quantity <= 0 || returned+line.Quantity > original.Quantity || line.UnitPriceMinor != original.UnitPriceMinor {
			return operations.ErrSourceMismatch
		}
		available, err := t.AvailableStock(ctx, document.Scope, line.ProductID)
		if err != nil || available < line.Quantity {
			return operations.ErrInsufficientStock
		}
		product, err := t.Product(ctx, document.Scope, line.ProductID)
		if err != nil {
			return err
		}
		if err := t.AppendStockMovement(ctx, inventory.Movement{ID: effects.MovementIDs[index], TenantID: document.Scope.TenantID, CompanyID: document.Scope.CompanyID, BranchID: document.Scope.BranchID, WarehouseID: document.Scope.WarehouseID, ProductID: line.ProductID, SourceType: string(document.Type), SourceID: document.ID, Quantity: -line.Quantity, OccurredAt: at}); err != nil {
			return err
		}
		journal.Entries = append(journal.Entries, finance.JournalEntry{AccountID: product.InventoryAccountID, CreditMinor: line.AmountMinor, Memo: "Purchase return"})
	}
	journal.Entries = append(journal.Entries, finance.JournalEntry{AccountID: config.PayableAccountID, DebitMinor: document.TotalMinor, Memo: "Supplier credit"})
	if _, err := t.tx.Exec(ctx, `INSERT INTO supplier_ledger(id,tenant_id,company_id,supplier_id,source_type,source_id,amount_minor,currency,occurred_at)
		VALUES($1,$2,$3,$4,'PURCHASE_RETURN',$5,$6,$7,$8)`, effects.LedgerID, document.Scope.TenantID, document.Scope.CompanyID, document.PartyID, document.ID, -document.TotalMinor, document.Currency, at); err != nil {
		return normalizeError(err)
	}
	if _, err := t.tx.Exec(ctx, `INSERT INTO supplier_payable_items(id,tenant_id,company_id,supplier_id,kind,source_type,source_id,amount_minor,currency,document_at,due_at)
		VALUES($1,$2,$3,$4,'CREDIT_NOTE','PURCHASE_RETURN',$5,$6,$7,$8,NULL)`, effects.PayableID, document.Scope.TenantID, document.Scope.CompanyID, document.PartyID, document.ID, document.TotalMinor, document.Currency, at); err != nil {
		return normalizeError(err)
	}
	if _, err := t.tx.Exec(ctx, `INSERT INTO supplier_payable_allocations(id,tenant_id,company_id,supplier_id,debit_item_id,credit_item_id,amount_minor,occurred_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, effects.AllocationID, document.Scope.TenantID, document.Scope.CompanyID, document.PartyID, invoiceItemID, effects.PayableID, document.TotalMinor, at); err != nil {
		return normalizeError(err)
	}
	return t.CreateJournal(ctx, journal)
}

func (t *transaction) dispatchTransfer(ctx context.Context, document operations.Document, effects operations.EffectIDs, at time.Time) error {
	config, err := t.operationPostingConfig(ctx, document.Scope)
	if err != nil {
		return err
	}
	journal := finance.Journal{ID: effects.JournalID, TenantID: document.Scope.TenantID, CompanyID: document.Scope.CompanyID, SourceType: "STOCK_TRANSFER_DISPATCH", SourceID: document.ID, Currency: document.Currency, OccurredAt: at}
	var total int64
	for index, line := range document.Lines {
		available, err := t.AvailableStock(ctx, document.Scope, line.ProductID)
		if err != nil || available < line.Quantity {
			return operations.ErrInsufficientStock
		}
		product, err := t.Product(ctx, document.Scope, line.ProductID)
		if err != nil {
			return err
		}
		value, err := operationProductValue(line.Quantity, product.StandardCostMinor)
		if err != nil {
			return err
		}
		total += value
		if err := t.AppendStockMovement(ctx, inventory.Movement{ID: effects.MovementIDs[index], TenantID: document.Scope.TenantID, CompanyID: document.Scope.CompanyID, BranchID: document.Scope.BranchID, WarehouseID: document.Scope.WarehouseID, ProductID: line.ProductID, SourceType: "STOCK_TRANSFER_DISPATCH", SourceID: document.ID, Quantity: -line.Quantity, OccurredAt: at}); err != nil {
			return err
		}
		journal.Entries = append(journal.Entries, finance.JournalEntry{AccountID: product.InventoryAccountID, CreditMinor: value, Memo: "Transfer dispatched"})
	}
	journal.Entries = append(journal.Entries, finance.JournalEntry{AccountID: config.StockInTransitAccountID, DebitMinor: total, Memo: "Stock in transit"})
	return t.CreateJournal(ctx, journal)
}

func (t *transaction) receiveTransfer(ctx context.Context, document operations.Document, effects operations.EffectIDs, at time.Time) error {
	config, err := t.operationPostingConfig(ctx, document.Scope)
	if err != nil {
		return err
	}
	var destinationBranch string
	if err := t.tx.QueryRow(ctx, `SELECT branch_id::text FROM warehouses WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, document.Scope.TenantID, document.Scope.CompanyID, document.DestinationWarehouseID).Scan(&destinationBranch); err != nil {
		return normalizeError(err)
	}
	journal := finance.Journal{ID: effects.JournalID, TenantID: document.Scope.TenantID, CompanyID: document.Scope.CompanyID, SourceType: "STOCK_TRANSFER_RECEIPT", SourceID: document.ID, Currency: document.Currency, OccurredAt: at}
	var total int64
	for index, line := range document.Lines {
		product, err := t.Product(ctx, document.Scope, line.ProductID)
		if err != nil {
			return err
		}
		value, err := operationProductValue(line.Quantity, product.StandardCostMinor)
		if err != nil {
			return err
		}
		total += value
		if err := t.AppendStockMovement(ctx, inventory.Movement{ID: effects.MovementIDs[index], TenantID: document.Scope.TenantID, CompanyID: document.Scope.CompanyID, BranchID: destinationBranch, WarehouseID: document.DestinationWarehouseID, ProductID: line.ProductID, SourceType: "STOCK_TRANSFER_RECEIPT", SourceID: document.ID, Quantity: line.Quantity, OccurredAt: at}); err != nil {
			return err
		}
		journal.Entries = append(journal.Entries, finance.JournalEntry{AccountID: product.InventoryAccountID, DebitMinor: value, Memo: "Transfer received"})
	}
	journal.Entries = append(journal.Entries, finance.JournalEntry{AccountID: config.StockInTransitAccountID, CreditMinor: total, Memo: "Stock in transit cleared"})
	return t.CreateJournal(ctx, journal)
}

func (t *transaction) postStockVariance(ctx context.Context, document operations.Document, effects operations.EffectIDs, at time.Time) error {
	config, err := t.operationPostingConfig(ctx, document.Scope)
	if err != nil {
		return err
	}
	journal := finance.Journal{ID: effects.JournalID, TenantID: document.Scope.TenantID, CompanyID: document.Scope.CompanyID, SourceType: string(document.Type), SourceID: document.ID, Currency: document.Currency, OccurredAt: at}
	var netValue int64
	for index, line := range document.Lines {
		movementQuantity := line.Quantity
		if document.Type == operations.StockCount {
			current, err := t.AvailableStock(ctx, document.Scope, line.ProductID)
			if err != nil {
				return err
			}
			movementQuantity = line.Quantity - current
		}
		if movementQuantity == 0 {
			continue
		}
		current, err := t.AvailableStock(ctx, document.Scope, line.ProductID)
		if err != nil || current+movementQuantity < 0 {
			return operations.ErrInsufficientStock
		}
		product, err := t.Product(ctx, document.Scope, line.ProductID)
		if err != nil {
			return err
		}
		value, err := operationProductValue(movementQuantity, product.StandardCostMinor)
		if err != nil {
			return err
		}
		netValue += value
		if err := t.AppendStockMovement(ctx, inventory.Movement{ID: effects.MovementIDs[index], TenantID: document.Scope.TenantID, CompanyID: document.Scope.CompanyID, BranchID: document.Scope.BranchID, WarehouseID: document.Scope.WarehouseID, ProductID: line.ProductID, SourceType: string(document.Type), SourceID: document.ID, Quantity: movementQuantity, OccurredAt: at}); err != nil {
			return err
		}
		if value > 0 {
			journal.Entries = append(journal.Entries, finance.JournalEntry{AccountID: product.InventoryAccountID, DebitMinor: value, Memo: "Stock variance gain"})
		} else {
			journal.Entries = append(journal.Entries, finance.JournalEntry{AccountID: product.InventoryAccountID, CreditMinor: -value, Memo: "Stock variance loss"})
		}
	}
	if netValue == 0 {
		return nil
	}
	if netValue > 0 {
		journal.Entries = append(journal.Entries, finance.JournalEntry{AccountID: config.InventoryAdjustmentAccountID, CreditMinor: netValue, Memo: "Inventory variance"})
	} else {
		journal.Entries = append(journal.Entries, finance.JournalEntry{AccountID: config.InventoryAdjustmentAccountID, DebitMinor: -netValue, Memo: "Inventory variance"})
	}
	return t.CreateJournal(ctx, journal)
}

func (t *transaction) operationDocumentByCompany(ctx context.Context, scope tenancy.Scope, id string, lock bool) (operations.Document, error) {
	var branchID, warehouseID string
	query := `SELECT branch_id::text,warehouse_id::text FROM operation_documents WHERE tenant_id=$1 AND company_id=$2 AND id=$3`
	if lock {
		query += ` FOR UPDATE`
	}
	if err := t.tx.QueryRow(ctx, query, scope.TenantID, scope.CompanyID, id).Scan(&branchID, &warehouseID); err != nil {
		return operations.Document{}, normalizeError(err)
	}
	sourceScope := scope
	sourceScope.BranchID, sourceScope.WarehouseID = branchID, warehouseID
	return t.operationDocument(ctx, sourceScope, id, false)
}

func matchingLine(lines []operations.DocumentLine, productID string) (operations.DocumentLine, bool) {
	for _, line := range lines {
		if line.ProductID == productID {
			return line, true
		}
	}
	return operations.DocumentLine{}, false
}

func sameLines(left, right []operations.DocumentLine) bool {
	if len(left) != len(right) {
		return false
	}
	for _, line := range left {
		match, ok := matchingLine(right, line.ProductID)
		if !ok || match.Quantity != line.Quantity || match.UnitPriceMinor != line.UnitPriceMinor || match.AmountMinor != line.AmountMinor {
			return false
		}
	}
	return true
}

func sameProductQuantities(left, right []operations.DocumentLine) bool {
	if len(left) != len(right) {
		return false
	}
	for _, line := range left {
		match, ok := matchingLine(right, line.ProductID)
		if !ok || match.Quantity != line.Quantity {
			return false
		}
	}
	return true
}

func operationProductValue(quantity, cost int64) (int64, error) {
	if cost < 0 || quantity == math.MinInt64 {
		return 0, operations.ErrInvalidCommand
	}
	abs := quantity
	if abs < 0 {
		abs = -abs
	}
	if cost != 0 && abs > math.MaxInt64/cost {
		return 0, operations.ErrInvalidCommand
	}
	return quantity * cost, nil
}
