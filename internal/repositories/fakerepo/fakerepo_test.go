package fakerepo_test

import (
	"context"
	"testing"

	"go-core/internal/models"
	"go-core/internal/repositories"
	"go-core/internal/repositories/fakerepo"
	"go-core/internal/testutil"
	"go-core/internal/utils"
)

var (
	_ repositories.EmployeeRepository   = (*fakerepo.Employees)(nil)
	_ repositories.DepartmentRepository = (*fakerepo.Departments)(nil)
)

func TestFakeMirrorsSQLiteSemantics(t *testing.T) {
	s := fakerepo.New()
	ctx := context.Background()
	d := &models.Department{Name: "Eng"}
	s.Departments().Create(ctx, d)
	testutil.AssertAppError(t, s.Departments().Create(ctx, &models.Department{Name: "eng"}), utils.KindConflict, `department "eng" already exists`)
	e := &models.Employee{Name: "An", Age: 1, Position: "Dev", DepartmentID: d.ID, Salary: 1}
	s.Employees().Create(ctx, e)
	testutil.AssertAppError(t, s.Employees().Create(ctx, &models.Employee{DepartmentID: 9}), utils.KindBusinessRule, "department 9 does not exist")
	s.Employees().SoftDelete(ctx, e.ID)
	got, _ := s.Departments().GetWithEmployees(ctx, d.ID)
	if got.Employees == nil || len(got.Employees) != 0 {
		t.Fatalf("%+v", got)
	}
	s.FailWith["Employee.List"] = utils.Timeout(nil)
	_, err := s.Employees().List(ctx, models.ListFilter{Limit: 10})
	testutil.AssertAppError(t, err, utils.KindTimeout, "request timed out")
}

