package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/financialops"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
	"github.com/jackc/pgx/v5"
)

func (s *Store) ListGLAccounts(ctx context.Context, scope tenancy.Scope, actor string) (financialops.GLAccountPage, error) {
	result := financialops.GLAccountPage{Items: []financialops.GLAccount{}}
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "finance.accounts.read")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		rows, e := tx.tx.Query(ctx, `SELECT record_id::text,id,code,name,account_type,COALESCE(parent_account_id,''),control_account,allow_manual_posting,status,COALESCE(created_by::text,''),created_at,COALESCE(approved_by::text,''),approved_at FROM gl_accounts WHERE tenant_id=$1 AND company_id=$2 ORDER BY code`, scope.TenantID, scope.CompanyID)
		if e != nil {
			return normalizeError(e)
		}
		defer rows.Close()
		for rows.Next() {
			v := financialops.GLAccount{TenantID: scope.TenantID, CompanyID: scope.CompanyID}
			if e = rows.Scan(&v.RecordID, &v.ID, &v.Code, &v.Name, &v.Type, &v.ParentAccountID, &v.ControlAccount, &v.AllowManualPosting, &v.Status, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy, &v.ApprovedAt); e != nil {
				return normalizeError(e)
			}
			result.Items = append(result.Items, v)
		}
		return normalizeError(rows.Err())
	})
	return result, err
}
func (s *Store) CreateGLAccount(ctx context.Context, v financialops.GLAccount, scope tenancy.Scope, idem, hash string) (financialops.GLAccount, error) {
	var result financialops.GLAccount
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, v.CreatedBy, "finance.accounts.manage")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		e = tx.tx.QueryRow(ctx, `SELECT record_id::text,id,code,name,account_type,COALESCE(parent_account_id,''),control_account,allow_manual_posting,status,COALESCE(created_by::text,''),created_at,COALESCE(approved_by::text,''),approved_at FROM gl_accounts WHERE tenant_id=$1 AND company_id=$2 AND create_idempotency_key=$3`, scope.TenantID, scope.CompanyID, idem).Scan(&result.RecordID, &result.ID, &result.Code, &result.Name, &result.Type, &result.ParentAccountID, &result.ControlAccount, &result.AllowManualPosting, &result.Status, &result.CreatedBy, &result.CreatedAt, &result.ApprovedBy, &result.ApprovedAt)
		if e == nil {
			var stored string
			if e = tx.tx.QueryRow(ctx, `SELECT create_request_hash FROM gl_accounts WHERE tenant_id=$1 AND company_id=$2 AND create_idempotency_key=$3`, scope.TenantID, scope.CompanyID, idem).Scan(&stored); e != nil {
				return normalizeError(e)
			}
			if stored != hash {
				return sales.ErrIdempotencyConflict
			}
			result.TenantID, result.CompanyID = scope.TenantID, scope.CompanyID
			return nil
		}
		if !isNoRows(e) {
			return normalizeError(e)
		}
		if v.ParentAccountID != "" {
			var active bool
			if e = tx.tx.QueryRow(ctx, `SELECT status='ACTIVE' FROM gl_accounts WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, scope.TenantID, scope.CompanyID, v.ParentAccountID).Scan(&active); e != nil {
				return normalizeError(e)
			}
			if !active {
				return financialops.ErrAccountGovernance
			}
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO gl_accounts(record_id,tenant_id,company_id,id,code,name,account_type,parent_account_id,control_account,allow_manual_posting,status,created_by,created_at,create_idempotency_key,create_request_hash) VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10,'SUBMITTED',$11,$12,$13,$14)`, v.RecordID, scope.TenantID, scope.CompanyID, v.ID, v.Code, v.Name, v.Type, v.ParentAccountID, v.ControlAccount, v.AllowManualPosting, v.CreatedBy, v.CreatedAt, idem, hash)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, v.CreatedBy, "finance.account_submitted", "gl_account", v.RecordID, v.ID, v.CreatedAt); e != nil {
			return e
		}
		result = v
		return nil
	})
	return result, err
}
func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
func (s *Store) DecideGLAccount(ctx context.Context, scope tenancy.Scope, actor, id string, status financialops.GovernanceStatus, reason, idem, hash string, at time.Time) (financialops.GLAccount, error) {
	var result financialops.GLAccount
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "finance.accounts.approve")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		var recordID string
		var current financialops.GovernanceStatus
		var creator, storedIdem, storedHash string
		e = tx.tx.QueryRow(ctx, `SELECT record_id::text,status,COALESCE(created_by::text,''),COALESCE(decision_idempotency_key,''),COALESCE(decision_request_hash,'') FROM gl_accounts WHERE tenant_id=$1 AND company_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.CompanyID, id).Scan(&recordID, &current, &creator, &storedIdem, &storedHash)
		if e != nil {
			return normalizeError(e)
		}
		if storedIdem == idem {
			if storedHash != hash {
				return sales.ErrIdempotencyConflict
			}
			return tx.scanGLAccount(ctx, scope, id, &result)
		}
		if current != financialops.GovernanceSubmitted || creator == actor {
			return financialops.ErrAccountGovernance
		}
		_, e = tx.tx.Exec(ctx, `UPDATE gl_accounts SET status=$1,approved_by=$2,approved_at=$3,decision_idempotency_key=$4,decision_request_hash=$5 WHERE tenant_id=$6 AND company_id=$7 AND id=$8`, status, actor, at, idem, hash, scope.TenantID, scope.CompanyID, id)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "finance.account_"+string(status), "gl_account", recordID, id, at); e != nil {
			return e
		}
		return tx.scanGLAccount(ctx, scope, id, &result)
	})
	return result, err
}
func (t *transaction) scanGLAccount(ctx context.Context, scope tenancy.Scope, id string, v *financialops.GLAccount) error {
	v.TenantID, v.CompanyID = scope.TenantID, scope.CompanyID
	return normalizeError(t.tx.QueryRow(ctx, `SELECT record_id::text,id,code,name,account_type,COALESCE(parent_account_id,''),control_account,allow_manual_posting,status,COALESCE(created_by::text,''),created_at,COALESCE(approved_by::text,''),approved_at FROM gl_accounts WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, scope.TenantID, scope.CompanyID, id).Scan(&v.RecordID, &v.ID, &v.Code, &v.Name, &v.Type, &v.ParentAccountID, &v.ControlAccount, &v.AllowManualPosting, &v.Status, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy, &v.ApprovedAt))
}

