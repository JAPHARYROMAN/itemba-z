package configuration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
	"strings"
	"time"
	"unicode/utf8"
)

type Repository interface {
	ConfigurationSnapshot(context.Context, tenancy.Scope, string) (Snapshot, error)
	CreateConfiguration(context.Context, Version, string, string) (Version, error)
	TransitionConfiguration(context.Context, tenancy.Scope, string, string, Status, string, string, string, time.Time) (Version, error)
	CreateSequence(context.Context, Sequence, string, string) (Sequence, error)
	AllocateNumber(context.Context, tenancy.Scope, string, string, string, string, string, time.Time) (Allocation, error)
}
type Service struct {
	repository Repository
	ids        identity.Generator
	clock      clock.Clock
}

func NewService(r Repository, i identity.Generator, c clock.Clock) (*Service, error) {
	if r == nil || i == nil || c == nil {
		return nil, errors.New("configuration repository, id generator, and clock are required")
	}
	return &Service{r, i, c}, nil
}

type CreateCommand struct {
	Scope                           tenancy.Scope
	Category                        Category
	Key, NameEN, NameSW             string
	Value                           json.RawMessage
	SecretRef                       string
	EffectiveFrom                   time.Time
	EffectiveTo                     *time.Time
	Reason, ActorID, IdempotencyKey string
}
type TransitionCommand struct {
	Scope                           tenancy.Scope
	ID                              string
	Status                          Status
	Reason, ActorID, IdempotencyKey string
}
type SequenceCommand struct {
	Scope                   tenancy.Scope
	Key, Prefix             string
	NextValue, Padding      int64
	ActorID, IdempotencyKey string
}
type AllocateCommand struct {
	Scope                               tenancy.Scope
	SequenceID, ActorID, IdempotencyKey string
}

func (s *Service) Get(ctx context.Context, scope tenancy.Scope, actor string) (Snapshot, error) {
	return s.repository.ConfigurationSnapshot(ctx, scope.Normalize(), identity.NormalizeClaim(actor))
}
func (s *Service) Create(ctx context.Context, c CreateCommand) (Version, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Key = strings.ToUpper(strings.TrimSpace(c.Key))
	c.NameEN = strings.TrimSpace(c.NameEN)
	c.NameSW = strings.TrimSpace(c.NameSW)
	c.SecretRef = strings.TrimSpace(c.SecretRef)
	c.Reason = strings.TrimSpace(c.Reason)
	c.EffectiveFrom = date(c.EffectiveFrom)
	if c.EffectiveTo != nil {
		x := date(*c.EffectiveTo)
		c.EffectiveTo = &x
	}
	var object map[string]any
	validCategory := c.Category == Sales || c.Category == Purchasing || c.Category == Inventory || c.Category == Finance || c.Category == HR || c.Category == Numbering || c.Category == Templates || c.Category == Notifications || c.Category == Imports || c.Category == Integrations
	if c.Scope.Validate() != nil || c.ActorID == "" || !validCategory || len(c.Key) < 2 || len(c.Key) > 80 || len(c.NameEN) < 2 || len(c.NameSW) < 2 || len(c.Value) == 0 || len(c.Value) > 32768 || json.Unmarshal(c.Value, &object) != nil || object == nil || c.EffectiveFrom.IsZero() || (c.EffectiveTo != nil && c.EffectiveTo.Before(c.EffectiveFrom)) || !reason(c.Reason) || !idem(c.IdempotencyKey) {
		return Version{}, ErrInvalidCommand
	}
	id, e := s.ids.New()
	if e != nil {
		return Version{}, e
	}
	v := Version{ID: id, Scope: c.Scope, Category: c.Category, Key: c.Key, NameEN: c.NameEN, NameSW: c.NameSW, Value: append(json.RawMessage(nil), c.Value...), SecretRef: c.SecretRef, EffectiveFrom: c.EffectiveFrom, EffectiveTo: c.EffectiveTo, Status: Draft, Reason: c.Reason, CreatedBy: c.ActorID, CreatedAt: s.clock.Now().UTC()}
	return s.repository.CreateConfiguration(ctx, v, c.IdempotencyKey, hash(c))
}
func (s *Service) Transition(ctx context.Context, c TransitionCommand) (Version, error) {
	c.Scope = c.Scope.Normalize()
	c.ID = identity.NormalizeClaim(c.ID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.ID) || c.ActorID == "" || (c.Status != Submitted && c.Status != Active && c.Status != Rejected && c.Status != Retired) || !reason(c.Reason) || !idem(c.IdempotencyKey) {
		return Version{}, ErrInvalidCommand
	}
	return s.repository.TransitionConfiguration(ctx, c.Scope, c.ActorID, c.ID, c.Status, c.Reason, c.IdempotencyKey, hash(c), s.clock.Now().UTC())
}
func (s *Service) CreateSequence(ctx context.Context, c SequenceCommand) (Sequence, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Key = strings.ToUpper(strings.TrimSpace(c.Key))
	c.Prefix = strings.ToUpper(strings.TrimSpace(c.Prefix))
	if c.Scope.Validate() != nil || c.ActorID == "" || len(c.Key) < 2 || len(c.Key) > 80 || len(c.Prefix) > 30 || c.NextValue < 1 || c.Padding < 1 || c.Padding > 18 || !idem(c.IdempotencyKey) {
		return Sequence{}, ErrInvalidCommand
	}
	id, e := s.ids.New()
	if e != nil {
		return Sequence{}, e
	}
	v := Sequence{ID: id, Scope: c.Scope, Key: c.Key, Prefix: c.Prefix, NextValue: c.NextValue, Padding: c.Padding, CreatedBy: c.ActorID, CreatedAt: s.clock.Now().UTC()}
	return s.repository.CreateSequence(ctx, v, c.IdempotencyKey, hash(c))
}
func (s *Service) Allocate(ctx context.Context, c AllocateCommand) (Allocation, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.SequenceID = identity.NormalizeClaim(c.SequenceID)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.SequenceID) || c.ActorID == "" || !idem(c.IdempotencyKey) {
		return Allocation{}, ErrInvalidCommand
	}
	id, e := s.ids.New()
	if e != nil {
		return Allocation{}, e
	}
	return s.repository.AllocateNumber(ctx, c.Scope, c.ActorID, c.SequenceID, id, c.IdempotencyKey, hash(c), s.clock.Now().UTC())
}
func reason(v string) bool { n := utf8.RuneCountInString(v); return n >= 8 && n <= 500 }
func idem(v string) bool   { n := len(strings.TrimSpace(v)); return n >= 16 && n <= 128 }
func date(v time.Time) time.Time {
	y, m, d := v.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
func hash(v any) string {
	b, _ := json.Marshal(v)
	x := sha256.Sum256(b)
	return hex.EncodeToString(x[:])
}
