package postgres

import (
	"context"
	"github.com/itemba-z/itemba-z/services/core-api/internal/operations"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) ListSuppliers(ctx context.Context, scope tenancy.Scope, actorID, cursor string, limit int) (operations.SupplierPage, error) {
	result := operations.SupplierPage{Items: make([]operations.Supplier, 0)}
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		allowed, err := tx.Authorize(ctx, scope, actorID, "operations.read")
		if err != nil {
			return err
		}
		if !allowed {
			return sales.ErrForbidden
		}
		rows, err := tx.tx.Query(ctx, `SELECT id::text,tenant_id::text,company_id::text,code,name,active,payment_terms_days FROM suppliers WHERE tenant_id=$1 AND company_id=$2 AND ($3='' OR id<NULLIF($3,'')::uuid) ORDER BY id DESC LIMIT $4`, scope.TenantID, scope.CompanyID, cursor, limit+1)
		if err != nil {
			return normalizeError(err)
		}
		defer rows.Close()
		for rows.Next() {
			var value operations.Supplier
			if err := rows.Scan(&value.ID, &value.TenantID, &value.CompanyID, &value.Code, &value.Name, &value.Active, &value.PaymentTermsDays); err != nil {
				return normalizeError(err)
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
