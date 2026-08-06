package integrations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
)

type FiscalProjectionRepository interface {
	ProjectFiscalDelivery(context.Context, outbox.Event, string, string, string) error
}

type Projector struct {
	repository FiscalProjectionRepository
	ids        identity.Generator
}

func NewProjector(repository FiscalProjectionRepository, ids identity.Generator) (*Projector, error) {
	if repository == nil || ids == nil {
		return nil, errors.New("integration projector repository and ID generator are required")
	}
	return &Projector{repository: repository, ids: ids}, nil
}

func (p *Projector) Publish(ctx context.Context, event outbox.Event) error {
	operation := ""
	switch event.EventType {
	case "sale.posted":
		operation = "SALES_RECEIPT"
	case "sale.reversed":
		operation = "SALES_RETURN"
	default:
		return nil
	}
	id, err := p.ids.New()
	if err != nil {
		return err
	}
	sum := sha256.Sum256(event.Payload)
	return p.repository.ProjectFiscalDelivery(ctx, event, id, operation, hex.EncodeToString(sum[:]))
}
