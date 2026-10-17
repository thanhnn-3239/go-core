package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-core/internal/repositories/fakerepo"
	"go-core/internal/services"
)

// newTestRouter builds the router on top of an in-memory store.
func newTestRouter(t *testing.T) (http.Handler, *fakerepo.Store) {
	t.Helper()
	store := fakerepo.New()
	dept := NewDepartmentHandler(services.NewDepartmentService(store.Departments()))
	return NewRouter(&EmployeeHandler{}, dept, &ExportHandler{}), store
}

// do sends one request through h and returns the recorded response.
func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
