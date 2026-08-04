// Package migrate applies ordered, checksummed SQL migrations atomically.
package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Runner struct{ Pool *pgxpool.Pool }

func (r Runner) Apply(ctx context.Context, files fs.FS) error {
	if r.Pool == nil || files == nil {
		return errors.New("migration pool and filesystem are required")
	}
	if _, err := r.Pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.itembaz_schema_migrations (
		version text PRIMARY KEY, checksum char(64) NOT NULL, applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("initialize migration ledger: %w", err)
	}
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		contents, err := fs.ReadFile(files, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		checksum := checksum(contents)
		var stored string
		err = r.Pool.QueryRow(ctx, `SELECT checksum FROM public.itembaz_schema_migrations WHERE version=$1`, name).Scan(&stored)
		if err == nil {
			if stored != checksum {
				return fmt.Errorf("migration %s checksum changed after application", name)
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("read migration state %s: %w", name, err)
		}
		tx, err := r.Pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		if _, err = tx.Exec(ctx, string(contents)); err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO public.itembaz_schema_migrations (version, checksum) VALUES ($1,$2)`, name, checksum)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}

func checksum(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}

func Directory(path string) fs.FS { return osDirectoryFS{root: filepath.Clean(path)} }

type osDirectoryFS struct{ root string }

func (f osDirectoryFS) Open(name string) (fs.File, error) { return filepathFSOpen(f.root, name) }
