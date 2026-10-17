package repositories

import "fmt"

const (
	MsgEmployeeNotFound   = "Employee not found"
	MsgDepartmentNotFound = "Department not found"
)

func MsgDepartmentExists(name string) string {
	return fmt.Sprintf("department %q already exists", name)
}

func MsgDepartmentMissing(id int64) string {
	return fmt.Sprintf("department %d does not exist", id)
}
