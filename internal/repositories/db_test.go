package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"go-core/internal/testutil"
	"go-core/internal/utils"
)

func TestOpenSQLiteCreatesDirAndEnablesFK(t *testing.T) {
	db := newTestDB(t)
	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("fk=%d err=%v", fk, err)
	}
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode=%s", mode)
	}
}

func TestTranslateError(t *testing.T) {
	m := errMsgs{notFound: MsgEmployeeNotFound}
	if translateError(context.Background(), nil, m) != nil {
		t.Fatal("nil")
	}
	testutil.AssertAppError(t, translateError(context.Background(), sql.ErrNoRows, m), utils.KindNotFound, "Employee not found")
	testutil.AssertAppError(t, translateError(context.Background(), fmt.Errorf("q: %w", context.DeadlineExceeded), m), utils.KindTimeout, "request timed out")
	testutil.AssertAppError(t, translateError(context.Background(), errors.New("conn reset"), m), utils.KindInternal, "internal server error")
}

func TestTranslateErrorConstraints(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.Exec(`INSERT INTO departments (name, created_at) VALUES ('Dup', 'x')`); err != nil {
		t.Fatal(err)
	}
	_, err := db.Exec(`INSERT INTO departments (name, created_at) VALUES ('dup', 'x')`)
	m := errMsgs{conflict: MsgDepartmentExists("dup"), foreignKey: MsgDepartmentMissing(7)}
	testutil.AssertAppError(t, translateError(context.Background(), err, m), utils.KindConflict, `department "dup" already exists`)

	now := formatTime(time.Now())
	_, err = db.Exec(`INSERT INTO employees (name, age, position, department_id, salary, created_at, updated_at)
		VALUES ('x', 30, 'dev', 7, 1000, ?, ?)`, now, now)
	testutil.AssertAppError(t, translateError(context.Background(), err, m), utils.KindBusinessRule, "department 7 does not exist")
}

func TestFormatParseTimeRoundTrip(t *testing.T) {
	in := time.Date(2026, 10, 16, 8, 30, 15, 123456789, time.FixedZone("x", 7*3600))
	s := formatTime(in)
	got, err := parseTime(s)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(in) || got.Location() != time.UTC {
		t.Fatalf("got %v (%s) want %v", got, got.Location(), in)
	}
	if _, err := parseTime("not a time"); err == nil {
		t.Fatal("want parse error")
	}
}

func TestTranslateErrorAfterDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	// Any error once the deadline has passed (e.g. SQLITE_BUSY after waiting
	// out busy_timeout) is a timeout.
	testutil.AssertAppError(t, translateError(ctx, errors.New("database is locked (5) (SQLITE_BUSY)"), errMsgs{}), utils.KindTimeout, "request timed out")
	if translateError(ctx, nil, errMsgs{}) != nil {
		t.Fatal("nil error must stay nil")
	}
	// A cancelled (not expired) context does not turn errors into timeouts.
	cctx, ccancel := context.WithCancel(context.Background())
	ccancel()
	testutil.AssertAppError(t, translateError(cctx, errors.New("boom"), errMsgs{}), utils.KindInternal, "internal server error")
}
