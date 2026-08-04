package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
)

var _ sales.Repository = (*Store)(nil)
var _ outbox.DeliveryRepository = (*Store)(nil)

func TestSchemaValidationRejectsSQLIdentifiers(t *testing.T) {
	for _, value := range []string{"", "ItembaZ", "itembaz;drop schema public", "two schemas", "1schema"} {
		if schemaPattern.MatchString(value) {
			t.Errorf("unsafe schema accepted: %q", value)
		}
	}
	for _, value := range []string{"itembaz", "itembaz_test_123"} {
		if !schemaPattern.MatchString(value) {
			t.Errorf("safe schema rejected: %q", value)
		}
	}
}

func TestNormalizeErrorMapsDomainRelevantDatabaseErrors(t *testing.T) {
	if !errors.Is(normalizeError(pgx.ErrNoRows), sales.ErrNotFound) {
		t.Fatal("no rows was not mapped")
	}
	duplicateReversal := &pgconn.PgError{Code: "23505", ConstraintName: "one_reversal_per_sale"}
	if !errors.Is(normalizeError(duplicateReversal), sales.ErrAlreadyReversed) {
		t.Fatal("duplicate reversal was not mapped")
	}
	sentinel := errors.New("connection lost")
	if !errors.Is(normalizeError(sentinel), sentinel) {
		t.Fatal("unrecognized error was replaced")
	}
}
