package postgres

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/commercial"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) CommercialWorkspace(ctx context.Context, scope tenancy.Scope, actor string) (commercial.Workspace, error) {
	w := commercial.Workspace{Revisions: []commercial.Revision{}, RFQs: []commercial.RFQ{}, Quotes: []commercial.SupplierQuote{}, Awards: []commercial.Award{}}
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		if err := commercialAuth(ctx, tx, scope, actor, "masterdata.read"); err != nil {
			return err
		}
		if err := commercialAuth(ctx, tx, scope, actor, "purchases.sourcing.read"); err != nil {
			return err
		}
		rows, e := tx.tx.Query(ctx, `SELECT id::text,entity_type,entity_id::text,status,supplier_data,product_data,reason,created_by::text,created_at,COALESCE(approved_by::text,'') FROM master_data_revisions WHERE tenant_id=$1 AND company_id=$2 ORDER BY created_at DESC`, scope.TenantID, scope.CompanyID)
		if e != nil {
			return normalizeError(e)
		}
		for rows.Next() {
			v := commercial.Revision{Scope: scope}
			var sd, pd []byte
			if e = rows.Scan(&v.ID, &v.EntityType, &v.EntityID, &v.Status, &sd, &pd, &v.Reason, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy); e != nil {
				rows.Close()
				return normalizeError(e)
			}
			if len(sd) > 0 {
				v.Supplier = &commercial.SupplierData{}
				if e = json.Unmarshal(sd, v.Supplier); e != nil {
					rows.Close()
					return e
				}
			}
			if len(pd) > 0 {
				v.Product = &commercial.ProductData{}
				if e = json.Unmarshal(pd, v.Product); e != nil {
					rows.Close()
					return e
				}
			}
			w.Revisions = append(w.Revisions, v)
		}
		rows.Close()
		w.RFQs, e = listRFQs(ctx, tx, scope)
		if e != nil {
			return e
		}
		w.Quotes, e = listQuotes(ctx, tx, scope, "")
		if e != nil {
			return e
		}
		rows, e = tx.tx.Query(ctx, `SELECT id::text,rfq_id::text,quote_id::text,supplier_id::text,purchase_order_id::text,reason,selected_by::text,selected_at FROM sourcing_awards WHERE tenant_id=$1 AND company_id=$2 ORDER BY selected_at DESC`, scope.TenantID, scope.CompanyID)
		if e != nil {
			return normalizeError(e)
		}
		defer rows.Close()
		for rows.Next() {
			v := commercial.Award{Scope: scope}
			if e = rows.Scan(&v.ID, &v.RFQID, &v.QuoteID, &v.SupplierID, &v.PurchaseOrderID, &v.Reason, &v.SelectedBy, &v.SelectedAt); e != nil {
				return normalizeError(e)
			}
			w.Awards = append(w.Awards, v)
		}
		return rows.Err()
	})
	return w, err
}

func (s *Store) CreateMasterRevision(ctx context.Context, v commercial.Revision, idem, hash string) (commercial.Revision, error) {
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		perm := "masterdata.products.manage"
		if v.EntityType == commercial.SupplierEntity {
			perm = "masterdata.suppliers.manage"
		}
		if e := commercialAuth(ctx, tx, v.Scope, v.CreatedBy, perm); e != nil {
			return e
		}
		acq, id, e := tx.ClaimIdempotency(ctx, v.Scope, "commercial.master.create.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			v.ID = id
			return nil
		}
		sd, pd := []byte(nil), []byte(nil)
		if v.Supplier != nil {
			sd, _ = json.Marshal(v.Supplier)
		}
		if v.Product != nil {
			pd, _ = json.Marshal(v.Product)
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO master_data_revisions(id,tenant_id,company_id,entity_type,entity_id,status,supplier_data,product_data,reason,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.EntityType, v.EntityID, v.Status, nullableJSON(sd), nullableJSON(pd), v.Reason, v.CreatedBy, v.CreatedAt)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, v.Scope, v.CreatedBy, "commercial.master_revision_created", "master_data_revision", v.ID, v.ID, v.CreatedAt); e != nil {
			return e
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "commercial.master.create.v1", idem, v.ID)
	})
	return v, err
}

