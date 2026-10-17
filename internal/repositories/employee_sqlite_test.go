package repositories

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-core/internal/models"
	"go-core/internal/testutil"
	"go-core/internal/utils"
)

// newEmpRepo creates a database with department "Eng" and returns an
// EmployeeSQLite on it together with that department's id.
func newEmpRepo(t *testing.T) (*EmployeeSQLite, int64) {
	t.Helper()
	db := newTestDB(t)
	emps, err := NewEmployeeSQLite(context.Background(), db)
	if err != nil {
		t.Fatalf("NewEmployeeSQLite: %v", err)
	}
	t.Cleanup(func() { emps.Close() })
	return emps, seedDept(t, emps, "Eng")
}

func seedDept(t *testing.T, emps *EmployeeSQLite, name string) int64 {
	t.Helper()
	depts, err := NewDepartmentSQLite(context.Background(), emps.db)
	if err != nil {
		t.Fatalf("NewDepartmentSQLite: %v", err)
	}
	defer depts.Close()
	d := &models.Department{Name: name}
	if err := depts.Create(context.Background(), d); err != nil {
		t.Fatalf("create department %q: %v", name, err)
	}
	return d.ID
}

func seedEmployee(t *testing.T, emps *EmployeeSQLite, deptID int64, name, position string) *models.Employee {
	t.Helper()
	e := &models.Employee{Name: name, Age: 30, Position: position, DepartmentID: deptID, Salary: 1000}
	if err := emps.Create(context.Background(), e); err != nil {
		t.Fatalf("create employee %q: %v", name, err)
	}
	return e
}

func TestEmployeeCreateGet(t *testing.T) {
	emps, deptID := newEmpRepo(t)
	ctx := context.Background()
	e := &models.Employee{Name: "Nguyễn Văn Ấn", Age: 28, Position: "Backend Engineer", DepartmentID: deptID, Salary: 2000}
	if err := emps.Create(ctx, e); err != nil || e.ID == 0 || e.CreatedAt.IsZero() || !e.CreatedAt.Equal(e.UpdatedAt) {
		t.Fatal(e, err)
	}
	got, err := emps.GetByID(ctx, e.ID)
	if err != nil || got.Name != "Nguyễn Văn Ấn" || got.Salary != 2000 {
		t.Fatal(got, err)
	}
	_, err = emps.GetByID(ctx, 999)
	testutil.AssertAppError(t, err, utils.KindNotFound, "Employee not found")
	err = emps.Create(ctx, &models.Employee{Name: "X", Age: 1, Position: "P", DepartmentID: 777, Salary: 1})
	testutil.AssertAppError(t, err, utils.KindBusinessRule, "department 777 does not exist")
}

func TestEmployeeListPagination(t *testing.T) {
	emps, d1 := newEmpRepo(t)
	d2 := seedDept(t, emps, "Sales")
	for i := 0; i < 5; i++ {
		seedEmployee(t, emps, d1, fmt.Sprintf("E%d", i), "Dev")
	}
	seedEmployee(t, emps, d2, "S0", "Sales")
	ctx := context.Background()
	page, err := emps.List(ctx, models.ListFilter{Limit: 2, Offset: 2})
	if err != nil || page.TotalCount != 6 || len(page.Employees) != 2 || page.Employees[0].Name != "E2" {
		t.Fatalf("%+v, %v", page, err)
	}
	page, err = emps.List(ctx, models.ListFilter{Limit: 10, DepartmentID: &d2})
	if err != nil || page.TotalCount != 1 || page.Employees[0].Name != "S0" {
		t.Fatalf("filter: %+v, %v", page, err)
	}
	page, err = emps.List(ctx, models.ListFilter{Limit: 10, Offset: 1000}) // Review Focus #5
	if err != nil || page.TotalCount != 6 || page.Employees == nil || len(page.Employees) != 0 {
		t.Fatalf("beyond: %+v, %v", page, err)
	}
	missing := int64(999)
	page, err = emps.List(ctx, models.ListFilter{Limit: 10, DepartmentID: &missing})
	if err != nil || page.TotalCount != 0 || page.Employees == nil {
		t.Fatalf("missing dept: %+v, %v", page, err)
	}
}

func TestEmployeeSearch(t *testing.T) {
	emps, d := newEmpRepo(t)
	seedEmployee(t, emps, d, "An Nguyen", "Dev")
	seedEmployee(t, emps, d, "Binh", "Android Engineer")
	seedEmployee(t, emps, d, "100% Pure", "QA")
	seedEmployee(t, emps, d, "Chi", "Tester")
	ctx := context.Background()
	got, err := emps.Search(ctx, "AN")
	if err != nil || len(got) != 2 { // khớp name "An Nguyen" và position "Android"
		t.Fatalf("an: %+v, %v", got, err)
	}
	got, err = emps.Search(ctx, "%")
	if err != nil || len(got) != 1 || got[0].Name != "100% Pure" {
		t.Fatalf("percent: %+v, %v", got, err)
	}
	got, err = emps.Search(ctx, "_")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("underscore: %+v, %v", got, err)
	}
}

func TestEmployeeSoftDelete(t *testing.T) {
	emps, d := newEmpRepo(t)
	e := seedEmployee(t, emps, d, "Gone", "Dev")
	keep := seedEmployee(t, emps, d, "Keep", "Dev")
	ctx := context.Background()
	if err := emps.SoftDelete(ctx, e.ID); err != nil {
		t.Fatal(err)
	}
	testutil.AssertAppError(t, emps.SoftDelete(ctx, e.ID), utils.KindNotFound, "Employee not found")
	_, err := emps.GetByID(ctx, e.ID)
	testutil.AssertAppError(t, err, utils.KindNotFound, "Employee not found")
	page, err := emps.List(ctx, models.ListFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	all, err := emps.ListAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found, err := emps.Search(ctx, "Gone")
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalCount != 1 || len(all) != 1 || all[0].ID != keep.ID || len(found) != 0 {
		t.Fatal(page, all, found)
	}
}

func TestEmployeeExpiredContext(t *testing.T) {
	emps, _ := newEmpRepo(t)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, err := emps.GetByID(ctx, 1)
	testutil.AssertAppError(t, err, utils.KindTimeout, "request timed out")
}
