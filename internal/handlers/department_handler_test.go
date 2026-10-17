package handlers

import (
	"strings"
	"testing"
)

func TestDepartmentEndpoints(t *testing.T) {
	h, _ := newTestRouter(t)
	rec := do(t, h, "POST", "/departments", `{"name":"Engineering"}`)
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), `"name":"Engineering"`) {
		t.Fatal(rec.Code, rec.Body)
	}
	if rec = do(t, h, "POST", "/departments", `{"name":"engineering"}`); rec.Code != 409 {
		t.Fatal(rec.Code)
	}
	if rec = do(t, h, "POST", "/departments", `{"name":""}`); rec.Code != 400 || !strings.Contains(rec.Body.String(), `"details"`) {
		t.Fatal(rec.Body)
	}
	rec = do(t, h, "GET", "/departments", "")
	if rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), `{"departments":[`) {
		t.Fatal(rec.Body)
	}
	rec = do(t, h, "GET", "/departments/1/employees", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"employees":[]`) {
		t.Fatal(rec.Body)
	}
	if rec = do(t, h, "GET", "/departments/99/employees", ""); rec.Code != 404 {
		t.Fatal(rec.Code)
	}
	rec = do(t, h, "GET", "/departments/abc/employees", "")
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), `invalid department id: \"abc\"`) {
		t.Fatal(rec.Body)
	}
}

func TestDepartmentListEmptyIsArray(t *testing.T) {
	h, _ := newTestRouter(t)
	rec := do(t, h, "GET", "/departments", "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"departments":[]}` {
		t.Fatal(rec.Code, rec.Body)
	}
}
