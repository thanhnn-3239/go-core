package handlers

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"go-core/internal/repositories/fakerepo"
	"go-core/internal/services"
)

// newTestRouter builds the router on top of an in-memory store; exports go to
// a per-test temp directory.
func newTestRouter(t *testing.T) (http.Handler, *fakerepo.Store) {
	t.Helper()
	store := fakerepo.New()
	dept := NewDepartmentHandler(services.NewDepartmentService(store.Departments()))
	emp := NewEmployeeHandler(services.NewEmployeeService(store.Employees(), store.Departments()))
	exp := NewExportHandler(services.NewExportService(store.Employees(), t.TempDir()))
	return NewRouter(emp, dept, exp), store
}

// do sends one request through h and returns the recorded response.
func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMain(m *testing.M) {
	// WriteError logs 5xx errors; keep test output clean.
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}
