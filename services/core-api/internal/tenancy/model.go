// Package tenancy defines the organizational boundary carried by every business
// operation. Application services must never infer or silently widen a scope.
package tenancy

import (
	"errors"
	"strings"
)

var ErrInvalidScope = errors.New("tenant, company, branch and warehouse are required")

// Scope is the smallest organizational boundary needed by an inventory sale.
// Tenant and Company are security boundaries; Branch and Warehouse are
// operational boundaries.
type Scope struct {
	TenantID    string `json:"tenant_id"`
	CompanyID   string `json:"company_id"`
	BranchID    string `json:"branch_id"`
	WarehouseID string `json:"warehouse_id"`
}

func (s Scope) Normalize() Scope {
	return Scope{
		TenantID:    strings.ToLower(strings.TrimSpace(s.TenantID)),
		CompanyID:   strings.ToLower(strings.TrimSpace(s.CompanyID)),
		BranchID:    strings.ToLower(strings.TrimSpace(s.BranchID)),
		WarehouseID: strings.ToLower(strings.TrimSpace(s.WarehouseID)),
	}
}

func (s Scope) Validate() error {
	if strings.TrimSpace(s.TenantID) == "" || strings.TrimSpace(s.CompanyID) == "" ||
		strings.TrimSpace(s.BranchID) == "" || strings.TrimSpace(s.WarehouseID) == "" {
		return ErrInvalidScope
	}
	return nil
}

func (s Scope) SameCompany(other Scope) bool {
	return s.TenantID == other.TenantID && s.CompanyID == other.CompanyID
}

type Tenant struct {
	ID   string
	Name string
}

type Company struct {
	ID                string
	TenantID          string
	Name              string
	BaseCurrency      string
	GeneralCustomerID string
}

type Branch struct {
	ID        string
	TenantID  string
	CompanyID string
	Name      string
}

type Warehouse struct {
	ID        string
	TenantID  string
	CompanyID string
	BranchID  string
	Name      string
}
