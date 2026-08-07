// Package configuration owns governed, effective-dated ERP policy and numbering configuration.
package configuration

import (
	"encoding/json"
	"errors"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
	"time"
)

type Category string
type Status string

const (
	Sales         Category = "SALES"
	Purchasing    Category = "PURCHASING"
	Inventory     Category = "INVENTORY"
	Finance       Category = "FINANCE"
	HR            Category = "HR"
	Numbering     Category = "NUMBERING"
	Templates     Category = "TEMPLATES"
	Notifications Category = "NOTIFICATIONS"
	Imports       Category = "IMPORTS"
	Integrations  Category = "INTEGRATIONS"
	Draft         Status   = "DRAFT"
	Submitted     Status   = "SUBMITTED"
	Active        Status   = "ACTIVE"
	Rejected      Status   = "REJECTED"
	Retired       Status   = "RETIRED"
)

type Version struct {
	ID            string          `json:"id"`
	Scope         tenancy.Scope   `json:"scope"`
	Category      Category        `json:"category"`
	Key           string          `json:"key"`
	NameEN        string          `json:"name_en"`
	NameSW        string          `json:"name_sw"`
	Value         json.RawMessage `json:"value"`
	SecretRef     string          `json:"secret_ref,omitempty"`
	EffectiveFrom time.Time       `json:"effective_from"`
	EffectiveTo   *time.Time      `json:"effective_to,omitempty"`
	Status        Status          `json:"status"`
	Reason        string          `json:"reason"`
	CreatedBy     string          `json:"created_by"`
	CreatedAt     time.Time       `json:"created_at"`
	ApprovedBy    string          `json:"approved_by,omitempty"`
}
type Sequence struct {
	ID        string        `json:"id"`
	Scope     tenancy.Scope `json:"scope"`
	Key       string        `json:"key"`
	Prefix    string        `json:"prefix"`
	NextValue int64         `json:"next_value"`
	Padding   int64         `json:"padding"`
	CreatedBy string        `json:"created_by"`
	CreatedAt time.Time     `json:"created_at"`
}
type Allocation struct {
	SequenceID  string    `json:"sequence_id"`
	Number      string    `json:"number"`
	Value       int64     `json:"value"`
	AllocatedAt time.Time `json:"allocated_at"`
}
type Snapshot struct {
	Versions  []Version  `json:"versions"`
	Sequences []Sequence `json:"sequences"`
}

var (
	ErrInvalidCommand     = errors.New("configuration command is invalid")
	ErrInvalidTransition  = errors.New("configuration transition is invalid")
	ErrSeparationOfDuties = errors.New("maker cannot approve own configuration")
	ErrEffectiveOverlap   = errors.New("active configuration effective range overlaps")
)