func (s *Store) TransitionMasterRevision(ctx context.Context, scope tenancy.Scope, actor, id string, to commercial.Status, reason, idem, hash string, at time.Time) (commercial.Revision, error) {
	v := commercial.Revision{ID: id, Scope: scope}
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		perm := "masterdata.products.manage"
		if to == commercial.Active || to == commercial.Rejected {
			perm = "masterdata.approve"
		} else {
			var entityType commercial.EntityType
			if e := tx.tx.QueryRow(ctx, `SELECT entity_type FROM master_data_revisions WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, scope.TenantID, scope.CompanyID, id).Scan(&entityType); e != nil {
				return normalizeError(e)
			}
			if entityType == commercial.SupplierEntity {
				perm = "masterdata.suppliers.manage"
			}
		}
		if e := commercialAuth(ctx, tx, scope, actor, perm); e != nil {
			return e
		}
		op := "commercial.master.transition." + string(to) + ".v1"
		acq, _, e := tx.ClaimIdempotency(ctx, scope, op, idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			v.Status = to
			return nil
		}
		var sd, pd []byte
		if e = tx.tx.QueryRow(ctx, `SELECT entity_type,entity_id::text,status,supplier_data,product_data,reason,created_by::text,created_at,COALESCE(approved_by::text,'') FROM master_data_revisions WHERE tenant_id=$1 AND company_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.CompanyID, id).Scan(&v.EntityType, &v.EntityID, &v.Status, &sd, &pd, &v.Reason, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy); e != nil {
			return normalizeError(e)
		}
		from := v.Status
		valid := from == commercial.Draft && to == commercial.Submitted || from == commercial.Submitted && (to == commercial.Active || to == commercial.Rejected)
		if !valid {
			return commercial.ErrInvalidTransition
		}
		if (to == commercial.Active || to == commercial.Rejected) && v.CreatedBy == actor {
			return commercial.ErrSeparationOfDuties
		}
		if len(sd) > 0 {
			v.Supplier = &commercial.SupplierData{}
			if e = json.Unmarshal(sd, v.Supplier); e != nil {
				return e
			}
		}
		if len(pd) > 0 {
			v.Product = &commercial.ProductData{}
			if e = json.Unmarshal(pd, v.Product); e != nil {
				return e
			}
		}
		if to == commercial.Active {
			_, e = tx.tx.Exec(ctx, `UPDATE master_data_revisions SET status='REJECTED' WHERE tenant_id=$1 AND company_id=$2 AND entity_type=$3 AND entity_id=$4 AND status='ACTIVE'`, scope.TenantID, scope.CompanyID, v.EntityType, v.EntityID)
			if e != nil {
				return normalizeError(e)
			}
			if v.Supplier != nil {
				_, e = tx.tx.Exec(ctx, `INSERT INTO suppliers(id,tenant_id,company_id,code,name,active,payment_terms_days,tax_id,email,phone) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(id) DO UPDATE SET code=EXCLUDED.code,name=EXCLUDED.name,active=EXCLUDED.active,payment_terms_days=EXCLUDED.payment_terms_days,tax_id=EXCLUDED.tax_id,email=EXCLUDED.email,phone=EXCLUDED.phone`, v.EntityID, scope.TenantID, scope.CompanyID, v.Supplier.Code, v.Supplier.Name, v.Supplier.Active, v.Supplier.PaymentTermsDays, v.Supplier.TaxID, v.Supplier.Email, v.Supplier.Phone)
			} else {
				p := v.Product
				_, e = tx.tx.Exec(ctx, `INSERT INTO products(id,tenant_id,company_id,sku,name,active,currency,list_price_minor,standard_cost_minor,tax_code,revenue_account_id,cogs_account_id,inventory_account_id,base_unit_code,price_version,master_data_version) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,1,1) ON CONFLICT(id) DO UPDATE SET sku=EXCLUDED.sku,name=EXCLUDED.name,active=EXCLUDED.active,currency=EXCLUDED.currency,list_price_minor=EXCLUDED.list_price_minor,standard_cost_minor=EXCLUDED.standard_cost_minor,tax_code=EXCLUDED.tax_code,revenue_account_id=EXCLUDED.revenue_account_id,cogs_account_id=EXCLUDED.cogs_account_id,inventory_account_id=EXCLUDED.inventory_account_id,base_unit_code=EXCLUDED.base_unit_code,price_version=products.price_version+1,master_data_version=products.master_data_version+1`, v.EntityID, scope.TenantID, scope.CompanyID, p.SKU, p.Name, p.Active, p.Currency, p.ListPriceMinor, p.StandardCostMinor, p.TaxCode, p.RevenueAccountID, p.COGSAccountID, p.InventoryAccountID, p.BaseUnitCode)
			}
			if e != nil {
				return normalizeError(e)
			}
		}
		_, e = tx.tx.Exec(ctx, `UPDATE master_data_revisions SET status=$1,approved_by=CASE WHEN $1 IN('ACTIVE','REJECTED') THEN $2::uuid ELSE approved_by END,approved_at=CASE WHEN $1 IN('ACTIVE','REJECTED') THEN $3 ELSE approved_at END WHERE tenant_id=$4 AND company_id=$5 AND id=$6`, to, actor, at, scope.TenantID, scope.CompanyID, id)
		if e != nil {
			return normalizeError(e)
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO master_data_transitions(tenant_id,company_id,revision_id,from_status,to_status,reason,actor_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, scope.TenantID, scope.CompanyID, id, from, to, reason, actor, at)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "commercial.master_revision_"+string(to), "master_data_revision", id, id, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, op, idem, id); e != nil {
			return e
		}
		v.Status = to
		if to == commercial.Active {
			v.ApprovedBy = actor
		}
		return nil
	})
	return v, err
}

