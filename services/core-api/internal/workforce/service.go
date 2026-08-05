package workforce

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type Repository interface {
	WorkforceSnapshot(context.Context, tenancy.Scope, string) (Snapshot, error)
	CreateShiftTemplate(context.Context, ShiftTemplate, string, string) (ShiftTemplate, error)
	TransitionShiftTemplate(context.Context, tenancy.Scope, string, string, Status, string, string, string, time.Time) (ShiftTemplate, error)
	CreateShiftAssignment(context.Context, Assignment, string, string) (Assignment, error)
	TransitionShiftAssignment(context.Context, tenancy.Scope, string, string, Status, string, string, string, time.Time) (Assignment, error)
	RegisterEmployeeDocument(context.Context, EmployeeDocument, string, string) (EmployeeDocument, error)
	GeneratePayrollArtifact(context.Context, PayrollArtifact, string, string) (PayrollArtifact, error)
}
type Service struct {
	repository Repository
	ids        identity.Generator
	clock      clock.Clock
}

func NewService(r Repository, ids identity.Generator, c clock.Clock) (*Service, error) {
	if r == nil || ids == nil || c == nil {
		return nil, errors.New("workforce repository, IDs, and clock are required")
	}
	return &Service{r, ids, c}, nil
}

type ShiftCommand struct {
	Scope                                             tenancy.Scope
	Code, NameEN, NameSW                              string
	StartMinute, EndMinute, BreakMinutes, WeekdayMask int64
	EffectiveFrom                                     time.Time
	EffectiveTo                                       *time.Time
	Reason, ActorID, IdempotencyKey                   string
}
type AssignmentCommand struct {
	Scope                           tenancy.Scope
	EmployeeID, ShiftTemplateID     string
	StartsOn, EndsOn                time.Time
	Reason, ActorID, IdempotencyKey string
}
type TransitionCommand struct {
	Scope                           tenancy.Scope
	ID                              string
	Status                          Status
	Reason, ActorID, IdempotencyKey string
}
type DocumentCommand struct {
	Scope                                                         tenancy.Scope
	EmployeeID, DocumentType, Title, ObjectKey, SHA256, MediaType string
	Classification                                                Classification
	IssuedOn, ExpiresOn                                           *time.Time
	Reason, ActorID, IdempotencyKey                               string
}
type ExportCommand struct {
	Scope                         tenancy.Scope
	PayrollRunID, ConfigurationID string
	Format                        ExportFormat
	ActorID, IdempotencyKey       string
}

