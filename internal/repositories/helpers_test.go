package repositories

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "sub", "test.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newDeptRepo(t *testing.T) *DepartmentSQLite {
	t.Helper()
	repo, _ := newDeptRepoWithDB(t)
	return repo
}

func newDeptRepoWithDB(t *testing.T) (*DepartmentSQLite, *sql.DB) {
	t.Helper()
	db := newTestDB(t)
	repo, err := NewDepartmentSQLite(context.Background(), db)
	if err != nil {
		t.Fatalf("NewDepartmentSQLite: %v", err)
	}
	t.Cleanup(func() { repo.Close() })
	return repo, db
}

// insertEmployeeRow inserts an employee directly with SQL, so JOIN tests can
// run before EmployeeSQLite exists. It returns the new row id.
func insertEmployeeRow(t *testing.T, db *sql.DB, deptID int64, name string, deleted bool) int64 {
	t.Helper()
	now := formatTime(time.Now())
	var deletedAt any
	if deleted {
		deletedAt = now
	}
	res, err := db.Exec(
		`INSERT INTO employees (name, age, position, department_id, salary, created_at, updated_at, deleted_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		name, 30, "Developer", deptID, 1000, now, now, deletedAt)
	if err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}
	return id
}

// holdWriteLock opens a write transaction on its own connection and leaves it
// uncommitted, so every other writer on the database file blocks until the
// returned release func (also registered as cleanup) rolls it back.
func holdWriteLock(t *testing.T, db *sql.DB) (release func()) {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("db.Conn: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		conn.Close()
		t.Fatalf("BEGIN IMMEDIATE: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO departments (name, created_at) VALUES ('held', 'x')`); err != nil {
		conn.Close()
		t.Fatalf("held insert: %v", err)
	}
	var once sync.Once
	release = func() {
		once.Do(func() {
			conn.ExecContext(ctx, "ROLLBACK")
			conn.Close()
		})
	}
	t.Cleanup(release)
	return release
}
