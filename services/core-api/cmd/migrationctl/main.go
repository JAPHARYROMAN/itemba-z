// Command migrationctl validates, stages, and reconciles restricted migration
// batches. It must run with a dedicated migration owner connection, never an
// application runtime role.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/migrationassurance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/telemetry"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := telemetry.NewJSONLogger(os.Stderr)
	if len(os.Args) < 2 {
		logger.Error("usage: migrationctl <validate|stage|transition|reconcile|evidence> [flags]")
		os.Exit(2)
	}
	command := os.Args[1]
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	var output any
	var err error
	switch command {
	case "validate", "stage":
		flags := flag.NewFlagSet(command, flag.ContinueOnError)
		manifestPath := flags.String("manifest", "", "path to the signed migration manifest")
		sourceRoot := flags.String("source-root", "", "restricted directory containing source files")
		if parseErr := flags.Parse(os.Args[2:]); parseErr != nil || *manifestPath == "" || *sourceRoot == "" {
			logger.Error("--manifest and --source-root are required")
			os.Exit(2)
		}
		batch, validateErr := migrationassurance.Validate(*manifestPath, *sourceRoot)
		if validateErr != nil {
			err = validateErr
			break
		}
		output = batch.Report
		if command == "stage" {
			pool := openPool(ctx, logger)
			defer pool.Close()
			output, err = migrationassurance.Stage(ctx, pool, batch)
		}
	case "transition":
		flags := flag.NewFlagSet(command, flag.ContinueOnError)
		batchID := flags.String("batch-id", "", "migration batch UUID")
		expected := flags.String("expected-status", "", "required current status")
		next := flags.String("next-status", "", "next lifecycle status")
		actor := flags.String("actor", "", "approved actor or ticket reference")
		evidence := flags.String("evidence-sha256", "", "SHA-256 of retained approval evidence")
		if parseErr := flags.Parse(os.Args[2:]); parseErr != nil || *batchID == "" || *expected == "" || *next == "" || *actor == "" || *evidence == "" {
			logger.Error("transition requires --batch-id, --expected-status, --next-status, --actor, and --evidence-sha256")
			os.Exit(2)
		}
		pool := openPool(ctx, logger)
		defer pool.Close()
		err = migrationassurance.Transition(ctx, pool, *batchID, *expected, *next, *actor, *evidence)
		output = map[string]any{"schema_version": 1, "batch_id": *batchID, "from_status": *expected, "to_status": *next, "evidence_sha256": *evidence}
	case "reconcile":
		flags := flag.NewFlagSet(command, flag.ContinueOnError)
		file := flags.String("file", "", "path to the signed reconciliation JSON")
		if parseErr := flags.Parse(os.Args[2:]); parseErr != nil || *file == "" {
			logger.Error("reconcile requires --file")
			os.Exit(2)
		}
		pool := openPool(ctx, logger)
		defer pool.Close()
		output, err = migrationassurance.RecordReconciliation(ctx, pool, *file)
	case "evidence":
		flags := flag.NewFlagSet(command, flag.ContinueOnError)
		batchID := flags.String("batch-id", "", "migration batch UUID")
		if parseErr := flags.Parse(os.Args[2:]); parseErr != nil || *batchID == "" {
			logger.Error("evidence requires --batch-id")
			os.Exit(2)
		}
		pool := openPool(ctx, logger)
		defer pool.Close()
		output, err = migrationassurance.CaptureBatchEvidence(ctx, pool, *batchID)
	default:
		logger.Error("unsupported migrationctl command", "command", command)
		os.Exit(2)
	}
	if err != nil {
		logger.Error("migration assurance command failed", "command", command, "error", err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		logger.Error("encode migration report", "error", err)
		os.Exit(1)
	}
}

func openPool(ctx context.Context, logger *slog.Logger) *pgxpool.Pool {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required for database mutation commands")
		os.Exit(2)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		logger.Error("open migration database", "error", err)
		os.Exit(1)
	}
	return pool
}
