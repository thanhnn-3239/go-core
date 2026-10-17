// Package fakerepo provides in-memory repositories for service and handler
// tests. They mirror the SQLite repositories' behavior and error messages.
package fakerepo

import (
	"context"
	"strings"
	"sync"
	"time"

	"go-core/internal/models"
	"go-core/internal/repositories"
	"go-core/internal/utils"
)

// Store holds the data behind both fake repositories. One mutex guards all
// of it, including FailWith: set FailWith keys before sharing the Store with
// other goroutines.
type Store struct {
	// FailWith makes a method return the mapped error. Keys are
	// "Employee.<Method>" or "Department.<Method>", for example
	// FailWith["Employee.List"] = utils.Timeout(nil).
	FailWith map[string]error

	mu          sync.Mutex
	departments []models.Department
	employees   []employeeRecord
	nextDeptID  int64
	nextEmpID   int64
}

type employeeRecord struct {
	models.Employee
	deleted bool
}

// New returns an empty Store with FailWith ready to assign to.
func New() *Store {
	return &Store{FailWith: map[string]error{}, nextDeptID: 1, nextEmpID: 1}
}

type Employees struct{ s *Store }

type Departments struct{ s *Store }

var (
	_ repositories.EmployeeRepository   = (*Employees)(nil)
	_ repositories.DepartmentRepository = (*Departments)(nil)
)

func (s *Store) Employees() *Employees { return &Employees{s: s} }

func (s *Store) Departments() *Departments { return &Departments{s: s} }

// begin locks the store and runs the checks every method starts with. A done
// context wins over FailWith, like the real repos where the database call
// fails on the context before anything else. On a non-nil error the store is
// already unlocked; otherwise the caller must unlock.
func (s *Store) begin(ctx context.Context, key string) error {
	s.mu.Lock()
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return utils.Timeout(err)
	}
	if err := s.FailWith[key]; err != nil {
		s.mu.Unlock()
		return err
	}
	return nil
}

// asciiLower folds only ASCII letters, like SQLite's LIKE and NOCASE.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

func (s *Store) departmentExists(id int64) bool {
	for _, d := range s.departments {
		if d.ID == id {
			return true
		}
	}
	return false
}

// liveEmployee returns the index of the non-deleted employee with this id, or -1.
func (s *Store) liveEmployee(id int64) int {
	for i, r := range s.employees {
		if r.ID == id && !r.deleted {
			return i
		}
	}
	return -1
}

func (r *Employees) Create(ctx context.Context, e *models.Employee) error {
	s := r.s
	if err := s.begin(ctx, "Employee.Create"); err != nil {
		return err
	}
	defer s.mu.Unlock()
	if !s.departmentExists(e.DepartmentID) {
		return utils.BusinessRule(repositories.MsgDepartmentMissing(e.DepartmentID), nil)
	}
	now := time.Now().UTC()
	e.ID, e.CreatedAt, e.UpdatedAt = s.nextEmpID, now, now
	s.nextEmpID++
	s.employees = append(s.employees, employeeRecord{Employee: *e})
	return nil
}

func (r *Employees) GetByID(ctx context.Context, id int64) (*models.Employee, error) {
	s := r.s
	if err := s.begin(ctx, "Employee.GetByID"); err != nil {
		return nil, err
	}
	defer s.mu.Unlock()
	i := s.liveEmployee(id)
	if i < 0 {
		return nil, utils.NotFound(repositories.MsgEmployeeNotFound)
	}
	e := s.employees[i].Employee
	return &e, nil
}

// List mirrors SQLite LIMIT/OFFSET: a negative limit means no limit and a
// negative offset counts as 0. TotalCount covers the whole filter.
func (r *Employees) List(ctx context.Context, f models.ListFilter) (models.EmployeePage, error) {
	s := r.s
	if err := s.begin(ctx, "Employee.List"); err != nil {
		return models.EmployeePage{}, err
	}
	defer s.mu.Unlock()
	var matched []models.Employee
	for _, rec := range s.employees {
		if rec.deleted || (f.DepartmentID != nil && rec.DepartmentID != *f.DepartmentID) {
			continue
		}
		matched = append(matched, rec.Employee)
	}
	page := models.EmployeePage{TotalCount: len(matched), Employees: []models.Employee{}}
	start := max(f.Offset, 0)
	if start >= len(matched) || f.Limit == 0 {
		return page, nil
	}
	end := len(matched)
	if f.Limit > 0 {
		end = min(start+f.Limit, end)
	}
	page.Employees = append(page.Employees, matched[start:end]...)
	return page, nil
}

