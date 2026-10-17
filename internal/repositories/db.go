package repositories

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"go-core/internal/utils"
)

//go:embed schema.sql
var schema string

// OpenSQLite opens (creating if needed) the SQLite database at path, applies
// the schema and verifies the connection. The PRAGMAs are set in the DSN so
// they apply to every connection in the pool.
func OpenSQLite(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return db, nil
}

// errMsgs carries the context-dependent client messages used by translateError.
type errMsgs struct{ notFound, conflict, foreignKey string }

// translateError converts a database error into an *utils.AppError (spec 7.3).
func translateError(err error, m errMsgs) error {
	if err == nil {
		return nil
	}
	var se *sqlite.Error
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return utils.NotFound(m.notFound)
	case errors.Is(err, context.DeadlineExceeded):
		return utils.Timeout(err)
	case errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE:
		return utils.Conflict(m.conflict, err)
	case errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
		return utils.BusinessRule(m.foreignKey, err)
	default:
		return utils.Internal(err)
	}
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", s, err)
	}
	return t.UTC(), nil
}
