package handlers

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-core/internal/middleware"
	"go-core/internal/repositories"
	"go-core/internal/services"
)

// newIntegrationServer wires the real stack the same way cmd/app run does:
// SQLite -> repositories -> services -> handlers -> router -> middleware.
func newIntegrationServer(t *testing.T) *httptest.Server {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	db, err := repositories.OpenSQLite(ctx, filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	empRepo, err := repositories.NewEmployeeSQLite(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { empRepo.Close() })
	deptRepo, err := repositories.NewDepartmentSQLite(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { deptRepo.Close() })

	router := NewRouter(
		NewEmployeeHandler(services.NewEmployeeService(empRepo, deptRepo)),
		NewDepartmentHandler(services.NewDepartmentService(deptRepo)),
		NewExportHandler(services.NewExportService(empRepo, filepath.Join(dir, "exports"))),
	)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := middleware.Chain(router,
		middleware.Logging(logger),
		middleware.Recover(logger),
		middleware.Auth(map[string]string{"admin": "admin123"}, "/health"),
		middleware.Timeout(5*time.Second),
	)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestFullFlow(t *testing.T) {
	srv := newIntegrationServer(t)
	call := func(method, path, body string, auth bool) (int, string) {
		t.Helper()
		req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if auth {
			req.SetBasicAuth("admin", "admin123")
		}
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, string(b)
	}

	if code, _ := call("GET", "/employees", "", false); code != 401 {
		t.Fatal(code)
	}
	if code, _ := call("GET", "/health", "", false); code != 200 {
		t.Fatal(code)
	}
	code, body := call("POST", "/departments", `{"name":"Engineering"}`, true)
	if code != 201 {
		t.Fatal(code, body)
	}
	code, body = call("POST", "/employees", `{"name":"Nguyễn Văn Ấn","age":28,"position":"Backend Engineer","departmentId":1,"salary":2000}`, true)
	if code != 201 {
		t.Fatal(code, body)
	}
	if code, body = call("GET", "/employees?limit=10&offset=0&departmentId=1", "", true); code != 200 || !strings.Contains(body, `"totalCount":1`) {
		t.Fatal(body)
	}
	if code, _ = call("PUT", "/employees/1", `{"position":"Lead"}`, true); code != 200 {
		t.Fatal(code)
	}
	if code, body = call("GET", "/employees/search?keyword=lead", "", true); code != 200 || !strings.Contains(body, `"totalCount":1`) {
		t.Fatal(code, body)
	}
	if code, body = call("GET", "/departments/1/employees", "", true); code != 200 || !strings.Contains(body, "Nguyễn Văn Ấn") {
		t.Fatal(body)
	}
	if code, body = call("POST", "/departments", `{"name":"engineering"}`, true); code != 409 {
		t.Fatal(code, body)
	}
	if code, body = call("POST", "/employees", `{"name":"Ghost","age":30,"position":"Dev","departmentId":999,"salary":1000}`, true); code != 422 {
		t.Fatal(code, body)
	}
	if code, body = call("POST", "/employees/export", "", true); code != 200 || !strings.Contains(body, `"count":1`) {
		t.Fatal(body)
	}
	if code, _ = call("DELETE", "/employees/1", "", true); code != 204 {
		t.Fatal(code)
	}
	if code, _ = call("GET", "/employees/1", "", true); code != 404 {
		t.Fatal(code)
	}
}
