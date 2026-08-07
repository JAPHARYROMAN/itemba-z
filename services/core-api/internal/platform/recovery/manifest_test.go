package recovery

import (
	"errors"
	"reflect"
	"testing"
)

type fakeRows struct {
	values []string
	index  int
	err    error
}

func (rows *fakeRows) Next() bool { return rows.index < len(rows.values) }
func (rows *fakeRows) Scan(destination ...any) error {
	*(destination[0].(*string)) = rows.values[rows.index]
	rows.index++
	return nil
}
func (rows *fakeRows) Err() error { return rows.err }
func (rows *fakeRows) Close()     {}

func TestDigestRowsIsDeterministicAndBoundarySafe(t *testing.T) {
	countA, digestA, err := digestRows(&fakeRows{values: []string{"ab", "c"}})
	if err != nil {
		t.Fatal(err)
	}
	countB, digestB, err := digestRows(&fakeRows{values: []string{"a", "bc"}})
	if err != nil {
		t.Fatal(err)
	}
	if countA != 2 || countB != 2 || digestA == digestB {
		t.Fatalf("unsafe digests: %d %s / %d %s", countA, digestA, countB, digestB)
	}
	_, digestAgain, _ := digestRows(&fakeRows{values: []string{"ab", "c"}})
	if digestAgain != digestA {
		t.Fatal("same ordered records produced different digest")
	}
}

func TestDigestRowsPropagatesIterationFailure(t *testing.T) {
	expected := errors.New("stream failed")
	_, _, err := digestRows(&fakeRows{err: expected})
	if !errors.Is(err, expected) {
		t.Fatalf("error=%v", err)
	}
}

func TestRequiredRecoveryTablesRemainExplicit(t *testing.T) {
	expected := []string{"journals", "journal_lines", "inventory_stock_ledger", "customer_ledger", "supplier_ledger", "audit_events", "outbox_events", "integration_deliveries", "employee_documents", "payroll_export_artifacts", "report_exports"}
	if !reflect.DeepEqual(requiredTables, expected) {
		t.Fatalf("recovery tables=%v", requiredTables)
	}
}
