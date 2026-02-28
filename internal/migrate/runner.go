// Package migrate provides a lightweight SQL migration runner that is
// fully compatible with the golang-migrate CLI tool.
//
// Both use the same schema_migrations table (version BIGINT, dirty BOOL),
// so switching between CLI and embedded runner is transparent — neither
// will re-run migrations the other already applied.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
)

// ensureTable creates the schema_migrations table using the same schema as
// the golang-migrate CLI so the two tools remain fully compatible.
const ensureTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT  NOT NULL PRIMARY KEY,
    dirty   BOOLEAN NOT NULL DEFAULT false
)`

// Run applies all pending *.up.sql migrations from fsys.
// Migrations are identified by the numeric prefix of their filename
// (e.g. "003_add_reviews.up.sql" → version 3), matching how the
// golang-migrate CLI numbers them.
// Already-applied versions (present in schema_migrations) are skipped.
func Run(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	if _, err := db.ExecContext(ctx, ensureTable); err != nil {
		return fmt.Errorf("migrate: ensure table: %w", err)
	}

	// Load applied versions
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty ORDER BY version`)
	if err != nil {
		return fmt.Errorf("migrate: query applied: %w", err)
	}
	applied := make(map[int64]bool)
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return fmt.Errorf("migrate: scan version: %w", err)
		}
		applied[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("migrate: rows error: %w", err)
	}

	// Collect and sort *.up.sql files
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return fmt.Errorf("migrate: read dir: %w", err)
	}

	type migration struct {
		version int64
		name    string
	}
	var pending []migration

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		var num int64
		if _, err := fmt.Sscanf(name, "%d", &num); err != nil {
			return fmt.Errorf("migrate: cannot parse version from filename %q: %w", name, err)
		}
		if !applied[num] {
			pending = append(pending, migration{version: num, name: name})
		}
	}

	sort.Slice(pending, func(i, j int) bool { return pending[i].version < pending[j].version })

	if len(pending) == 0 {
		slog.Info("database schema is up to date")
		return nil
	}

	for _, m := range pending {
		content, err := fs.ReadFile(fsys, m.name)
		if err != nil {
			return fmt.Errorf("migrate: read %s: %w", m.name, err)
		}

		slog.Info("applying migration", "file", m.name, "version", m.version)

		if err := applyOne(ctx, db, m.version, string(content)); err != nil {
			return fmt.Errorf("migrate: apply %s: %w", m.name, err)
		}

		slog.Info("migration applied", "file", m.name)
	}

	return nil
}

func applyOne(ctx context.Context, db *sql.DB, version int64, sqlContent string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Mark as dirty before running — if the server crashes mid-migration
	// the CLI tool will flag it as dirty and refuse to proceed (safe default).
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, dirty) VALUES ($1, true)
		 ON CONFLICT (version) DO UPDATE SET dirty = true`, version); err != nil {
		return err
	}

	if _, err = tx.ExecContext(ctx, sqlContent); err != nil {
		return err
	}

	// Mark clean on success
	if _, err = tx.ExecContext(ctx,
		`UPDATE schema_migrations SET dirty = false WHERE version = $1`, version); err != nil {
		return err
	}

	return tx.Commit()
}