// seed creates departments Eng (id 1) and Ops (id 2), then four employees:
// ids 1, 2, 4 in Eng and id 3 in Ops.
func seed(t *testing.T) (s *fakerepo.Store, eng, ops models.Department) {
	t.Helper()
	ctx := context.Background()
	s = fakerepo.New()
	eng, ops = models.Department{Name: "Eng"}, models.Department{Name: "Ops"}
	for _, d := range []*models.Department{&eng, &ops} {
		if err := s.Departments().Create(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	for _, e := range []models.Employee{
		{Name: "An Nguyen", Age: 30, Position: "Developer", DepartmentID: eng.ID, Salary: 100},
		{Name: "Binh", Age: 31, Position: "Tester", DepartmentID: eng.ID, Salary: 200},
		{Name: "Chi", Age: 32, Position: "developer", DepartmentID: ops.ID, Salary: 300},
		{Name: "Dung", Age: 33, Position: "Manager", DepartmentID: eng.ID, Salary: 400},
	} {
		if err := s.Employees().Create(ctx, &e); err != nil {
			t.Fatal(err)
		}
	}
	return s, eng, ops
}

func TestCreateSetsIDAndTimestamps(t *testing.T) {
	s, eng, _ := seed(t)
	ctx := context.Background()
	if eng.ID != 1 || eng.CreatedAt.IsZero() || eng.CreatedAt.Location().String() != "UTC" {
		t.Fatalf("department = %+v", eng)
	}
	e := &models.Employee{Name: "Eo", Age: 1, Position: "P", DepartmentID: eng.ID, Salary: 1}
	if err := s.Employees().Create(ctx, e); err != nil {
		t.Fatal(err)
	}
	if e.ID != 5 || e.CreatedAt.IsZero() || !e.CreatedAt.Equal(e.UpdatedAt) || e.CreatedAt.Location().String() != "UTC" {
		t.Fatalf("employee = %+v", e)
	}
	// A failed create must not consume an ID.
	_ = s.Employees().Create(ctx, &models.Employee{DepartmentID: 99})
	e2 := &models.Employee{Name: "Fe", Age: 1, Position: "P", DepartmentID: eng.ID, Salary: 1}
	_ = s.Employees().Create(ctx, e2)
	if e2.ID != 6 {
		t.Fatalf("ID = %d, want 6", e2.ID)
	}
}

func TestGetByIDReturnsCopy(t *testing.T) {
	s, _, _ := seed(t)
	ctx := context.Background()
	got, err := s.Employees().GetByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	got.Name = "mutated"
	again, _ := s.Employees().GetByID(ctx, 1)
	if again.Name != "An Nguyen" {
		t.Fatalf("store mutated through returned pointer: %q", again.Name)
	}
	_, err = s.Employees().GetByID(ctx, 99)
	testutil.AssertAppError(t, err, utils.KindNotFound, repositories.MsgEmployeeNotFound)
}

func TestListPagingAndFilter(t *testing.T) {
	s, eng, _ := seed(t)
	ctx := context.Background()
	page, err := s.Employees().List(ctx, models.ListFilter{Limit: 2, Offset: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalCount != 4 || len(page.Employees) != 2 || page.Employees[0].ID != 2 || page.Employees[1].ID != 3 {
		t.Fatalf("page = %+v", page)
	}
	page, _ = s.Employees().List(ctx, models.ListFilter{Limit: 10, DepartmentID: &eng.ID})
	if page.TotalCount != 3 || len(page.Employees) != 3 {
		t.Fatalf("filtered page = %+v", page)
	}
	// totalCount spans the filter, not the page; offset past the end gives [].
	page, _ = s.Employees().List(ctx, models.ListFilter{Limit: 10, Offset: 50, DepartmentID: &eng.ID})
	if page.TotalCount != 3 || page.Employees == nil || len(page.Employees) != 0 {
		t.Fatalf("past-end page = %+v", page)
	}
	s.Employees().SoftDelete(ctx, 1)
	page, _ = s.Employees().List(ctx, models.ListFilter{Limit: 10})
	if page.TotalCount != 3 || page.Employees[0].ID != 2 {
		t.Fatalf("after delete = %+v", page)
	}
}

func TestSearchCaseInsensitiveOnNameOrPosition(t *testing.T) {
	s, _, _ := seed(t)
	ctx := context.Background()
	got, err := s.Employees().Search(ctx, "DEVELOPER")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 3 {
		t.Fatalf("position match = %+v", got)
	}
	got, _ = s.Employees().Search(ctx, "nguy")
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("name match = %+v", got)
	}
	got, err = s.Employees().Search(ctx, "zzz")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("no match = %#v, %v", got, err)
	}
	// SQL LIKE wildcards are not special: the keyword is a literal substring.
	got, _ = s.Employees().Search(ctx, "%")
	if len(got) != 0 {
		t.Fatalf("percent matched %+v", got)
	}
}

func TestUpdatePartialAndErrors(t *testing.T) {
	s, _, ops := seed(t)
	ctx := context.Background()
	before, _ := s.Employees().GetByID(ctx, 1)
	name, dept := "Renamed", ops.ID
	got, err := s.Employees().Update(ctx, 1, models.UpdateEmployeeInput{Name: &name, DepartmentID: &dept})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Renamed" || got.DepartmentID != ops.ID || got.Age != before.Age || got.Salary != before.Salary || got.Position != before.Position {
		t.Fatalf("updated = %+v", got)
	}
	if got.UpdatedAt.Before(before.UpdatedAt) || !got.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("timestamps = %+v (before %+v)", got, before)
	}
	stored, _ := s.Employees().GetByID(ctx, 1)
	if stored.Name != "Renamed" {
		t.Fatalf("not persisted: %+v", stored)
	}

	bad := int64(99)
	_, err = s.Employees().Update(ctx, 1, models.UpdateEmployeeInput{DepartmentID: &bad})
	testutil.AssertAppError(t, err, utils.KindBusinessRule, "department 99 does not exist")
	stored, _ = s.Employees().GetByID(ctx, 1)
	if stored.DepartmentID != ops.ID {
		t.Fatalf("failed update changed the record: %+v", stored)
	}

	_, err = s.Employees().Update(ctx, 99, models.UpdateEmployeeInput{Name: &name})
	testutil.AssertAppError(t, err, utils.KindNotFound, repositories.MsgEmployeeNotFound)
	s.Employees().SoftDelete(ctx, 2)
	_, err = s.Employees().Update(ctx, 2, models.UpdateEmployeeInput{Name: &name})
	testutil.AssertAppError(t, err, utils.KindNotFound, repositories.MsgEmployeeNotFound)
}

