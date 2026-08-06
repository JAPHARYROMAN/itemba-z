// Package migrationassurance validates and stages checksum-bound migration
// batches. It deliberately does not contain source-specific transformation or
// posting rules: those adapters require an approved mapping version per source.
package migrationassurance

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	ModeSyntheticRehearsal = "SYNTHETIC_REHEARSAL"
	ModeProductionTrial    = "PRODUCTION_TRIAL"
	ModeFinalCutover       = "FINAL_CUTOVER"
	maxRowBytes            = 16 << 20
	maxRowsPerFile         = 5_000_000
)

var (
	checksumPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	versionPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	decimalPattern  = regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]{1,6})?$`)
	validModes      = setOf(ModeSyntheticRehearsal, ModeProductionTrial, ModeFinalCutover)
	validClasses    = setOf("INTERNAL", "CONFIDENTIAL", "RESTRICTED")
	validUnits      = setOf("COUNT", "MINOR_UNITS", "QUANTITY", "DAYS")
	validCategories = setOf(
		"ORGANIZATIONS", "USERS", "CUSTOMERS", "SUPPLIERS", "PRODUCTS", "UNITS",
		"PRICES", "TAX_MAPPINGS", "ACCOUNTS", "EMPLOYEES", "HISTORICAL_SUMMARIES",
		"OPEN_DOCUMENTS", "OPEN_BALANCES",
	)
	requiredProductionCategories = sortedKeys(validCategories)
)

type Manifest struct {
	SchemaVersion  int            `json:"schema_version"`
	BatchID        string         `json:"batch_id"`
	Mode           string         `json:"mode"`
	TrialNumber    *int           `json:"trial_number"`
	MappingVersion string         `json:"mapping_version"`
	CutoffAt       time.Time      `json:"cutoff_at"`
	ActorReference string         `json:"actor_reference"`
	Sources        []Source       `json:"sources"`
	ControlTotals  []ControlTotal `json:"control_totals"`
}

type Source struct {
	ID             string `json:"id"`
	Category       string `json:"category"`
	File           string `json:"file"`
	SHA256         string `json:"sha256"`
	ByteCount      int64  `json:"byte_count"`
	ExpectedRows   int64  `json:"expected_rows"`
	Classification string `json:"classification"`
	OwnerReference string `json:"owner_reference"`
}

type ControlTotal struct {
	Domain        string `json:"domain"`
	Metric        string `json:"metric"`
	Unit          string `json:"unit"`
	ExpectedValue string `json:"expected_value"`
}

type CanonicalRow struct {
	SourceKey  string         `json:"source_key"`
	TenantKey  string         `json:"tenant_key"`
	CompanyKey string         `json:"company_key"`
	Record     map[string]any `json:"record"`
}

type SourceReport struct {
	ID        string `json:"id"`
	Category  string `json:"category"`
	File      string `json:"file"`
	SHA256    string `json:"sha256"`
	ByteCount int64  `json:"byte_count"`
	Rows      int64  `json:"rows"`
}

type ValidationReport struct {
	SchemaVersion  int            `json:"schema_version"`
	BatchID        string         `json:"batch_id"`
	Mode           string         `json:"mode"`
	ManifestSHA256 string         `json:"manifest_sha256"`
	EvidenceSHA256 string         `json:"evidence_sha256"`
	Sources        []SourceReport `json:"sources"`
	TotalRows      int64          `json:"total_rows"`
	Valid          bool           `json:"valid"`
}

type ValidatedBatch struct {
	Manifest     Manifest
	ManifestPath string
	SourceRoot   string
	Report       ValidationReport
}

func Validate(manifestPath, sourceRoot string) (ValidatedBatch, error) {
	contents, err := os.ReadFile(manifestPath)
	if err != nil {
		return ValidatedBatch{}, fmt.Errorf("read migration manifest: %w", err)
	}
	var manifest Manifest
	if err := decodeStrict(contents, &manifest); err != nil {
		return ValidatedBatch{}, fmt.Errorf("decode migration manifest: %w", err)
	}
	if err := validateManifest(manifest); err != nil {
		return ValidatedBatch{}, err
	}
	root, err := filepath.Abs(sourceRoot)
	if err != nil {
		return ValidatedBatch{}, fmt.Errorf("resolve source root: %w", err)
	}
	manifestSum := sha256.Sum256(contents)
	report := ValidationReport{
		SchemaVersion:  1,
		BatchID:        manifest.BatchID,
		Mode:           manifest.Mode,
		ManifestSHA256: hex.EncodeToString(manifestSum[:]),
		Valid:          true,
	}
	for _, source := range manifest.Sources {
		sourceReport, sourceErr := validateSource(root, manifest.BatchID, source, nil)
		if sourceErr != nil {
			return ValidatedBatch{}, sourceErr
		}
		report.Sources = append(report.Sources, sourceReport)
		report.TotalRows += sourceReport.Rows
	}
	report.EvidenceSHA256 = evidenceChecksum(report)
	return ValidatedBatch{Manifest: manifest, ManifestPath: manifestPath, SourceRoot: root, Report: report}, nil
}

func validateManifest(manifest Manifest) error {
	if manifest.SchemaVersion != 1 {
		return fmt.Errorf("unsupported migration manifest schema_version %d", manifest.SchemaVersion)
	}
	if !isUUID(manifest.BatchID) {
		return errors.New("migration batch_id must be a UUID")
	}
	if !validModes[manifest.Mode] {
		return fmt.Errorf("unsupported migration mode %q", manifest.Mode)
	}
	if !versionPattern.MatchString(manifest.MappingVersion) {
		return errors.New("mapping_version must be 1-64 safe version characters")
	}
	if manifest.CutoffAt.IsZero() {
		return errors.New("cutoff_at must be an explicit RFC3339 timestamp")
	}
	if !validReference(manifest.ActorReference) {
		return errors.New("actor_reference must contain 3-200 non-whitespace characters")
	}
	if manifest.Mode == ModeFinalCutover {
		if manifest.TrialNumber != nil {
			return errors.New("FINAL_CUTOVER must not set trial_number")
		}
	} else if manifest.TrialNumber == nil || (*manifest.TrialNumber != 1 && *manifest.TrialNumber != 2) {
		return errors.New("rehearsal and production trial manifests require trial_number 1 or 2")
	}
	if len(manifest.Sources) == 0 {
		return errors.New("migration manifest requires at least one source")
	}

	seenIDs, seenFiles := map[string]bool{}, map[string]bool{}
	categories := map[string]bool{}
	for _, source := range manifest.Sources {
		if !isUUID(source.ID) {
			return fmt.Errorf("source %q id must be a UUID", source.File)
		}
		if seenIDs[source.ID] {
			return fmt.Errorf("duplicate source id %s", source.ID)
		}
		seenIDs[source.ID] = true
		if !validCategories[source.Category] {
			return fmt.Errorf("source %s has unsupported category %q", source.ID, source.Category)
		}
		categories[source.Category] = true
		if !safeBaseName(source.File) || filepath.Ext(source.File) != ".ndjson" {
			return fmt.Errorf("source %s file must be a basename ending in .ndjson", source.ID)
		}
		if seenFiles[source.File] {
			return fmt.Errorf("duplicate source file %q", source.File)
		}
		seenFiles[source.File] = true
		if !checksumPattern.MatchString(source.SHA256) {
			return fmt.Errorf("source %s has invalid SHA-256", source.ID)
		}
		if source.ByteCount < 0 || source.ExpectedRows < 0 || source.ExpectedRows > maxRowsPerFile {
			return fmt.Errorf("source %s has invalid declared size or row count", source.ID)
		}
		if !validClasses[source.Classification] || !validReference(source.OwnerReference) {
			return fmt.Errorf("source %s has invalid classification or owner_reference", source.ID)
		}
		if manifest.Mode != ModeSyntheticRehearsal && strings.HasPrefix(strings.ToUpper(source.OwnerReference), "SYNTHETIC") {
			return fmt.Errorf("source %s uses a synthetic owner in production mode", source.ID)
		}
	}
	if manifest.Mode != ModeSyntheticRehearsal {
		for _, category := range requiredProductionCategories {
			if !categories[category] {
				return fmt.Errorf("production migration manifest is missing category %s", category)
			}
		}
	}
	seenTotals := map[string]bool{}
	for _, total := range manifest.ControlTotals {
		key := total.Domain + "\x00" + total.Metric + "\x00" + total.Unit
		if !validReference(total.Domain) || !validReference(total.Metric) || !validUnits[total.Unit] || !decimalPattern.MatchString(total.ExpectedValue) || seenTotals[key] {
			return fmt.Errorf("invalid or duplicate control total %q/%q", total.Domain, total.Metric)
		}
		seenTotals[key] = true
	}
	return nil
}

func validateSource(root, batchID string, source Source, visit func(int64, string, string, CanonicalRow) error) (SourceReport, error) {
	path := filepath.Join(root, source.File)
	resolved, err := filepath.Abs(path)
	if err != nil || filepath.Dir(resolved) != root {
		return SourceReport{}, fmt.Errorf("source %s escapes the approved source root", source.ID)
	}
	file, err := os.Open(resolved)
	if err != nil {
		return SourceReport{}, fmt.Errorf("open source %s: %w", source.ID, err)
	}
	defer file.Close()
	hash := sha256.New()
	reader := io.TeeReader(file, hash)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), maxRowBytes)
	seenKeys := make(map[string]struct{})
	var rows int64
	for scanner.Scan() {
		rows++
		if rows > maxRowsPerFile {
			return SourceReport{}, fmt.Errorf("source %s exceeds the row safety limit", source.ID)
		}
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			return SourceReport{}, fmt.Errorf("source %s row %d is blank", source.ID, rows)
		}
		var row CanonicalRow
		if err := decodeStrict(raw, &row); err != nil {
			return SourceReport{}, fmt.Errorf("source %s row %d: %w", source.ID, rows, err)
		}
		if !validReference(row.SourceKey) || !validReference(row.TenantKey) || !validReference(row.CompanyKey) || row.Record == nil {
			return SourceReport{}, fmt.Errorf("source %s row %d has incomplete canonical keys or record", source.ID, rows)
		}
		keyDigest := sha256.Sum256([]byte(batchID + "\x00" + source.Category + "\x00" + row.SourceKey))
		keyHash := hex.EncodeToString(keyDigest[:])
		if _, duplicate := seenKeys[keyHash]; duplicate {
			return SourceReport{}, fmt.Errorf("source %s has duplicate source_key at row %d", source.ID, rows)
		}
		seenKeys[keyHash] = struct{}{}
		rowDigest := sha256.Sum256(raw)
		rowHash := hex.EncodeToString(rowDigest[:])
		if visit != nil {
			if err := visit(rows, rowHash, keyHash, row); err != nil {
				return SourceReport{}, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return SourceReport{}, fmt.Errorf("scan source %s: %w", source.ID, err)
	}
	info, err := file.Stat()
	if err != nil {
		return SourceReport{}, fmt.Errorf("stat source %s: %w", source.ID, err)
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if digest != source.SHA256 || info.Size() != source.ByteCount || rows != source.ExpectedRows {
		return SourceReport{}, fmt.Errorf("source %s checksum, byte count, or row count differs from the signed manifest", source.ID)
	}
	return SourceReport{ID: source.ID, Category: source.Category, File: source.File, SHA256: digest, ByteCount: info.Size(), Rows: rows}, nil
}

func decodeStrict(contents []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("JSON contains more than one value")
	}
	return nil
}

func evidenceChecksum(report ValidationReport) string {
	type evidenceSource struct {
		ID, Category, SHA256 string
		ByteCount, Rows      int64
	}
	payload := struct {
		SchemaVersion                 int
		BatchID, Mode, ManifestSHA256 string
		Sources                       []evidenceSource
	}{SchemaVersion: report.SchemaVersion, BatchID: report.BatchID, Mode: report.Mode, ManifestSHA256: report.ManifestSHA256}
	for _, source := range report.Sources {
		payload.Sources = append(payload.Sources, evidenceSource{source.ID, source.Category, source.SHA256, source.ByteCount, source.Rows})
	}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func safeBaseName(name string) bool {
	return name != "" && name == filepath.Base(name) && name != "." && name != ".." && !strings.ContainsAny(name, `/\\`)
}

func validReference(value string) bool {
	trimmed := strings.TrimSpace(value)
	return len(trimmed) >= 3 && len(trimmed) <= 200
}

func isUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	_, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil
}

func setOf(values ...string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func sortedKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
