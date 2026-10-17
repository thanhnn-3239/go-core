package services

import (
	"context"
	"strings"

	"go-core/internal/models"
	"go-core/internal/repositories"
	"go-core/internal/utils"
)

// EmployeeService implements employee use cases: field validation, the
// department-must-exist business rule, and pass-through to the repositories.
// Repository errors are returned unchanged.
type EmployeeService struct {
	emps  repositories.EmployeeRepository
	depts repositories.DepartmentRepository
}

func NewEmployeeService(emps repositories.EmployeeRepository, depts repositories.DepartmentRepository) *EmployeeService {
	return &EmployeeService{emps: emps, depts: depts}
}

// validateFields checks every non-nil field and returns the failures in the
// fixed order name, age, position, departmentId, salary. String fields are
// expected to be trimmed already.
func validateFields(name *string, age *int, position *string, deptID, salary *int64) []utils.FieldError {
	var errs []utils.FieldError
	add := func(field, msg string) { errs = append(errs, utils.FieldError{Field: field, Message: msg}) }
	if name != nil && *name == "" {
		add("name", "is required")
	}
	if age != nil && *age <= 0 {
		add("age", "must be greater than 0")
	}
	if position != nil && *position == "" {
		add("position", "is required")
	}
	if deptID != nil && *deptID <= 0 {
		add("departmentId", "must be a positive integer")
	}
	if salary != nil && *salary <= 0 {
		add("salary", "must be greater than 0")
	}
	return errs
}

// requireDepartment returns a business-rule error when the department does
// not exist; an Exists failure is returned unchanged.
func (s *EmployeeService) requireDepartment(ctx context.Context, id int64) error {
	ok, err := s.depts.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return utils.BusinessRule(repositories.MsgDepartmentMissing(id), nil)
	}
	return nil
}

// Create trims name and position, validates all fields, checks the department
// exists, and stores the employee.
func (s *EmployeeService) Create(ctx context.Context, in models.CreateEmployeeInput) (*models.Employee, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Position = strings.TrimSpace(in.Position)
	if errs := validateFields(&in.Name, &in.Age, &in.Position, &in.DepartmentID, &in.Salary); len(errs) > 0 {
		return nil, utils.Validation("validation failed", errs...)
	}
	if err := s.requireDepartment(ctx, in.DepartmentID); err != nil {
		return nil, err
	}
	e := &models.Employee{
		Name:         in.Name,
		Age:          in.Age,
		Position:     in.Position,
		DepartmentID: in.DepartmentID,
		Salary:       in.Salary,
	}
	if err := s.emps.Create(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (s *EmployeeService) GetByID(ctx context.Context, id int64) (*models.Employee, error) {
	return s.emps.GetByID(ctx, id)
}

func (s *EmployeeService) List(ctx context.Context, f models.ListFilter) (models.EmployeePage, error) {
	return s.emps.List(ctx, f)
}

// Search finds employees by a trimmed, non-empty keyword.
func (s *EmployeeService) Search(ctx context.Context, keyword string) (models.EmployeePage, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return models.EmployeePage{}, utils.Validation("keyword is required")
	}
	list, err := s.emps.Search(ctx, keyword)
	if err != nil {
		return models.EmployeePage{}, err
	}
	if list == nil {
		list = []models.Employee{}
	}
	return models.EmployeePage{TotalCount: len(list), Employees: list}, nil
}

// Update applies a partial update. Order: at least one field, then field
// validation (400), then the department rule (422), then the repository (404).
func (s *EmployeeService) Update(ctx context.Context, id int64, in models.UpdateEmployeeInput) (*models.Employee, error) {
	if in.Name == nil && in.Age == nil && in.Position == nil && in.DepartmentID == nil && in.Salary == nil {
		return nil, utils.Validation("no fields to update")
	}
	// Trim into fresh values so the caller's pointers are not mutated.
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		in.Name = &n
	}
	if in.Position != nil {
		p := strings.TrimSpace(*in.Position)
		in.Position = &p
	}
	if errs := validateFields(in.Name, in.Age, in.Position, in.DepartmentID, in.Salary); len(errs) > 0 {
		return nil, utils.Validation("validation failed", errs...)
	}
	if in.DepartmentID != nil {
		if err := s.requireDepartment(ctx, *in.DepartmentID); err != nil {
			return nil, err
		}
	}
	return s.emps.Update(ctx, id, in)
}

func (s *EmployeeService) Delete(ctx context.Context, id int64) error {
	return s.emps.SoftDelete(ctx, id)
}
