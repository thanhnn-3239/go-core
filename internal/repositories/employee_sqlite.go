package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-core/internal/models"
	"go-core/internal/utils"
)

const employeeColumns = `id, name, age, position, department_id, salary, created_at, updated_at`

// EmployeeSQLite implements EmployeeRepository on SQLite with statements
// prepared once at construction. db is kept for the transactions in List
// and Update.
type EmployeeSQLite struct {
	db                                                            *sql.DB
	insert, getByID, count, list, search, softDelete, all, update *sql.Stmt
}

var _ EmployeeRepository = (*EmployeeSQLite)(nil)

func NewEmployeeSQLite(ctx context.Context, db *sql.DB) (*EmployeeSQLite, error) {
	r := &EmployeeSQLite{db: db}
	stmts := []struct {
		dst   **sql.Stmt
		query string
	}{
		{&r.insert, `INSERT INTO employees (name, age, position, department_id, salary, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`},
		{&r.getByID, `SELECT ` + employeeColumns + ` FROM employees WHERE id = ? AND deleted_at IS NULL`},
		{&r.count, `SELECT COUNT(*) FROM employees
			WHERE deleted_at IS NULL AND (? IS NULL OR department_id = ?)`},
		{&r.list, `SELECT ` + employeeColumns + ` FROM employees
			WHERE deleted_at IS NULL AND (? IS NULL OR department_id = ?)
			ORDER BY id LIMIT ? OFFSET ?`},
		{&r.search, `SELECT ` + employeeColumns + ` FROM employees
			WHERE deleted_at IS NULL AND (name LIKE ? ESCAPE '\' OR position LIKE ? ESCAPE '\')
			ORDER BY id`},
		{&r.softDelete, `UPDATE employees SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`},
		{&r.all, `SELECT ` + employeeColumns + ` FROM employees WHERE deleted_at IS NULL ORDER BY id`},
		{&r.update, `UPDATE employees SET name = ?, age = ?, position = ?, department_id = ?, salary = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`},
	}
	for _, s := range stmts {
		st, err := db.PrepareContext(ctx, s.query)
		if err != nil {
			r.Close()
			return nil, fmt.Errorf("prepare employee statement: %w", err)
		}
		*s.dst = st
	}
	return r, nil
}

// Close releases the prepared statements.
func (r *EmployeeSQLite) Close() error {
	var errs []error
	for _, st := range []*sql.Stmt{r.insert, r.getByID, r.count, r.list, r.search, r.softDelete, r.all, r.update} {
		if st != nil {
			errs = append(errs, st.Close())
		}
	}
	return errors.Join(errs...)
}

// scanEmployee reads one row selected with employeeColumns. Scan errors are
// returned untranslated; callers pass them through translateError.
func scanEmployee(s interface{ Scan(...any) error }) (models.Employee, error) {
	var (
		e                models.Employee
		created, updated string
	)
	if err := s.Scan(&e.ID, &e.Name, &e.Age, &e.Position, &e.DepartmentID, &e.Salary, &created, &updated); err != nil {
		return models.Employee{}, err
	}
	var err error
	if e.CreatedAt, err = parseTime(created); err != nil {
		return models.Employee{}, err
	}
	if e.UpdatedAt, err = parseTime(updated); err != nil {
		return models.Employee{}, err
	}
	return e, nil
}

// collectEmployees drains rows into a non-nil slice and closes them.
func collectEmployees(rows *sql.Rows) ([]models.Employee, error) {
	defer rows.Close()
	out := []models.Employee{}
	for rows.Next() {
		e, err := scanEmployee(rows)
		if err != nil {
			return nil, translateError(err, errMsgs{})
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err, errMsgs{})
	}
	return out, nil
}

func (r *EmployeeSQLite) Create(ctx context.Context, e *models.Employee) error {
	now := time.Now().UTC()
	ts := formatTime(now)
	res, err := r.insert.ExecContext(ctx, e.Name, e.Age, e.Position, e.DepartmentID, e.Salary, ts, ts)
	if err != nil {
		return translateError(err, errMsgs{foreignKey: MsgDepartmentMissing(e.DepartmentID)})
	}
	id, err := res.LastInsertId()
	if err != nil {
		return utils.Internal(err)
	}
	e.ID, e.CreatedAt, e.UpdatedAt = id, now, now
	return nil
}

