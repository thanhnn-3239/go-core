package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go-core/internal/models"
	"go-core/internal/utils"
)

// DepartmentSQLite implements DepartmentRepository on SQLite with
// statements prepared once at construction.
type DepartmentSQLite struct {
	insert, list, exists, withEmployees *sql.Stmt
}

func NewDepartmentSQLite(ctx context.Context, db *sql.DB) (*DepartmentSQLite, error) {
	r := &DepartmentSQLite{}
	stmts := []struct {
		dst   **sql.Stmt
		query string
	}{
		{&r.insert, `INSERT INTO departments (name, created_at) VALUES (?, ?)`},
		{&r.list, `SELECT id, name, created_at FROM departments ORDER BY id`},
		{&r.exists, `SELECT EXISTS(SELECT 1 FROM departments WHERE id = ?)`},
		{&r.withEmployees, `SELECT d.id, d.name, d.created_at,
			e.id, e.name, e.age, e.position, e.department_id, e.salary, e.created_at, e.updated_at
			FROM departments d
			LEFT JOIN employees e ON e.department_id = d.id AND e.deleted_at IS NULL
			WHERE d.id = ?
			ORDER BY e.id`},
	}
	for _, s := range stmts {
		st, err := db.PrepareContext(ctx, s.query)
		if err != nil {
			r.Close()
			return nil, fmt.Errorf("prepare department statement: %w", err)
		}
		*s.dst = st
	}
	return r, nil
}

// Close releases the prepared statements.
func (r *DepartmentSQLite) Close() error {
	var errs []error
	for _, st := range []*sql.Stmt{r.insert, r.list, r.exists, r.withEmployees} {
		if st != nil {
			errs = append(errs, st.Close())
		}
	}
	return errors.Join(errs...)
}

func (r *DepartmentSQLite) Create(ctx context.Context, d *models.Department) error {
	now := time.Now().UTC()
	res, err := r.insert.ExecContext(ctx, d.Name, formatTime(now))
	if err != nil {
		return translateError(err, errMsgs{conflict: MsgDepartmentExists(d.Name)})
	}
	id, err := res.LastInsertId()
	if err != nil {
		return utils.Internal(err)
	}
	d.ID, d.CreatedAt = id, now
	return nil
}

func (r *DepartmentSQLite) List(ctx context.Context) ([]models.Department, error) {
	rows, err := r.list.QueryContext(ctx)
	if err != nil {
		return nil, translateError(err, errMsgs{})
	}
	defer rows.Close()
	out := []models.Department{}
	for rows.Next() {
		var d models.Department
		var created string
		if err := rows.Scan(&d.ID, &d.Name, &created); err != nil {
			return nil, translateError(err, errMsgs{})
		}
		if d.CreatedAt, err = parseTime(created); err != nil {
			return nil, utils.Internal(err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err, errMsgs{})
	}
	return out, nil
}

func (r *DepartmentSQLite) Exists(ctx context.Context, id int64) (bool, error) {
	var ok bool
	if err := r.exists.QueryRowContext(ctx, id).Scan(&ok); err != nil {
		return false, translateError(err, errMsgs{})
	}
	return ok, nil
}

func (r *DepartmentSQLite) GetWithEmployees(ctx context.Context, id int64) (*models.DepartmentEmployees, error) {
	rows, err := r.withEmployees.QueryContext(ctx, id)
	if err != nil {
		return nil, translateError(err, errMsgs{})
	}
	defer rows.Close()

	var out *models.DepartmentEmployees
	for rows.Next() {
		var (
			dept                                 models.Department
			dCreated                             string
			eID, eAge, eDept, eSalary            sql.NullInt64
			eName, ePosition, eCreated, eUpdated sql.NullString
		)
		if err := rows.Scan(&dept.ID, &dept.Name, &dCreated,
			&eID, &eName, &eAge, &ePosition, &eDept, &eSalary, &eCreated, &eUpdated); err != nil {
			return nil, translateError(err, errMsgs{})
		}
		if out == nil {
			if dept.CreatedAt, err = parseTime(dCreated); err != nil {
				return nil, utils.Internal(err)
			}
			out = &models.DepartmentEmployees{Department: dept, Employees: []models.Employee{}}
		}
		if !eID.Valid {
			continue // department exists but has no non-deleted employees
		}
		emp := models.Employee{
			ID:           eID.Int64,
			Name:         eName.String,
			Age:          int(eAge.Int64),
			Position:     ePosition.String,
			DepartmentID: eDept.Int64,
			Salary:       eSalary.Int64,
		}
		if emp.CreatedAt, err = parseTime(eCreated.String); err != nil {
			return nil, utils.Internal(err)
		}
		if emp.UpdatedAt, err = parseTime(eUpdated.String); err != nil {
			return nil, utils.Internal(err)
		}
		out.Employees = append(out.Employees, emp)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err, errMsgs{})
	}
	if out == nil {
		return nil, utils.NotFound(MsgDepartmentNotFound)
	}
	return out, nil
}
