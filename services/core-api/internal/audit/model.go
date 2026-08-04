// Package audit defines immutable, actor-attributed audit events.
package audit

import (
	"encoding/json"
	"time"
)

type Event struct {
	ID            string
	TenantID      string
	CompanyID     string
	ActorID       string
	Action        string
	EntityType    string
	EntityID      string
	CorrelationID string
	CausationID   string
	Data          json.RawMessage
	OccurredAt    time.Time
}
