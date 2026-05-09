package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const migrationsTable = `CREATE TABLE IF NOT EXISTS _migrations (
  id          INT NOT NULL,
  filename    VARCHAR(255) NOT NULL,
  applied_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id)
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci`

// Migrate applies any embedded SQL files in migrations/ that have not yet
// been recorded in the _migrations table. Files are applied in lexical
// order; each file is one statement, wrapped in a transaction.
//
// File naming: NNNN_description.sql where NNNN is a zero-padded integer.
// The integer becomes the migration id stored in _migrations.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, migrationsTable); err != nil {
		return fmt.Errorf("migrate: ensure _migrations: %w", err)
	}

	applied, err := loadAppliedIDs(ctx, db)
	if err != nil {
		return err
	}

	files, err := listMigrationFiles()
	if err != nil {
		return err
	}

	pending := 0
	for _, f := range files {
		id, err := parseMigrationID(f)
		if err != nil {
			return err
		}
		if applied[id] {
			continue
		}
		if err := applyOne(ctx, db, id, f); err != nil {
			return fmt.Errorf("migrate: apply %s: %w", f, err)
		}
		pending++
	}
	log.Printf("migrate: applied %d migrations", pending)
	return nil
}

func loadAppliedIDs(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	rows, err := db.QueryContext(ctx, "SELECT id FROM _migrations")
	if err != nil {
		return nil, fmt.Errorf("migrate: load applied: %w", err)
	}
	defer rows.Close()

	out := make(map[int]bool)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("migrate: scan applied: %w", err)
		}
		out[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("migrate: iterate applied: %w", err)
	}
	return out, nil
}

func listMigrationFiles() ([]string, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migrate: read embed: %w", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)
	return files, nil
}

func parseMigrationID(filename string) (int, error) {
	prefix, _, ok := strings.Cut(filename, "_")
	if !ok {
		return 0, fmt.Errorf("migrate: malformed filename %q (need NNNN_name.sql)", filename)
	}
	id, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("migrate: parse id from %q: %w", filename, err)
	}
	return id, nil
}

func applyOne(ctx context.Context, db *sql.DB, id int, filename string) error {
	body, err := fs.ReadFile(migrationsFS, "migrations/"+filename)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	stmts := splitStatements(string(body))
	if len(stmts) == 0 {
		return errors.New("empty migration body")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // best-effort

	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("exec: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO _migrations (id, filename) VALUES (?, ?)", id, filename); err != nil {
		return fmt.Errorf("record: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// splitStatements breaks a SQL file into individual statements on bare
// semicolons. Strips line comments and blank lines so the migrator can run
// without relying on the driver's multiStatements DSN flag.
func splitStatements(body string) []string {
	var clean []string
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "--") {
			continue
		}
		clean = append(clean, line)
	}
	joined := strings.Join(clean, "\n")

	var out []string
	for _, raw := range strings.Split(joined, ";") {
		s := strings.TrimSpace(raw)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
