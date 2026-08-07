package migrationassurance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateSyntheticManifest(t *testing.T) {
	dir := t.TempDir()
	row := []byte(`{"source_key":"customer-1","tenant_key":"itemba","company_key":"itemba-trading","record":{"name":"Synthetic Customer"}}` + "\n")
	manifestPath := writeTestManifest(t, dir, row, ModeSyntheticRehearsal)

	batch, err := Validate(manifestPath, dir)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !batch.Report.Valid || batch.Report.TotalRows != 1 || len(batch.Report.Sources) != 1 {
		t.Fatalf("unexpected validation report: %+v", batch.Report)
	}
	if !checksumPattern.MatchString(batch.Report.ManifestSHA256) || !checksumPattern.MatchString(batch.Report.EvidenceSHA256) {
		t.Fatalf("expected deterministic checksums: %+v", batch.Report)
	}
}

func TestValidateRejectsSourceChangedAfterManifest(t *testing.T) {
	dir := t.TempDir()
	row := []byte(`{"source_key":"customer-1","tenant_key":"itemba","company_key":"itemba-trading","record":{"name":"Synthetic Customer"}}` + "\n")
	manifestPath := writeTestManifest(t, dir, row, ModeSyntheticRehearsal)
	tampered := []byte(strings.Replace(string(row), "Synthetic Customer", "Changed Customer", 1))
	if err := os.WriteFile(filepath.Join(dir, "customers.ndjson"), tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Validate(manifestPath, dir)
	if err == nil || !strings.Contains(err.Error(), "differs from the signed manifest") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestValidateRejectsDuplicateSourceKeyWithoutLeakingKey(t *testing.T) {
	dir := t.TempDir()
	row := `{"source_key":"private-customer-key","tenant_key":"itemba","company_key":"itemba-trading","record":{"name":"Synthetic"}}` + "\n"
	contents := []byte(row + row)
	manifestPath := writeTestManifest(t, dir, contents, ModeSyntheticRehearsal)
	_, err := Validate(manifestPath, dir)
	if err == nil || !strings.Contains(err.Error(), "duplicate source_key") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	if strings.Contains(err.Error(), "private-customer-key") {
		t.Fatal("validation error leaked a source key")
	}
}

func TestValidateProductionRequiresCompleteCategorySet(t *testing.T) {
	dir := t.TempDir()
	row := []byte(`{"source_key":"customer-1","tenant_key":"itemba","company_key":"itemba-trading","record":{"name":"Synthetic Customer"}}` + "\n")
	manifestPath := writeTestManifest(t, dir, row, ModeProductionTrial)
	_, err := Validate(manifestPath, dir)
	if err == nil || !strings.Contains(err.Error(), "missing category") {
		t.Fatalf("expected production category failure, got %v", err)
	}
}

func TestDecodeStrictRejectsUnknownFields(t *testing.T) {
	var row CanonicalRow
	err := decodeStrict([]byte(`{"source_key":"a","tenant_key":"b","company_key":"c","record":{},"unexpected":true}`), &row)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected strict JSON error, got %v", err)
	}
}

func writeTestManifest(t *testing.T, dir string, rows []byte, mode string) string {
	t.Helper()
	sourcePath := filepath.Join(dir, "customers.ndjson")
	if err := os.WriteFile(sourcePath, rows, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(rows)
	trial := 1
	ownerReference := "synthetic-fixture-owner"
	if mode != ModeSyntheticRehearsal {
		ownerReference = "migration-data-owner"
	}
	manifest := Manifest{
		SchemaVersion:  1,
		BatchID:        "11111111-1111-4111-8111-111111111111",
		Mode:           mode,
		TrialNumber:    &trial,
		MappingVersion: "synthetic-v1",
		CutoffAt:       time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC),
		ActorReference: "qualification-workflow",
		Sources: []Source{{
			ID: "22222222-2222-4222-8222-222222222222", Category: "CUSTOMERS", File: "customers.ndjson",
			SHA256: hex.EncodeToString(sum[:]), ByteCount: int64(len(rows)), ExpectedRows: int64(bytesCount(rows, '\n')),
			Classification: "INTERNAL", OwnerReference: ownerReference,
		}},
		ControlTotals: []ControlTotal{{Domain: "CUSTOMERS", Metric: "row_count", Unit: "COUNT", ExpectedValue: "1"}},
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manifestPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return manifestPath
}

func bytesCount(contents []byte, target byte) int {
	count := 0
	for _, value := range contents {
		if value == target {
			count++
		}
	}
	return count
}