func (s *Store) CreateRFQ(ctx context.Context, v commercial.RFQ, idem, hash string) (commercial.RFQ, error) {
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		if e := commercialAuth(ctx, tx, v.Scope, v.CreatedBy, "purchases.sourcing.manage"); e != nil {
			return e
		}
		acq, id, e := tx.ClaimIdempotency(ctx, v.Scope, "commercial.rfq.create.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			v.ID = id
			return nil
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO rfqs(id,tenant_id,company_id,branch_id,warehouse_id,number,status,currency,response_due_at,reason,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.Scope.BranchID, v.Scope.WarehouseID, v.Number, v.Status, v.Currency, v.ResponseDueAt, v.Reason, v.CreatedBy, v.CreatedAt)
		if e != nil {
			return normalizeError(e)
		}
		for _, l := range v.Lines {
			if _, e = tx.tx.Exec(ctx, `INSERT INTO rfq_lines(id,tenant_id,company_id,rfq_id,product_id,quantity) VALUES($1,$2,$3,$4,$5,$6)`, l.ID, v.Scope.TenantID, v.Scope.CompanyID, v.ID, l.ProductID, l.Quantity); e != nil {
				return normalizeError(e)
			}
		}
		if e = tx.financialEvidence(ctx, v.Scope, v.CreatedBy, "commercial.rfq_created", "rfq", v.ID, v.ID, v.CreatedAt); e != nil {
			return e
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "commercial.rfq.create.v1", idem, v.ID)
	})
	return v, err
}
func (s *Store) TransitionRFQ(ctx context.Context, scope tenancy.Scope, actor, id string, to commercial.RFQStatus, reason, idem, hash string, at time.Time) (commercial.RFQ, error) {
	v := commercial.RFQ{ID: id, Scope: scope}
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		perm := "purchases.sourcing.manage"
		if to == commercial.RFQApproved {
			perm = "purchases.sourcing.approve"
		}
		if e := commercialAuth(ctx, tx, scope, actor, perm); e != nil {
			return e
		}
		op := "commercial.rfq.transition." + string(to) + ".v1"
		acq, _, e := tx.ClaimIdempotency(ctx, scope, op, idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			v.Status = to
			return nil
		}
		if e = scanRFQHeader(tx.tx.QueryRow(ctx, `SELECT id::text,number,status,currency,response_due_at,reason,created_by::text,created_at,COALESCE(approved_by::text,'') FROM rfqs WHERE tenant_id=$1 AND company_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.CompanyID, id), &v); e != nil {
			return normalizeError(e)
		}
		valid := v.Status == commercial.RFQDraft && (to == commercial.RFQSubmitted || to == commercial.RFQCancelled) || v.Status == commercial.RFQSubmitted && (to == commercial.RFQApproved || to == commercial.RFQCancelled)
		if !valid {
			return commercial.ErrInvalidTransition
		}
		if to == commercial.RFQApproved && v.CreatedBy == actor {
			return commercial.ErrSeparationOfDuties
		}
		_, e = tx.tx.Exec(ctx, `UPDATE rfqs SET status=$1,approved_by=CASE WHEN $1='APPROVED' THEN $2::uuid ELSE approved_by END,approved_at=CASE WHEN $1='APPROVED' THEN $3 ELSE approved_at END WHERE tenant_id=$4 AND company_id=$5 AND id=$6`, to, actor, at, scope.TenantID, scope.CompanyID, id)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "commercial.rfq_"+string(to), "rfq", id, id, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, op, idem, id); e != nil {
			return e
		}
		v.Status = to
		return nil
	})
	return v, err
}

func (s *Store) CreateSupplierQuote(ctx context.Context, v commercial.SupplierQuote, idem, hash string) (commercial.SupplierQuote, error) {
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		if e := commercialAuth(ctx, tx, v.Scope, v.CreatedBy, "purchases.sourcing.manage"); e != nil {
			return e
		}
		acq, id, e := tx.ClaimIdempotency(ctx, v.Scope, "commercial.quote.create.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			v.ID = id
			return nil
		}
		var status commercial.RFQStatus
		var currency string
		if e = tx.tx.QueryRow(ctx, `SELECT status,currency FROM rfqs WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, v.Scope.TenantID, v.Scope.CompanyID, v.RFQID).Scan(&status, &currency); e != nil {
			return normalizeError(e)
		}
		if status != commercial.RFQApproved || currency != v.Currency {
			return commercial.ErrSourceMismatch
		}
		if !dbQuoteMatches(ctx, tx, v.Scope, v.RFQID, v.Lines) {
			return commercial.ErrSourceMismatch
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO supplier_quotes(id,tenant_id,company_id,rfq_id,supplier_id,reference,status,currency,delivery_days,payment_terms_days,valid_until,total_minor,reason,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.RFQID, v.SupplierID, v.Reference, v.Status, v.Currency, v.DeliveryDays, v.PaymentTermsDays, v.ValidUntil, v.TotalMinor, v.Reason, v.CreatedBy, v.CreatedAt)
		if e != nil {
			return normalizeError(e)
		}
		for _, l := range v.Lines {
			if _, e = tx.tx.Exec(ctx, `INSERT INTO supplier_quote_lines(id,tenant_id,company_id,quote_id,product_id,quantity,unit_price_minor,amount_minor) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, l.ID, v.Scope.TenantID, v.Scope.CompanyID, v.ID, l.ProductID, l.Quantity, l.UnitPriceMinor, l.AmountMinor); e != nil {
				return normalizeError(e)
			}
		}
		if e = tx.financialEvidence(ctx, v.Scope, v.CreatedBy, "commercial.supplier_quote_created", "supplier_quote", v.ID, v.ID, v.CreatedAt); e != nil {
			return e
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "commercial.quote.create.v1", idem, v.ID)
	})
	return v, err
}
func (s *Store) SubmitSupplierQuote(ctx context.Context, scope tenancy.Scope, actor, id, reason, idem, hash string, at time.Time) (commercial.SupplierQuote, error) {
	v := commercial.SupplierQuote{ID: id, Scope: scope}
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		if e := commercialAuth(ctx, tx, scope, actor, "purchases.sourcing.manage"); e != nil {
			return e
		}
		acq, _, e := tx.ClaimIdempotency(ctx, scope, "commercial.quote.submit.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			v.Status = commercial.QuoteSubmitted
			return nil
		}
		var current commercial.QuoteStatus
		if e = tx.tx.QueryRow(ctx, `SELECT status FROM supplier_quotes WHERE tenant_id=$1 AND company_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.CompanyID, id).Scan(&current); e != nil {
			return normalizeError(e)
		}
		if current != commercial.QuoteDraft {
			return commercial.ErrInvalidTransition
		}
		if _, e = tx.tx.Exec(ctx, `UPDATE supplier_quotes SET status='SUBMITTED' WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, scope.TenantID, scope.CompanyID, id); e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "commercial.supplier_quote_submitted", "supplier_quote", id, id, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, "commercial.quote.submit.v1", idem, id); e != nil {
			return e
		}
		v.Status = commercial.QuoteSubmitted
		v.Reason = reason
		return nil
	})
	return v, err
}
func (s *Store) RFQComparison(ctx context.Context, scope tenancy.Scope, actor, rfqID string) (commercial.Comparison, error) {
	c := commercial.Comparison{}
	err := s.WithTransaction(ctx, func(x sales.Transaction) error {
		tx := x.(*transaction)
		if e := commercialAuth(ctx, tx, scope, actor, "purchases.sourcing.read"); e != nil {
			return e
		}
		rfqs, e := listRFQs(ctx, tx, scope)
		if e != nil {
			return e
		}
		found := false
		for _, r := range rfqs {
			if r.ID == rfqID {
				c.RFQ = r
				found = true
				break
			}
		}
		if !found {
			return sales.ErrNotFound
		}
		quotes, e := listQuotes(ctx, tx, scope, rfqID)
		if e != nil {
			return e
		}
		for _, q := range quotes {
			if q.Status != commercial.QuoteDraft {
				c.Quotes = append(c.Quotes, commercial.ComparisonItem{QuoteID: q.ID, SupplierID: q.SupplierID, Reference: q.Reference, TotalMinor: q.TotalMinor, DeliveryDays: q.DeliveryDays, PaymentTermsDays: q.PaymentTermsDays, ValidUntil: q.ValidUntil, Comparable: quoteMatchesDB(c.RFQ, q), Selected: q.Status == commercial.QuoteSelected})
			}
		}
		sort.Slice(c.Quotes, func(i, j int) bool { return c.Quotes[i].TotalMinor < c.Quotes[j].TotalMinor })
		return nil
	})
	return c, err
}
func (s *Store) AwardRFQ(ctx context.Context, v commercial.Award, idem, hash string) (commercial.Award, error) {
	err := s.WithTransaction(ctx, func(x sales.Transaction) error {
		tx := x.(*transaction)
		if e := commercialAuth(ctx, tx, v.Scope, v.SelectedBy, "purchases.sourcing.approve"); e != nil {
			return e
		}
		acq, id, e := tx.ClaimIdempotency(ctx, v.Scope, "commercial.rfq.award.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			v.ID = id
			return nil
		}
		var rs commercial.RFQStatus
		var creator string
		if e = tx.tx.QueryRow(ctx, `SELECT status,created_by::text FROM rfqs WHERE tenant_id=$1 AND company_id=$2 AND id=$3 FOR UPDATE`, v.Scope.TenantID, v.Scope.CompanyID, v.RFQID).Scan(&rs, &creator); e != nil {
			return normalizeError(e)
		}
		if creator == v.SelectedBy {
			return commercial.ErrSeparationOfDuties
		}
		var qs commercial.QuoteStatus
		var qRFQ string
		var currency string
		var total int64
		if e = tx.tx.QueryRow(ctx, `SELECT status,rfq_id::text,supplier_id::text,currency,total_minor FROM supplier_quotes WHERE tenant_id=$1 AND company_id=$2 AND id=$3 FOR UPDATE`, v.Scope.TenantID, v.Scope.CompanyID, v.QuoteID).Scan(&qs, &qRFQ, &v.SupplierID, &currency, &total); e != nil {
			return normalizeError(e)
		}
		if rs != commercial.RFQApproved || qs != commercial.QuoteSubmitted || qRFQ != v.RFQID {
			return commercial.ErrSourceMismatch
		}
		var exists bool
		if e = tx.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sourcing_awards WHERE tenant_id=$1 AND company_id=$2 AND rfq_id=$3)`, v.Scope.TenantID, v.Scope.CompanyID, v.RFQID).Scan(&exists); e != nil {
			return normalizeError(e)
		}
		if exists {
			return commercial.ErrAwardExists
		}
		if _, e = tx.tx.Exec(ctx, `INSERT INTO operation_documents(id,tenant_id,company_id,branch_id,warehouse_id,number,document_type,status,party_type,party_id,currency,subtotal_minor,total_minor,reason,created_by,created_at,correlation_id,idempotency_key,request_hash) VALUES($1,$2,$3,$4,$5,$6,'PURCHASE_ORDER','DRAFT','SUPPLIER',$7,$8,$9,$9,$10,$11,$12,$13,$14,$15)`, v.PurchaseOrderID, v.Scope.TenantID, v.Scope.CompanyID, v.Scope.BranchID, v.Scope.WarehouseID, "PO-"+v.PurchaseOrderID[:12], v.SupplierID, currency, total, v.Reason, v.SelectedBy, v.SelectedAt, v.ID, "sourcing-award:"+v.ID, hash); e != nil {
			return normalizeError(e)
		}
		if _, e = tx.tx.Exec(ctx, `INSERT INTO operation_document_lines(id,tenant_id,company_id,document_id,product_id,quantity,unit_price_minor,amount_minor) SELECT id,tenant_id,company_id,$1,product_id,quantity,unit_price_minor,amount_minor FROM supplier_quote_lines WHERE tenant_id=$2 AND company_id=$3 AND quote_id=$4`, v.PurchaseOrderID, v.Scope.TenantID, v.Scope.CompanyID, v.QuoteID); e != nil {
			return normalizeError(e)
		}
		if _, e = tx.tx.Exec(ctx, `INSERT INTO sourcing_awards(id,tenant_id,company_id,rfq_id,quote_id,supplier_id,purchase_order_id,reason,selected_by,selected_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.RFQID, v.QuoteID, v.SupplierID, v.PurchaseOrderID, v.Reason, v.SelectedBy, v.SelectedAt); e != nil {
			return normalizeError(e)
		}
		if _, e = tx.tx.Exec(ctx, `UPDATE supplier_quotes SET status=CASE WHEN id=$1 THEN 'SELECTED' ELSE 'REJECTED' END WHERE tenant_id=$2 AND company_id=$3 AND rfq_id=$4 AND status='SUBMITTED'`, v.QuoteID, v.Scope.TenantID, v.Scope.CompanyID, v.RFQID); e != nil {
			return normalizeError(e)
		}
		if _, e = tx.tx.Exec(ctx, `UPDATE rfqs SET status='CLOSED' WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, v.Scope.TenantID, v.Scope.CompanyID, v.RFQID); e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, v.Scope, v.SelectedBy, "commercial.rfq_awarded", "sourcing_award", v.ID, v.ID, v.SelectedAt); e != nil {
			return e
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "commercial.rfq.award.v1", idem, v.ID)
	})
	return v, err
}

