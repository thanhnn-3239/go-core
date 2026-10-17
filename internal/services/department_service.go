// Package services holds the business logic that sits between the HTTP
// handlers and the repositories.
package services

import (
	"context"
	"strings"

	"go-core/internal/models"
	"go-core/internal/repositories"
	"go-core/internal/utils"
)

// DepartmentService implements department use cases on top of a
// DepartmentRepository.
type DepartmentService struct {
	repo repositories.DepartmentRepository
}

func NewDepartmentService(repo repositories.DepartmentRepository) *DepartmentService {
	return &DepartmentService{repo: repo}
}

// Create trims the name, rejects an empty one, and stores the department.
// Repository errors are returned unchanged.
func (s *DepartmentService) Create(ctx context.Context, in models.CreateDepartmentInput) (*models.Department, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, utils.Validation("validation failed", utils.FieldError{Field: "name", Message: "is required"})
	}
	d := &models.Department{Name: name}
	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *DepartmentService) List(ctx context.Context) ([]models.Department, error) {
	return s.repo.List(ctx)
}

func (s *DepartmentService) GetWithEmployees(ctx context.Context, id int64) (*models.DepartmentEmployees, error) {
	return s.repo.GetWithEmployees(ctx, id)
}
