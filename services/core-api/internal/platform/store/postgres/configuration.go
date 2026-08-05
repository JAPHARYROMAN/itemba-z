package postgres

import (
	"context"
	"github.com/itemba-z/itemba-z/services/core-api/internal/configuration"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
	"time"
)

func (s *Store) ConfigurationSnapshot(ctx context.Context, scope tenancy.Scope, actor string) (configuration.Snapshot, error) {
	r := configuration.Snapshot{Versions: []configuration.Version{}, Sequences: []configuration.Sequence{}}
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "settings.read")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		rows, e := tx.tx.Query(ctx, `SELECT id::text,category,config_key,name_en,name_sw,value,secret_ref,effective_from,effective_to,status,reason,created_by::text,created_at,COALESCE(approved_by::text,'') FROM configuration_versions WHERE tenant_id=$1 AND company_id=$2 ORDER BY category,config_key,effective_from DESC`, scope.TenantID, scope.CompanyID)
		if e != nil {
			return normalizeError(e)
		}
		for rows.Next() {
			v := configuration.Version{Scope: scope}
			if e = rows.Scan(&v.ID, &v.Category, &v.Key, &v.NameEN, &v.NameSW, &v.Value, &v.SecretRef, &v.EffectiveFrom, &v.EffectiveTo, &v.Status, &v.Reason, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy); e != nil {
				rows.Close()
				return normalizeError(e)
			}
			r.Versions = append(r.Versions, v)
		}
		rows.Close()
		rows, e = tx.tx.Query(ctx, `SELECT id::text,sequence_key,prefix,next_value,padding,created_by::text,created_at FROM number_sequences WHERE tenant_id=$1 AND company_id=$2 ORDER BY sequence_key`, scope.TenantID, scope.CompanyID)
		if e != nil {
			return normalizeError(e)
		}
		for rows.Next() {
			v := configuration.Sequence{Scope: scope}
			if e = rows.Scan(&v.ID, &v.Key, &v.Prefix, &v.NextValue, &v.Padding, &v.CreatedBy, &v.CreatedAt); e != nil {
				rows.Close()
				return normalizeError(e)
			}
			r.Sequences = append(r.Sequences, v)
		}
		rows.Close()
		return nil
	})
	return r, err
}
func (s *Store) CreateConfiguration(ctx context.Context, v configuration.Version, idem, hash string) (configuration.Version, error) {
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		ok, e := tx.Authorize(ctx, v.Scope, v.CreatedBy, "settings.manage")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acq, id, e := tx.ClaimIdempotency(ctx, v.Scope, "settings.configuration.create.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			v.ID = id
			return nil
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO configuration_versions(id,tenant_id,company_id,category,config_key,name_en,name_sw,value,secret_ref,effective_from,effective_to,status,reason,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.Category, v.Key, v.NameEN, v.NameSW, v.Value, v.SecretRef, v.EffectiveFrom, v.EffectiveTo, v.Status, v.Reason, v.CreatedBy, v.CreatedAt)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, v.Scope, v.CreatedBy, "settings.configuration_created", "configuration", v.ID, v.ID, v.CreatedAt); e != nil {
			return e
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "settings.configuration.create.v1", idem, v.ID)
	})
	return v, err
}
func (s *Store) TransitionConfiguration(ctx context.Context, scope tenancy.Scope, actor, id string, to configuration.Status, reason, idem, hash string, at time.Time) (configuration.Version, error) {
	v := configuration.Version{ID: id, Scope: scope}
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		perm := "settings.manage"
		if to == configuration.Active || to == configuration.Rejected || to == configuration.Retired {
			perm = "settings.approve"
		}
		ok, e := tx.Authorize(ctx, scope, actor, perm)
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		op := "settings.configuration.transition." + string(to) + ".v1"
		acq, _, e := tx.ClaimIdempotency(ctx, scope, op, idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			v.Status = to
			return nil
		}
		if e = tx.tx.QueryRow(ctx, `SELECT category,config_key,name_en,name_sw,value,secret_ref,effective_from,effective_to,status,reason,created_by::text,created_at,COALESCE(approved_by::text,'') FROM configuration_versions WHERE tenant_id=$1 AND company_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.CompanyID, id).Scan(&v.Category, &v.Key, &v.NameEN, &v.NameSW, &v.Value, &v.SecretRef, &v.EffectiveFrom, &v.EffectiveTo, &v.Status, &v.Reason, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy); e != nil {
			return normalizeError(e)
		}
		from := v.Status
		valid := from == configuration.Draft && to == configuration.Submitted || from == configuration.Submitted && (to == configuration.Active || to == configuration.Rejected) || from == configuration.Active && to == configuration.Retired
		if !valid {
			return configuration.ErrInvalidTransition
		}
		if (to == configuration.Active || to == configuration.Rejected) && actor == v.CreatedBy {
			return configuration.ErrSeparationOfDuties
		}
		if to == configuration.Active {
			var overlap bool
			if e = tx.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM configuration_versions WHERE tenant_id=$1 AND company_id=$2 AND category=$3 AND config_key=$4 AND status='ACTIVE' AND id<>$5 AND COALESCE(effective_to,'infinity'::date)>=$6 AND COALESCE($7::date,'infinity'::date)>=effective_from)`, scope.TenantID, scope.CompanyID, v.Category, v.Key, id, v.EffectiveFrom, v.EffectiveTo).Scan(&overlap); e != nil {
				return normalizeError(e)
			}
			if overlap {
				return configuration.ErrEffectiveOverlap
			}
		}
		_, e = tx.tx.Exec(ctx, `UPDATE configuration_versions SET status=$1,approved_by=CASE WHEN $1 IN('ACTIVE','REJECTED') THEN $2::uuid ELSE approved_by END,approved_at=CASE WHEN $1 IN('ACTIVE','REJECTED') THEN $3 ELSE approved_at END WHERE tenant_id=$4 AND company_id=$5 AND id=$6`, to, actor, at, scope.TenantID, scope.CompanyID, id)
		if e != nil {
			return normalizeError(e)
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO configuration_transitions(tenant_id,company_id,configuration_id,from_status,to_status,reason,actor_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, scope.TenantID, scope.CompanyID, id, from, to, reason, actor, at)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "settings.configuration_"+string(to), "configuration", id, id, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, op, idem, id); e != nil {
			return e
		}
		v.Status = to
		if to == configuration.Active {
			v.ApprovedBy = actor
		}
		return nil
	})
	return v, err
}
func (s *Store) CreateSequence(ctx context.Context, v configuration.Sequence, idem, hash string) (configuration.Sequence, error) {
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		ok, e := tx.Authorize(ctx, v.Scope, v.CreatedBy, "settings.numbering.manage")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acq, id, e := tx.ClaimIdempotency(ctx, v.Scope, "settings.sequence.create.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			v.ID = id
			return nil
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO number_sequences(id,tenant_id,company_id,sequence_key,prefix,next_value,padding,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.Key, v.Prefix, v.NextValue, v.Padding, v.CreatedBy, v.CreatedAt)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, v.Scope, v.CreatedBy, "settings.sequence_created", "number_sequence", v.ID, v.ID, v.CreatedAt); e != nil {
			return e
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "settings.sequence.create.v1", idem, v.ID)
	})
	return v, err
}
func (s *Store) AllocateNumber(ctx context.Context, scope tenancy.Scope, actor, sequenceID, allocationID, idem, hash string, at time.Time) (configuration.Allocation, error) {
	var a configuration.Allocation
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "settings.numbering.allocate")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		op := "settings.sequence.allocate.v1"
		acq, resultID, e := tx.ClaimIdempotency(ctx, scope, op, idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			return tx.tx.QueryRow(ctx, `SELECT sequence_id::text,allocated_number,allocated_value,allocated_at FROM number_allocations WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, scope.TenantID, scope.CompanyID, resultID).Scan(&a.SequenceID, &a.Number, &a.Value, &a.AllocatedAt)
		}
		var prefix string
		var padding int64
		if e = tx.tx.QueryRow(ctx, `UPDATE number_sequences SET next_value=next_value+1 WHERE tenant_id=$1 AND company_id=$2 AND id=$3 RETURNING id::text,prefix,next_value-1,padding`, scope.TenantID, scope.CompanyID, sequenceID).Scan(&a.SequenceID, &prefix, &a.Value, &padding); e != nil {
			return normalizeError(e)
		}
		if e = tx.tx.QueryRow(ctx, `SELECT $1::text||lpad($2::bigint::text,$3::bigint::int,'0')`, prefix, a.Value, padding).Scan(&a.Number); e != nil {
			return normalizeError(e)
		}
		a.AllocatedAt = at
		_, e = tx.tx.Exec(ctx, `INSERT INTO number_allocations(id,tenant_id,company_id,sequence_id,allocated_value,allocated_number,allocated_by,allocated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, allocationID, scope.TenantID, scope.CompanyID, a.SequenceID, a.Value, a.Number, actor, at)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "settings.number_allocated", "number_sequence", sequenceID, allocationID, at); e != nil {
			return e
		}
		return tx.CompleteIdempotency(ctx, scope, op, idem, allocationID)
	})
	return a, err
}