// Search matches keyword as an ASCII case-insensitive substring of name or
// position, ordered by id.
func (r *Employees) Search(ctx context.Context, keyword string) ([]models.Employee, error) {
	s := r.s
	if err := s.begin(ctx, "Employee.Search"); err != nil {
		return nil, err
	}
	defer s.mu.Unlock()
	kw := asciiLower(keyword)
	out := []models.Employee{}
	for _, rec := range s.employees {
		if rec.deleted {
			continue
		}
		if strings.Contains(asciiLower(rec.Name), kw) || strings.Contains(asciiLower(rec.Position), kw) {
			out = append(out, rec.Employee)
		}
	}
	return out, nil
}

func (r *Employees) Update(ctx context.Context, id int64, in models.UpdateEmployeeInput) (*models.Employee, error) {
	s := r.s
	if err := s.begin(ctx, "Employee.Update"); err != nil {
		return nil, err
	}
	defer s.mu.Unlock()
	i := s.liveEmployee(id)
	if i < 0 {
		return nil, utils.NotFound(repositories.MsgEmployeeNotFound)
	}
	e := s.employees[i].Employee // work on a copy so a failed update changes nothing
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
	if !s.departmentExists(e.DepartmentID) {
		return nil, utils.BusinessRule(repositories.MsgDepartmentMissing(e.DepartmentID), nil)
	}
	e.UpdatedAt = time.Now().UTC()
	s.employees[i].Employee = e
	return &e, nil
}

func (r *Employees) SoftDelete(ctx context.Context, id int64) error {
	s := r.s
	if err := s.begin(ctx, "Employee.SoftDelete"); err != nil {
		return err
	}
	defer s.mu.Unlock()
	i := s.liveEmployee(id)
	if i < 0 {
		return utils.NotFound(repositories.MsgEmployeeNotFound)
	}
	s.employees[i].deleted = true
	return nil
}

func (r *Employees) ListAll(ctx context.Context) ([]models.Employee, error) {
	s := r.s
	if err := s.begin(ctx, "Employee.ListAll"); err != nil {
		return nil, err
	}
	defer s.mu.Unlock()
	out := []models.Employee{}
	for _, rec := range s.employees {
		if !rec.deleted {
			out = append(out, rec.Employee)
		}
	}
	return out, nil
}

// Create rejects a name that equals an existing one ignoring ASCII case.
func (r *Departments) Create(ctx context.Context, d *models.Department) error {
	s := r.s
	if err := s.begin(ctx, "Department.Create"); err != nil {
		return err
	}
	defer s.mu.Unlock()
	name := asciiLower(d.Name)
	for _, existing := range s.departments {
		if asciiLower(existing.Name) == name {
			return utils.Conflict(repositories.MsgDepartmentExists(d.Name), nil)
		}
	}
	d.ID, d.CreatedAt = s.nextDeptID, time.Now().UTC()
	s.nextDeptID++
	s.departments = append(s.departments, *d)
	return nil
}

func (r *Departments) List(ctx context.Context) ([]models.Department, error) {
	s := r.s
	if err := s.begin(ctx, "Department.List"); err != nil {
		return nil, err
	}
	defer s.mu.Unlock()
	return append([]models.Department{}, s.departments...), nil
}

func (r *Departments) Exists(ctx context.Context, id int64) (bool, error) {
	s := r.s
	if err := s.begin(ctx, "Department.Exists"); err != nil {
		return false, err
	}
	defer s.mu.Unlock()
	return s.departmentExists(id), nil
}

func (r *Departments) GetWithEmployees(ctx context.Context, id int64) (*models.DepartmentEmployees, error) {
	s := r.s
	if err := s.begin(ctx, "Department.GetWithEmployees"); err != nil {
		return nil, err
	}
	defer s.mu.Unlock()
	for _, d := range s.departments {
		if d.ID != id {
			continue
		}
		out := &models.DepartmentEmployees{Department: d, Employees: []models.Employee{}}
		for _, rec := range s.employees {
			if !rec.deleted && rec.DepartmentID == id {
				out.Employees = append(out.Employees, rec.Employee)
			}
		}
		return out, nil
	}
	return nil, utils.NotFound(repositories.MsgDepartmentNotFound)
}
