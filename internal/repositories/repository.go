package repositories

import (
	"context"

	"go-core/internal/models"
)

type EmployeeRepository interface {
	Create(ctx context.Context, e *models.Employee) error // sets ID, CreatedAt, UpdatedAt on e
	GetByID(ctx context.Context, id int64) (*models.Employee, error)
	List(ctx context.Context, f models.ListFilter) (models.EmployeePage, error)
	Search(ctx context.Context, keyword string) ([]models.Employee, error)
	Update(ctx context.Context, id int64, in models.UpdateEmployeeInput) (*models.Employee, error)
	SoftDelete(ctx context.Context, id int64) error
	ListAll(ctx context.Context) ([]models.Employee, error)
}

type DepartmentRepository interface {
	Create(ctx context.Context, d *models.Department) error
	List(ctx context.Context) ([]models.Department, error)
	Exists(ctx context.Context, id int64) (bool, error)
	GetWithEmployees(ctx context.Context, id int64) (*models.DepartmentEmployees, error)
}
