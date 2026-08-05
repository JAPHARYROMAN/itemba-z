package memory

import (
	"context"
	"fmt"
	"github.com/itemba-z/itemba-z/services/core-api/internal/configuration"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
	"sort"
	"time"
)

func (s *Store) ConfigurationSnapshot(_ context.Context, scope tenancy.Scope, actor string) (configuration.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := configuration.Snapshot{Versions: []configuration.Version{}, Sequences: []configuration.Sequence{}}
	if !s.state.permissions[permissionKey(scope, actor, "settings.read")] {
		return r, sales.ErrForbidden
	}
	for _, v := range s.state.configurations {
		if v.Scope.TenantID == scope.TenantID && v.Scope.CompanyID == scope.CompanyID {
			v.Value = append([]byte(nil), v.Value...)
			r.Versions = append(r.Versions, v)
		}
	}
	for _, v := range s.state.numberSequences {
		if v.Scope.TenantID == scope.TenantID && v.Scope.CompanyID == scope.CompanyID {
			r.Sequences = append(r.Sequences, v)
		}
	}
	sort.Slice(r.Versions, func(i, j int) bool { return r.Versions[i].Key < r.Versions[j].Key })
	return r, nil
}
func (s *Store) CreateConfiguration(_ context.Context, v configuration.Version, idem, hash string) (configuration.Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, v.CreatedBy, "settings.manage")] {
		return v, sales.ErrForbidden
	}
	if x, ok, e := s.memoryIdem(v.Scope, "settings.configuration.create.v1", idem, hash); e != nil {
		return v, e
	} else if ok {
		return s.state.configurations[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, x)], nil
	}
	s.state.configurations[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.ID)] = v
	s.putIdem(v.Scope, "settings.configuration.create.v1", idem, hash, v.ID)
	return v, nil
}
func (s *Store) TransitionConfiguration(_ context.Context, scope tenancy.Scope, actor, id string, to configuration.Status, _ string, idem, hash string, _ time.Time) (configuration.Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	perm := "settings.manage"
	if to == configuration.Active || to == configuration.Rejected || to == configuration.Retired {
		perm = "settings.approve"
	}
	if !s.state.permissions[permissionKey(scope, actor, perm)] {
		return configuration.Version{}, sales.ErrForbidden
	}
	k := companyEntityKey(scope.TenantID, scope.CompanyID, id)
	v, ok := s.state.configurations[k]
	if !ok {
		return v, sales.ErrNotFound
	}
	op := "settings.configuration.transition." + string(to) + ".v1"
	if x, hit, e := s.memoryIdem(scope, op, idem, hash); e != nil {
		return v, e
	} else if hit {
		return s.state.configurations[companyEntityKey(scope.TenantID, scope.CompanyID, x)], nil
	}
	valid := v.Status == configuration.Draft && to == configuration.Submitted || v.Status == configuration.Submitted && (to == configuration.Active || to == configuration.Rejected) || v.Status == configuration.Active && to == configuration.Retired
	if !valid {
		return v, configuration.ErrInvalidTransition
	}
	if (to == configuration.Active || to == configuration.Rejected) && actor == v.CreatedBy {
		return v, configuration.ErrSeparationOfDuties
	}
	if to == configuration.Active {
		for _, x := range s.state.configurations {
			if x.ID != v.ID && x.Category == v.Category && x.Key == v.Key && x.Status == configuration.Active && rangesOverlap(x.EffectiveFrom, x.EffectiveTo, v.EffectiveFrom, v.EffectiveTo) {
				return v, configuration.ErrEffectiveOverlap
			}
		}
		v.ApprovedBy = actor
	}
	v.Status = to
	s.state.configurations[k] = v
	s.putIdem(scope, op, idem, hash, id)
	return v, nil
}
func rangesOverlap(a time.Time, ae *time.Time, b time.Time, be *time.Time) bool {
	return (ae == nil || !ae.Before(b)) && (be == nil || !be.Before(a))
}
func (s *Store) CreateSequence(_ context.Context, v configuration.Sequence, idem, hash string) (configuration.Sequence, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, v.CreatedBy, "settings.numbering.manage")] {
		return v, sales.ErrForbidden
	}
	if x, ok, e := s.memoryIdem(v.Scope, "settings.sequence.create.v1", idem, hash); e != nil {
		return v, e
	} else if ok {
		return s.state.numberSequences[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, x)], nil
	}
	for _, x := range s.state.numberSequences {
		if same(x.Scope, v.Scope) && x.Key == v.Key {
			return v, configuration.ErrInvalidCommand
		}
	}
	s.state.numberSequences[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.ID)] = v
	s.putIdem(v.Scope, "settings.sequence.create.v1", idem, hash, v.ID)
	return v, nil
}
func (s *Store) AllocateNumber(_ context.Context, scope tenancy.Scope, actor, sequenceID, allocationID, idem, hash string, at time.Time) (configuration.Allocation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "settings.numbering.allocate")] {
		return configuration.Allocation{}, sales.ErrForbidden
	}
	op := "settings.sequence.allocate.v1"
	if x, ok, e := s.memoryIdem(scope, op, idem, hash); e != nil {
		return configuration.Allocation{}, e
	} else if ok {
		return s.state.numberAllocations[x], nil
	}
	k := companyEntityKey(scope.TenantID, scope.CompanyID, sequenceID)
	seq, ok := s.state.numberSequences[k]
	if !ok {
		return configuration.Allocation{}, sales.ErrNotFound
	}
	a := configuration.Allocation{SequenceID: seq.ID, Value: seq.NextValue, Number: fmt.Sprintf("%s%0*d", seq.Prefix, int(seq.Padding), seq.NextValue), AllocatedAt: at}
	seq.NextValue++
	s.state.numberSequences[k] = seq
	s.state.numberAllocations[allocationID] = a
	s.putIdem(scope, op, idem, hash, allocationID)
	return a, nil
}
