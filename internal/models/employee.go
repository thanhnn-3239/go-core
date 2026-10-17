package models

import "time"

type Employee struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Age          int       `json:"age"`
	Position     string    `json:"position"`
	DepartmentID int64     `json:"departmentId"`
	Salary       int64     `json:"salary"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type CreateEmployeeInput struct {
	Name         string `json:"name"`
	Age          int    `json:"age"`
	Position     string `json:"position"`
	DepartmentID int64  `json:"departmentId"`
	Salary       int64  `json:"salary"`
}

// UpdateEmployeeInput: a nil field means keep the current value.
type UpdateEmployeeInput struct {
	Name         *string `json:"name"`
	Age          *int    `json:"age"`
	Position     *string `json:"position"`
	DepartmentID *int64  `json:"departmentId"`
	Salary       *int64  `json:"salary"`
}

type ListFilter struct {
	Limit, Offset int
	DepartmentID  *int64
}

type EmployeePage struct {
	TotalCount int        `json:"totalCount"`
	Employees  []Employee `json:"employees"` // always [], never null
}
