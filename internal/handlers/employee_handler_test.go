package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"go-core/internal/models"
	"go-core/internal/utils"
)

func TestEmployeeEndpoints(t *testing.T) {
	h, store := newTestRouter(t)
	d := &models.Department{Name: "Eng"}
	if err := store.Departments().Create(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"name":"Nguyen Van A","age":28,"position":"Backend Engineer","departmentId":%d,"salary":2000}`, d.ID)
	rec := do(t, h, "POST", "/employees", body)
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), `"departmentId":1`) {
		t.Fatal(rec.Code, rec.Body)
	}
	if rec = do(t, h, "POST", "/employees", `{"agee":1}`); rec.Code != 400 ||
		!strings.Contains(rec.Body.String(), `invalid request body: unknown field \"agee\"`) {
		t.Fatal(rec.Body)
	}
	if rec = do(t, h, "POST", "/employees", `{"name":"B","age":1,"position":"P","departmentId":99,"salary":1}`); rec.Code != 422 {
		t.Fatal(rec.Code)
	}
	rec = do(t, h, "GET", "/employees?limit=10&offset=0", "")
	if rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), `{"totalCount":1,"employees":[`) {
		t.Fatal(rec.Body)
	}
	if rec = do(t, h, "GET", "/employees?limit=101", ""); rec.Code != 400 {
		t.Fatal(rec.Code)
	}
	rec = do(t, h, "GET", "/employees/search?keyword=nguyen", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"totalCount":1`) {
		t.Fatal(rec.Body)
	}
	if rec = do(t, h, "GET", "/employees/search", ""); rec.Code != 400 { // not read as {id}="search"
		t.Fatal(rec.Code)
	}
	if rec = do(t, h, "GET", "/employees/1", ""); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	rec = do(t, h, "GET", "/employees/abc", "")
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), `invalid employee id: \"abc\"`) {
		t.Fatal(rec.Body)
	}
	rec = do(t, h, "PUT", "/employees/1", `{"salary":3000}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"salary":3000`) {
		t.Fatal(rec.Body)
	}
	rec = do(t, h, "DELETE", "/employees/1", "")
	if rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatal(rec.Code, rec.Body)
	}
	rec = do(t, h, "GET", "/employees/1", "")
	if rec.Code != 404 || strings.TrimSpace(rec.Body.String()) != `{"error":"Employee not found","code":404}` {
		t.Fatal(rec.Body)
	}
	if rec = do(t, h, "PATCH", "/employees/1", ""); rec.Code != 405 || rec.Header().Get("Allow") != "DELETE, GET, PUT" {
		t.Fatal(rec.Header())
	}
}

func TestEmployeeTimeoutFromRepo(t *testing.T) {
	h, store := newTestRouter(t)
	store.FailWith["Employee.List"] = utils.Timeout(nil)
	if rec := do(t, h, "GET", "/employees", ""); rec.Code != 504 {
		t.Fatal(rec.Code)
	}
}

func TestEmployeeListDepartmentFilter(t *testing.T) {
	h, store := newTestRouter(t)
	d := &models.Department{Name: "Eng"}
	if err := store.Departments().Create(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"name":"Nguyen Van A","age":28,"position":"Backend Engineer","departmentId":%d,"salary":2000}`, d.ID)
	if rec := do(t, h, "POST", "/employees", body); rec.Code != 201 {
		t.Fatal(rec.Code, rec.Body)
	}
	rec := do(t, h, "GET", fmt.Sprintf("/employees?departmentId=%d", d.ID), "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"totalCount":1`) {
		t.Fatal(rec.Code, rec.Body)
	}
	rec = do(t, h, "GET", "/employees?departmentId=x", "")
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), `invalid departmentId: \"x\"`) {
		t.Fatal(rec.Code, rec.Body)
	}
	rec = do(t, h, "GET", "/employees?departmentId=99", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"totalCount":0`) ||
		!strings.Contains(rec.Body.String(), `"employees":[]`) {
		t.Fatal(rec.Code, rec.Body)
	}
}

func TestEmployeeMissingID(t *testing.T) {
	h, _ := newTestRouter(t)
	if rec := do(t, h, "PUT", "/employees/99", `{"salary":3000}`); rec.Code != 404 {
		t.Fatal(rec.Code, rec.Body)
	}
	if rec := do(t, h, "DELETE", "/employees/99", ""); rec.Code != 404 {
		t.Fatal(rec.Code, rec.Body)
	}
}