func (s *Service) Get(ctx context.Context, scope tenancy.Scope, actor string) (Snapshot, error) {
	return s.repository.WorkforceSnapshot(ctx, scope.Normalize(), identity.NormalizeClaim(actor))
}
func (s *Service) CreateShift(ctx context.Context, c ShiftCommand) (ShiftTemplate, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Code = strings.ToUpper(strings.TrimSpace(c.Code))
	c.NameEN = strings.TrimSpace(c.NameEN)
	c.NameSW = strings.TrimSpace(c.NameSW)
	c.Reason = strings.TrimSpace(c.Reason)
	c.EffectiveFrom = date(c.EffectiveFrom)
	c.EffectiveTo = datePtr(c.EffectiveTo)
	if c.Scope.Validate() != nil || c.ActorID == "" || len(c.Code) < 2 || len(c.Code) > 40 || len(c.NameEN) < 2 || len(c.NameSW) < 2 || c.StartMinute < 0 || c.StartMinute > 1439 || c.EndMinute < 0 || c.EndMinute > 1439 || c.StartMinute == c.EndMinute || c.BreakMinutes < 0 || c.BreakMinutes > 720 || c.WeekdayMask < 1 || c.WeekdayMask > 127 || c.EffectiveFrom.IsZero() || (c.EffectiveTo != nil && c.EffectiveTo.Before(c.EffectiveFrom)) || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return ShiftTemplate{}, ErrInvalidCommand
	}
	id, e := s.ids.New()
	if e != nil {
		return ShiftTemplate{}, e
	}
	v := ShiftTemplate{ID: id, Scope: c.Scope, Code: c.Code, NameEN: c.NameEN, NameSW: c.NameSW, Status: Draft, StartMinute: c.StartMinute, EndMinute: c.EndMinute, BreakMinutes: c.BreakMinutes, WeekdayMask: c.WeekdayMask, EffectiveFrom: c.EffectiveFrom, EffectiveTo: c.EffectiveTo, Reason: c.Reason, CreatedBy: c.ActorID, CreatedAt: s.clock.Now().UTC()}
	return s.repository.CreateShiftTemplate(ctx, v, c.IdempotencyKey, hash(c))
}
func (s *Service) TransitionShift(ctx context.Context, c TransitionCommand) (ShiftTemplate, error) {
	if !validTransition(&c) {
		return ShiftTemplate{}, ErrInvalidCommand
	}
	return s.repository.TransitionShiftTemplate(ctx, c.Scope, c.ActorID, c.ID, c.Status, c.Reason, c.IdempotencyKey, hash(c), s.clock.Now().UTC())
}
func (s *Service) CreateAssignment(ctx context.Context, c AssignmentCommand) (Assignment, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.EmployeeID = identity.NormalizeClaim(c.EmployeeID)
	c.ShiftTemplateID = identity.NormalizeClaim(c.ShiftTemplateID)
	c.StartsOn = date(c.StartsOn)
	c.EndsOn = date(c.EndsOn)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.EmployeeID) || !identity.IsUUID(c.ShiftTemplateID) || c.ActorID == "" || c.StartsOn.IsZero() || c.EndsOn.Before(c.StartsOn) || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return Assignment{}, ErrInvalidCommand
	}
	id, e := s.ids.New()
	if e != nil {
		return Assignment{}, e
	}
	v := Assignment{ID: id, Scope: c.Scope, EmployeeID: c.EmployeeID, ShiftTemplateID: c.ShiftTemplateID, Status: Draft, StartsOn: c.StartsOn, EndsOn: c.EndsOn, Reason: c.Reason, CreatedBy: c.ActorID, CreatedAt: s.clock.Now().UTC()}
	return s.repository.CreateShiftAssignment(ctx, v, c.IdempotencyKey, hash(c))
}
func (s *Service) TransitionAssignment(ctx context.Context, c TransitionCommand) (Assignment, error) {
	if !validTransition(&c) {
		return Assignment{}, ErrInvalidCommand
	}
	return s.repository.TransitionShiftAssignment(ctx, c.Scope, c.ActorID, c.ID, c.Status, c.Reason, c.IdempotencyKey, hash(c), s.clock.Now().UTC())
}
func (s *Service) RegisterDocument(ctx context.Context, c DocumentCommand) (EmployeeDocument, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.EmployeeID = identity.NormalizeClaim(c.EmployeeID)
	c.DocumentType = strings.ToUpper(strings.TrimSpace(c.DocumentType))
	c.Title = strings.TrimSpace(c.Title)
	c.ObjectKey = strings.TrimSpace(c.ObjectKey)
	c.SHA256 = strings.ToLower(strings.TrimSpace(c.SHA256))
	c.MediaType = strings.ToLower(strings.TrimSpace(c.MediaType))
	c.Reason = strings.TrimSpace(c.Reason)
	c.IssuedOn = datePtr(c.IssuedOn)
	c.ExpiresOn = datePtr(c.ExpiresOn)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.EmployeeID) || c.ActorID == "" || len(c.DocumentType) < 2 || len(c.DocumentType) > 60 || len(c.Title) < 2 || len(c.Title) > 200 || len(c.ObjectKey) < 3 || len(c.ObjectKey) > 500 || len(c.SHA256) != 64 || !isHex(c.SHA256) || len(c.MediaType) < 3 || (c.Classification != Internal && c.Classification != Confidential && c.Classification != Restricted) || (c.IssuedOn != nil && c.ExpiresOn != nil && c.ExpiresOn.Before(*c.IssuedOn)) || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return EmployeeDocument{}, ErrInvalidCommand
	}
	id, e := s.ids.New()
	if e != nil {
		return EmployeeDocument{}, e
	}
	v := EmployeeDocument{ID: id, Scope: c.Scope, EmployeeID: c.EmployeeID, DocumentType: c.DocumentType, Title: c.Title, ObjectKey: c.ObjectKey, SHA256: c.SHA256, MediaType: c.MediaType, Classification: c.Classification, IssuedOn: c.IssuedOn, ExpiresOn: c.ExpiresOn, Reason: c.Reason, CreatedBy: c.ActorID, CreatedAt: s.clock.Now().UTC()}
	return s.repository.RegisterEmployeeDocument(ctx, v, c.IdempotencyKey, hash(c))
}
func (s *Service) GenerateExport(ctx context.Context, c ExportCommand) (PayrollArtifact, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.PayrollRunID = identity.NormalizeClaim(c.PayrollRunID)
	c.ConfigurationID = identity.NormalizeClaim(c.ConfigurationID)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.PayrollRunID) || !identity.IsUUID(c.ConfigurationID) || c.ActorID == "" || (c.Format != BankCSV && c.Format != StatutoryCSV) || !validIdem(c.IdempotencyKey) {
		return PayrollArtifact{}, ErrInvalidCommand
	}
	id, e := s.ids.New()
	if e != nil {
		return PayrollArtifact{}, e
	}
	v := PayrollArtifact{ID: id, Scope: c.Scope, PayrollRunID: c.PayrollRunID, ConfigurationID: c.ConfigurationID, Format: c.Format, MediaType: "text/csv", CreatedBy: c.ActorID, CreatedAt: s.clock.Now().UTC()}
	return s.repository.GeneratePayrollArtifact(ctx, v, c.IdempotencyKey, hash(c))
}
func validTransition(c *TransitionCommand) bool {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.ID = identity.NormalizeClaim(c.ID)
	c.Reason = strings.TrimSpace(c.Reason)
	return c.Scope.Validate() == nil && identity.IsUUID(c.ID) && c.ActorID != "" && (c.Status == Submitted || c.Status == Active || c.Status == Approved || c.Status == Rejected) && validReason(c.Reason) && validIdem(c.IdempotencyKey)
}
func validReason(v string) bool { n := utf8.RuneCountInString(v); return n >= 8 && n <= 500 }
func validIdem(v string) bool   { n := len(strings.TrimSpace(v)); return n >= 16 && n <= 128 }
func date(v time.Time) time.Time {
	if v.IsZero() {
		return v
	}
	y, m, d := v.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
func datePtr(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	x := date(*v)
	return &x
}
func hash(v any) string {
	b, _ := json.Marshal(v)
	x := sha256.Sum256(b)
	return hex.EncodeToString(x[:])
}
func isHex(v string) bool { _, e := hex.DecodeString(v); return e == nil }
