package repositories

import (
	"context"
	"errors"
	"sync"
	"testing"

	"go-core/internal/models"
	"go-core/internal/testutil"
	"go-core/internal/utils"
)

func TestDepartmentCreateListExists(t *testing.T) {
	repo := newDeptRepo(t)
	ctx := context.Background()
	d := &models.Department{Name: "Engineering"}
	if err := repo.Create(ctx, d); err != nil || d.ID == 0 || d.CreatedAt.IsZero() {
		t.Fatalf("%+v %v", d, err)
	}
	err := repo.Create(ctx, &models.Department{Name: "engineering"})
	testutil.AssertAppError(t, err, utils.KindConflict, `department "engineering" already exists`)
	if err := repo.Create(ctx, &models.Department{Name: "Sales"}); err != nil {
		t.Fatal(err)
	}
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "Engineering" || list[1].Name != "Sales" {
		t.Fatalf("%+v", list)
	}
	if ok, _ := repo.Exists(ctx, d.ID); !ok {
		t.Fatal("exists")
	}
	if ok, _ := repo.Exists(ctx, 999); ok {
		t.Fatal("not exists")
	}
}

func TestDepartmentListEmptyIsNotNil(t *testing.T) {
	list, err := newDeptRepo(t).List(context.Background())
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("list=%#v err=%v", list, err)
	}
}

func TestDepartmentConcurrentDuplicate(t *testing.T) {
	repo := newDeptRepo(t)
	var wg sync.WaitGroup
	var mu sync.Mutex
	okCount, conflicts := 0, 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := repo.Create(context.Background(), &models.Department{Name: "Ops"})
			var ae *utils.AppError
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				okCount++
			case errors.As(err, &ae) && ae.Kind == utils.KindConflict:
				conflicts++
			default:
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	wg.Wait()
	if okCount != 1 || conflicts != 9 {
		t.Fatalf("ok=%d conflicts=%d", okCount, conflicts)
	}
}

func TestDepartmentGetWithEmployees(t *testing.T) {
	repo, db := newDeptRepoWithDB(t)
	ctx := context.Background()
	empty := &models.Department{Name: "Empty"}
	if err := repo.Create(ctx, empty); err != nil {
		t.Fatal(err)
	}
	full := &models.Department{Name: "Full"}
	if err := repo.Create(ctx, full); err != nil {
		t.Fatal(err)
	}
	a := insertEmployeeRow(t, db, full.ID, "An", false)
	insertEmployeeRow(t, db, full.ID, "Gone", true)
	got, err := repo.GetWithEmployees(ctx, empty.ID)
	if err != nil || got.Employees == nil || len(got.Employees) != 0 {
		t.Fatalf("empty: %+v %v", got, err)
	}
	got, err = repo.GetWithEmployees(ctx, full.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Employees) != 1 || got.Employees[0].ID != a || got.Name != "Full" {
		t.Fatalf("full: %+v", got)
	}
	e := got.Employees[0]
	if e.Name != "An" || e.Age != 30 || e.Position != "Developer" || e.DepartmentID != full.ID || e.Salary != 1000 || e.CreatedAt.IsZero() || e.UpdatedAt.IsZero() {
		t.Fatalf("employee fields not scanned: %+v", e)
	}
	_, err = repo.GetWithEmployees(ctx, 999)
	testutil.AssertAppError(t, err, utils.KindNotFound, "Department not found")
}