func (r *EmployeeSQLite) GetByID(ctx context.Context, id int64) (*models.Employee, error) {
	e, err := scanEmployee(r.getByID.QueryRowContext(ctx, id))
	if err != nil {
		return nil, translateError(err, errMsgs{notFound: MsgEmployeeNotFound})
	}
	return &e, nil
}

// List counts and fetches the page inside one transaction so totalCount
// matches the page contents.
func (r *EmployeeSQLite) List(ctx context.Context, f models.ListFilter) (models.EmployeePage, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.EmployeePage{}, translateError(err, errMsgs{})
	}
	defer tx.Rollback()

	var dept any // nil binds as SQL NULL: no department filter
	if f.DepartmentID != nil {
		dept = *f.DepartmentID
	}
	var page models.EmployeePage
	if err := tx.StmtContext(ctx, r.count).QueryRowContext(ctx, dept, dept).Scan(&page.TotalCount); err != nil {
		return models.EmployeePage{}, translateError(err, errMsgs{})
	}
	rows, err := tx.StmtContext(ctx, r.list).QueryContext(ctx, dept, dept, f.Limit, f.Offset)
	if err != nil {
		return models.EmployeePage{}, translateError(err, errMsgs{})
	}
	if page.Employees, err = collectEmployees(rows); err != nil {
		return models.EmployeePage{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.EmployeePage{}, translateError(err, errMsgs{})
	}
	return page, nil
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func (r *EmployeeSQLite) Search(ctx context.Context, keyword string) ([]models.Employee, error) {
	pattern := "%" + likeEscaper.Replace(keyword) + "%"
	rows, err := r.search.QueryContext(ctx, pattern, pattern)
	if err != nil {
		return nil, translateError(err, errMsgs{})
	}
	return collectEmployees(rows)
}

// Update applies the non-nil fields of in to the employee inside one
// transaction: read the live row, overwrite the fields, write the full row
// with a new updated_at, commit.
func (r *EmployeeSQLite) Update(ctx context.Context, id int64, in models.UpdateEmployeeInput) (*models.Employee, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, translateError(err, errMsgs{})
	}
	defer tx.Rollback()

	e, err := scanEmployee(tx.StmtContext(ctx, r.getByID).QueryRowContext(ctx, id))
	if err != nil {
		return nil, translateError(err, errMsgs{notFound: MsgEmployeeNotFound})
	}
	if in.Name != nil {
		e.Name = *in.Name
	}
	if in.Age != nil {
		e.Age = *in.Age
	}
	if in.Position != nil {
		e.Position = *in.Position
	}
	if in.DepartmentID != nil {
		e.DepartmentID = *in.DepartmentID
	}
	if in.Salary != nil {
		e.Salary = *in.Salary
	}
	e.UpdatedAt = time.Now().UTC()
	res, err := tx.StmtContext(ctx, r.update).ExecContext(ctx,
		e.Name, e.Age, e.Position, e.DepartmentID, e.Salary, formatTime(e.UpdatedAt), id)
	if err != nil {
		return nil, translateError(err, errMsgs{foreignKey: MsgDepartmentMissing(e.DepartmentID)})
	}
	// Defense in depth: the row was live at SELECT time and the tx holds the
	// write lock, so this only trips if the guard in the statement stops matching.
	if n, err := res.RowsAffected(); err != nil {
		return nil, utils.Internal(err)
	} else if n == 0 {
		return nil, utils.NotFound(MsgEmployeeNotFound)
	}
	if err := tx.Commit(); err != nil {
		return nil, translateError(err, errMsgs{})
	}
	return &e, nil
}

func (r *EmployeeSQLite) SoftDelete(ctx context.Context, id int64) error {
	ts := formatTime(time.Now())
	res, err := r.softDelete.ExecContext(ctx, ts, ts, id)
	if err != nil {
		return translateError(err, errMsgs{})
	}
	n, err := res.RowsAffected()
	if err != nil {
		return utils.Internal(err)
	}
	if n == 0 {
		return utils.NotFound(MsgEmployeeNotFound)
	}
	return nil
}

func (r *EmployeeSQLite) ListAll(ctx context.Context) ([]models.Employee, error) {
	rows, err := r.all.QueryContext(ctx)
	if err != nil {
		return nil, translateError(err, errMsgs{})
	}
	return collectEmployees(rows)
}