func commercialAuth(ctx context.Context, tx *transaction, scope tenancy.Scope, actor, perm string) error {
	ok, e := tx.Authorize(ctx, scope, actor, perm)
	if e != nil {
		return e
	}
	if !ok {
		return sales.ErrForbidden
	}
	return nil
}
func nullableJSON(v []byte) any {
	if len(v) == 0 {
		return nil
	}
	return v
}

type commercialRowScanner interface{ Scan(...any) error }

func scanRFQHeader(row commercialRowScanner, v *commercial.RFQ) error {
	return row.Scan(&v.ID, &v.Number, &v.Status, &v.Currency, &v.ResponseDueAt, &v.Reason, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy)
}
func listRFQs(ctx context.Context, tx *transaction, scope tenancy.Scope) ([]commercial.RFQ, error) {
	rows, e := tx.tx.Query(ctx, `SELECT id::text,number,status,currency,response_due_at,reason,created_by::text,created_at,COALESCE(approved_by::text,'') FROM rfqs WHERE tenant_id=$1 AND company_id=$2 ORDER BY created_at DESC`, scope.TenantID, scope.CompanyID)
	if e != nil {
		return nil, normalizeError(e)
	}
	defer rows.Close()
	out := []commercial.RFQ{}
	for rows.Next() {
		v := commercial.RFQ{Scope: scope, Lines: []commercial.RFQLine{}}
		if e = scanRFQHeader(rows, &v); e != nil {
			return nil, normalizeError(e)
		}
		out = append(out, v)
	}
	for i := range out {
		lr, e := tx.tx.Query(ctx, `SELECT id::text,product_id::text,quantity FROM rfq_lines WHERE tenant_id=$1 AND company_id=$2 AND rfq_id=$3 ORDER BY id`, scope.TenantID, scope.CompanyID, out[i].ID)
		if e != nil {
			return nil, normalizeError(e)
		}
		for lr.Next() {
			var l commercial.RFQLine
			if e = lr.Scan(&l.ID, &l.ProductID, &l.Quantity); e != nil {
				lr.Close()
				return nil, normalizeError(e)
			}
			out[i].Lines = append(out[i].Lines, l)
		}
		lr.Close()
	}
	return out, rows.Err()
}
func listQuotes(ctx context.Context, tx *transaction, scope tenancy.Scope, rfqID string) ([]commercial.SupplierQuote, error) {
	query := `SELECT id::text,rfq_id::text,supplier_id::text,reference,status,currency,delivery_days,payment_terms_days,valid_until,total_minor,reason,created_by::text,created_at FROM supplier_quotes WHERE tenant_id=$1 AND company_id=$2`
	args := []any{scope.TenantID, scope.CompanyID}
	if rfqID != "" {
		query += ` AND rfq_id=$3`
		args = append(args, rfqID)
	}
	query += ` ORDER BY created_at DESC`
	rows, e := tx.tx.Query(ctx, query, args...)
	if e != nil {
		return nil, normalizeError(e)
	}
	defer rows.Close()
	out := []commercial.SupplierQuote{}
	for rows.Next() {
		v := commercial.SupplierQuote{Scope: scope, Lines: []commercial.QuoteLine{}}
		if e = rows.Scan(&v.ID, &v.RFQID, &v.SupplierID, &v.Reference, &v.Status, &v.Currency, &v.DeliveryDays, &v.PaymentTermsDays, &v.ValidUntil, &v.TotalMinor, &v.Reason, &v.CreatedBy, &v.CreatedAt); e != nil {
			return nil, normalizeError(e)
		}
		out = append(out, v)
	}
	for i := range out {
		lr, e := tx.tx.Query(ctx, `SELECT id::text,product_id::text,quantity,unit_price_minor,amount_minor FROM supplier_quote_lines WHERE tenant_id=$1 AND company_id=$2 AND quote_id=$3 ORDER BY id`, scope.TenantID, scope.CompanyID, out[i].ID)
		if e != nil {
			return nil, normalizeError(e)
		}
		for lr.Next() {
			var l commercial.QuoteLine
			if e = lr.Scan(&l.ID, &l.ProductID, &l.Quantity, &l.UnitPriceMinor, &l.AmountMinor); e != nil {
				lr.Close()
				return nil, normalizeError(e)
			}
			out[i].Lines = append(out[i].Lines, l)
		}
		lr.Close()
	}
	return out, rows.Err()
}
func dbQuoteMatches(ctx context.Context, tx *transaction, scope tenancy.Scope, rfqID string, lines []commercial.QuoteLine) bool {
	rows, e := tx.tx.Query(ctx, `SELECT product_id::text,quantity FROM rfq_lines WHERE tenant_id=$1 AND company_id=$2 AND rfq_id=$3`, scope.TenantID, scope.CompanyID, rfqID)
	if e != nil {
		return false
	}
	defer rows.Close()
	m := map[string]int64{}
	for rows.Next() {
		var id string
		var q int64
		if rows.Scan(&id, &q) != nil {
			return false
		}
		m[id] = q
	}
	if len(m) != len(lines) {
		return false
	}
	for _, l := range lines {
		if m[l.ProductID] != l.Quantity {
			return false
		}
	}
	return true
}
func quoteMatchesDB(r commercial.RFQ, q commercial.SupplierQuote) bool {
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
