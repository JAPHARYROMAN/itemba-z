// Package outbox defines durable domain events written in the same transaction
// as their aggregate changes.
package outbox

import (
	"encoding/json"
	"time"
)

type Event struct {
	ID            string
	TenantID      string
	CompanyID     string
	AggregateType string
	AggregateID   string
	EventType     string
	Version       int
	CorrelationID string
	CausationID   string
	Payload       json.RawMessage
	OccurredAt    time.Time
}