func (s *Store) ListPostingMappings(ctx context.Context, scope tenancy.Scope, actor string) (financialops.PostingMappingPage, error) {
	result := financialops.PostingMappingPage{Items: []financialops.PostingMapping{}}
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "finance.accounts.read")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		rows, e := tx.tx.Query(ctx, `SELECT id::text,mapping_key,account_id,effective_from,status,reason,COALESCE(created_by::text,''),created_at,COALESCE(approved_by::text,''),approved_at FROM posting_mappings WHERE tenant_id=$1 AND company_id=$2 ORDER BY mapping_key,effective_from DESC`, scope.TenantID, scope.CompanyID)
		if e != nil {
			return normalizeError(e)
		}
		defer rows.Close()
		for rows.Next() {
			v := financialops.PostingMapping{TenantID: scope.TenantID, CompanyID: scope.CompanyID}
			if e = rows.Scan(&v.ID, &v.Key, &v.AccountID, &v.EffectiveFrom, &v.Status, &v.Reason, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy, &v.ApprovedAt); e != nil {
				return normalizeError(e)
			}
			result.Items = append(result.Items, v)
		}
		return normalizeError(rows.Err())
	})
	return result, err
}
func (s *Store) CreatePostingMapping(ctx context.Context, v financialops.PostingMapping, scope tenancy.Scope, idem, hash string) (financialops.PostingMapping, error) {
	var result financialops.PostingMapping
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, v.CreatedBy, "finance.accounts.manage")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acquired, resultID, e := tx.ClaimIdempotency(ctx, scope, "finance.mapping.create.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			return tx.scanPostingMapping(ctx, scope, resultID, &result)
		}
		var accountType financialops.AccountType
		var active bool
		e = tx.tx.QueryRow(ctx, `SELECT account_type,status='ACTIVE' FROM gl_accounts WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, scope.TenantID, scope.CompanyID, v.AccountID).Scan(&accountType, &active)
		if e != nil {
			return normalizeError(e)
		}
		if !active || !mappingCompatible(v.Key, accountType) {
			return financialops.ErrAccountGovernance
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO posting_mappings(id,tenant_id,company_id,mapping_key,account_id,effective_from,status,reason,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,'SUBMITTED',$7,$8,$9)`, v.ID, scope.TenantID, scope.CompanyID, v.Key, v.AccountID, v.EffectiveFrom, v.Reason, v.CreatedBy, v.CreatedAt)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, v.CreatedBy, "finance.mapping_submitted", "posting_mapping", v.ID, v.ID, v.CreatedAt); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, "finance.mapping.create.v1", idem, v.ID); e != nil {
			return e
		}
		result = v
		return nil
	})
	return result, err
}
func (s *Store) DecidePostingMapping(ctx context.Context, scope tenancy.Scope, actor, id string, status financialops.GovernanceStatus, reason, idem, hash string, at time.Time) (financialops.PostingMapping, error) {
	var result financialops.PostingMapping
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "finance.accounts.approve")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acquired, resultID, e := tx.ClaimIdempotency(ctx, scope, "finance.mapping.decide.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			return tx.scanPostingMapping(ctx, scope, resultID, &result)
		}
		var current financialops.GovernanceStatus
		var creator string
		e = tx.tx.QueryRow(ctx, `SELECT status,COALESCE(created_by::text,'') FROM posting_mappings WHERE tenant_id=$1 AND company_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.CompanyID, id).Scan(&current, &creator)
		if e != nil {
			return normalizeError(e)
		}
		if current != financialops.GovernanceSubmitted || creator == actor {
			return financialops.ErrAccountGovernance
		}
		_, e = tx.tx.Exec(ctx, `UPDATE posting_mappings SET status=$1,approved_by=$2,approved_at=$3 WHERE tenant_id=$4 AND company_id=$5 AND id=$6`, status, actor, at, scope.TenantID, scope.CompanyID, id)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "finance.mapping_"+string(status), "posting_mapping", id, id, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, "finance.mapping.decide.v1", idem, id); e != nil {
			return e
		}
		return tx.scanPostingMapping(ctx, scope, id, &result)
	})
	return result, err
}
func (t *transaction) scanPostingMapping(ctx context.Context, scope tenancy.Scope, id string, v *financialops.PostingMapping) error {
	v.TenantID, v.CompanyID = scope.TenantID, scope.CompanyID
	return normalizeError(t.tx.QueryRow(ctx, `SELECT id::text,mapping_key,account_id,effective_from,status,reason,COALESCE(created_by::text,''),created_at,COALESCE(approved_by::text,''),approved_at FROM posting_mappings WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, scope.TenantID, scope.CompanyID, id).Scan(&v.ID, &v.Key, &v.AccountID, &v.EffectiveFrom, &v.Status, &v.Reason, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy, &v.ApprovedAt))
}
func mappingCompatible(key financialops.MappingKey, t financialops.AccountType) bool {
	switch key {
	case financialops.MapSalesReceivable, financialops.MapPaymentCash, financialops.MapPaymentMobileMoney, financialops.MapPaymentBankCard, financialops.MapPaymentBankTransfer, financialops.MapStockInTransit:
		return t == financialops.AccountAsset
	case financialops.MapSalesTaxPayable, financialops.MapProcurementGRNI, financialops.MapProcurementPayable:
		return t == financialops.AccountLiability
	case financialops.MapInventoryAdjustment:
		return t == financialops.AccountExpense
	}
	return false
}

func (t *transaction) activePostingAccounts(ctx context.Context, scope tenancy.Scope, at time.Time) (map[financialops.MappingKey]string, error) {
	rows, err := t.tx.Query(ctx, `SELECT DISTINCT ON (mapping_key) mapping_key,account_id FROM posting_mappings WHERE tenant_id=$1 AND company_id=$2 AND status='ACTIVE' AND effective_from<=$3 ORDER BY mapping_key,effective_from DESC,created_at DESC`, scope.TenantID, scope.CompanyID, at)
	if err != nil {
		return nil, normalizeError(err)
	}
	defer rows.Close()
	result := make(map[financialops.MappingKey]string)
	for rows.Next() {
		var key financialops.MappingKey
		var account string
		if err = rows.Scan(&key, &account); err != nil {
			return nil, normalizeError(err)
		}
		result[key] = account
	}
	return result, normalizeError(rows.Err())
}
