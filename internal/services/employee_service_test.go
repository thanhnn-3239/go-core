package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"go-core/internal/models"
	"go-core/internal/repositories/fakerepo"
	"go-core/internal/services"
	"go-core/internal/testutil"
	"go-core/internal/utils"
)

func newEmpSvc(t *testing.T) (*services.EmployeeService, int64) {
	t.Helper()
	s := fakerepo.New()
	d := &models.Department{Name: "Eng"}
	if err := s.Departments().Create(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	return services.NewEmployeeService(s.Employees(), s.Departments()), d.ID
}

func TestEmployeeServiceCreateValidation(t *testing.T) {
	svc, _ := newEmpSvc(t)
	_, err := svc.Create(context.Background(), models.CreateEmployeeInput{Name: " ", Age: 0, Position: "", DepartmentID: 0, Salary: -5})
	ae := testutil.AssertAppError(t, err, utils.KindValidation, "validation failed")
	fe := func(f, m string) utils.FieldError { return utils.FieldError{Field: f, Message: m} }
	want := []utils.FieldError{
		fe("name", "is required"), fe("age", "must be greater than 0"), fe("position", "is required"),
		fe("departmentId", "must be a positive integer"), fe("salary", "must be greater than 0"),
	}
	if !reflect.DeepEqual(ae.Details, want) {
		t.Fatalf("details %+v", ae.Details)
	}
}

func TestEmployeeServiceCreate(t *testing.T) {
	svc, d := newEmpSvc(t)
	ctx := context.Background()
	e, err := svc.Create(ctx, models.CreateEmployeeInput{Name: " An ", Age: 28, Position: " Dev ", DepartmentID: d, Salary: 2000})
	if err != nil || e.ID == 0 || e.Name != "An" || e.Position != "Dev" {
		t.Fatal(e, err)
	}
	_, err = svc.Create(ctx, models.CreateEmployeeInput{Name: "B", Age: 1, Position: "P", DepartmentID: 99, Salary: 1})
	testutil.AssertAppError(t, err, utils.KindBusinessRule, "department 99 does not exist")
}

func TestEmployeeServiceUpdate(t *testing.T) {
	svc, d := newEmpSvc(t)
	ctx := context.Background()
	e, _ := svc.Create(ctx, models.CreateEmployeeInput{Name: "An", Age: 28, Position: "Dev", DepartmentID: d, Salary: 2000})
	_, err := svc.Update(ctx, e.ID, models.UpdateEmployeeInput{})
	testutil.AssertAppError(t, err, utils.KindValidation, "no fields to update")
	age, salary := 0, int64(-1)
	_, err = svc.Update(ctx, e.ID, models.UpdateEmployeeInput{Age: &age, Salary: &salary})
	ae := testutil.AssertAppError(t, err, utils.KindValidation, "validation failed")
	if len(ae.Details) != 2 || ae.Details[0].Field != "age" || ae.Details[1].Field != "salary" {
		t.Fatal(ae.Details)
	}
	missing := int64(99)
	_, err = svc.Update(ctx, 12345, models.UpdateEmployeeInput{DepartmentID: &missing}) // 422 xảy ra trước 404
	testutil.AssertAppError(t, err, utils.KindBusinessRule, "department 99 does not exist")
	name := "  Binh "
	got, err := svc.Update(ctx, e.ID, models.UpdateEmployeeInput{Name: &name})
	if err != nil || got.Name != "Binh" || got.Salary != 2000 {
		t.Fatal(got, err)
	}
}

func TestEmployeeServiceUpdateNullFields(t *testing.T) { // Review Focus #1
	svc, d := newEmpSvc(t)
	e, _ := svc.Create(context.Background(), models.CreateEmployeeInput{Name: "An", Age: 1, Position: "P", DepartmentID: d, Salary: 1})
	var in models.UpdateEmployeeInput
	if err := json.Unmarshal([]byte(`{"name":null,"age":null}`), &in); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Update(context.Background(), e.ID, in)
	testutil.AssertAppError(t, err, utils.KindValidation, "no fields to update")
}

func TestEmployeeServiceSearch(t *testing.T) {
	svc, d := newEmpSvc(t)
	svc.Create(context.Background(), models.CreateEmployeeInput{Name: "An", Age: 1, Position: "Dev", DepartmentID: d, Salary: 1})
	_, err := svc.Search(context.Background(), "   ")
	testutil.AssertAppError(t, err, utils.KindValidation, "keyword is required")
	page, err := svc.Search(context.Background(), " an ")
	if err != nil || page.TotalCount != 1 || len(page.Employees) != 1 {
		t.Fatal(page, err)
	}
	page, err = svc.Search(context.Background(), "zzz")
	if err != nil || page.TotalCount != 0 || page.Employees == nil {
		t.Fatalf("no match page = %#v, %v", page, err)
	}
}

func TestEmployeeServiceUpdateValidationOrderAndTrim(t *testing.T) {
	svc, d := newEmpSvc(t)
	ctx := context.Background()
	e, _ := svc.Create(ctx, models.CreateEmployeeInput{Name: "An", Age: 28, Position: "Dev", DepartmentID: d, Salary: 2000})
	blank, zero, neg := "  ", int64(0), int64(-1)
	_, err := svc.Update(ctx, e.ID, models.UpdateEmployeeInput{Salary: &neg, DepartmentID: &zero, Position: &blank, Name: &blank})
	ae := testutil.AssertAppError(t, err, utils.KindValidation, "validation failed")
	var fields []string
	for _, fe := range ae.Details {
		fields = append(fields, fe.Field)
	}
	if want := []string{"name", "position", "departmentId", "salary"}; !reflect.DeepEqual(fields, want) {
		t.Fatalf("fields = %v, want %v", fields, want)
	}
	pos := " Lead "
	got, err := svc.Update(ctx, e.ID, models.UpdateEmployeeInput{Position: &pos})
	if err != nil || got.Position != "Lead" {
		t.Fatal(got, err)
	}
	if pos != " Lead " {
		t.Fatalf("caller's string was mutated: %q", pos)
	}
}

func TestEmployeeServicePassThrough(t *testing.T) {
	s := fakerepo.New()
	ctx := context.Background()
	dept := &models.Department{Name: "Eng"}
	s.Departments().Create(ctx, dept)
	svc := services.NewEmployeeService(s.Employees(), s.Departments())
	e, _ := svc.Create(ctx, models.CreateEmployeeInput{Name: "An", Age: 1, Position: "P", DepartmentID: dept.ID, Salary: 1})

	if got, err := svc.GetByID(ctx, e.ID); err != nil || got.ID != e.ID {
		t.Fatal(got, err)
	}
	if page, err := svc.List(ctx, models.ListFilter{Limit: 10}); err != nil || page.TotalCount != 1 {
		t.Fatal(page, err)
	}
	if err := svc.Delete(ctx, e.ID); err != nil {
		t.Fatal(err)
	}
	_, err := svc.GetByID(ctx, e.ID)
	testutil.AssertAppError(t, err, utils.KindNotFound, "Employee not found")
	testutil.AssertAppError(t, svc.Delete(ctx, e.ID), utils.KindNotFound, "Employee not found")
}

func TestEmployeeServiceRepositoryErrorsPassThrough(t *testing.T) {
	s := fakerepo.New()
	ctx := context.Background()
	dept := &models.Department{Name: "Eng"}
	s.Departments().Create(ctx, dept)
	svc := services.NewEmployeeService(s.Employees(), s.Departments())
	boom := errors.New("boom")
	s.FailWith["Employee.Create"] = boom
	_, err := svc.Create(ctx, models.CreateEmployeeInput{Name: "An", Age: 1, Position: "P", DepartmentID: dept.ID, Salary: 1})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}
