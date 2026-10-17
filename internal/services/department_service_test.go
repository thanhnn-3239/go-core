package services_test

import (
	"context"
	"errors"
	"testing"

	"go-core/internal/models"
	"go-core/internal/repositories/fakerepo"
	"go-core/internal/services"
	"go-core/internal/testutil"
	"go-core/internal/utils"
)

func TestDepartmentServiceCreate(t *testing.T) {
	svc := services.NewDepartmentService(fakerepo.New().Departments())
	ctx := context.Background()
	d, err := svc.Create(ctx, models.CreateDepartmentInput{Name: "  Engineering  "})
	if err != nil || d.Name != "Engineering" {
		t.Fatal(d, err)
	}
	_, err = svc.Create(ctx, models.CreateDepartmentInput{Name: "   "})
	ae := testutil.AssertAppError(t, err, utils.KindValidation, "validation failed")
	if len(ae.Details) != 1 || ae.Details[0] != (utils.FieldError{Field: "name", Message: "is required"}) {
		t.Fatal(ae.Details)
	}
	_, err = svc.Create(ctx, models.CreateDepartmentInput{Name: "engineering"})
	testutil.AssertAppError(t, err, utils.KindConflict, `department "engineering" already exists`)
}

func TestDepartmentServicePassesRepoErrors(t *testing.T) {
	s := fakerepo.New()
	s.FailWith["Department.List"] = utils.Internal(errors.New("db down"))
	_, err := services.NewDepartmentService(s.Departments()).List(context.Background())
	testutil.AssertAppError(t, err, utils.KindInternal, "internal server error")
}