func TestSoftDeleteHidesAndSecondDeleteIsNotFound(t *testing.T) {
	s, _, _ := seed(t)
	ctx := context.Background()
	if err := s.Employees().SoftDelete(ctx, 1); err != nil {
		t.Fatal(err)
	}
	testutil.AssertAppError(t, s.Employees().SoftDelete(ctx, 1), utils.KindNotFound, repositories.MsgEmployeeNotFound)
	testutil.AssertAppError(t, s.Employees().SoftDelete(ctx, 99), utils.KindNotFound, repositories.MsgEmployeeNotFound)
	_, err := s.Employees().GetByID(ctx, 1)
	testutil.AssertAppError(t, err, utils.KindNotFound, repositories.MsgEmployeeNotFound)
	all, _ := s.Employees().ListAll(ctx)
	if len(all) != 3 || all[0].ID != 2 {
		t.Fatalf("ListAll = %+v", all)
	}
	if found, _ := s.Employees().Search(ctx, "Nguyen"); len(found) != 0 {
		t.Fatalf("deleted employee found by search: %+v", found)
	}
}

func TestEmptyResultsAreNotNil(t *testing.T) {
	s := fakerepo.New()
	ctx := context.Background()
	all, _ := s.Employees().ListAll(ctx)
	page, _ := s.Employees().List(ctx, models.ListFilter{Limit: 10})
	found, _ := s.Employees().Search(ctx, "x")
	depts, _ := s.Departments().List(ctx)
	if all == nil || page.Employees == nil || found == nil || depts == nil {
		t.Fatalf("nil slice: %#v %#v %#v %#v", all, page.Employees, found, depts)
	}
}

func TestDepartments(t *testing.T) {
	s, eng, _ := seed(t)
	ctx := context.Background()
	list, _ := s.Departments().List(ctx)
	if len(list) != 2 || list[0].Name != "Eng" || list[1].Name != "Ops" {
		t.Fatalf("list = %+v", list)
	}
	for id, want := range map[int64]bool{eng.ID: true, 99: false} {
		if ok, err := s.Departments().Exists(ctx, id); err != nil || ok != want {
			t.Fatalf("Exists(%d) = %v, %v", id, ok, err)
		}
	}
	got, err := s.Departments().GetWithEmployees(ctx, eng.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Eng" || len(got.Employees) != 3 || got.Employees[0].ID != 1 || got.Employees[2].ID != 4 {
		t.Fatalf("GetWithEmployees = %+v", got)
	}
	_, err = s.Departments().GetWithEmployees(ctx, 99)
	testutil.AssertAppError(t, err, utils.KindNotFound, repositories.MsgDepartmentNotFound)
}

func TestCanceledContextIsTimeout(t *testing.T) {
	s, _, _ := seed(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.Employees().ListAll(ctx)
	ae := testutil.AssertAppError(t, err, utils.KindTimeout, "request timed out")
	if ae.Err != context.Canceled {
		t.Fatalf("underlying error = %v, want context.Canceled", ae.Err)
	}
	_, err = s.Departments().List(ctx)
	testutil.AssertAppError(t, err, utils.KindTimeout, "request timed out")
	// A canceled create must not write anything.
	testutil.AssertAppError(t, s.Departments().Create(ctx, &models.Department{Name: "X"}), utils.KindTimeout, "request timed out")
	if ok, _ := s.Departments().Exists(context.Background(), 3); ok {
		t.Fatal("canceled Create stored a department")
	}
}
