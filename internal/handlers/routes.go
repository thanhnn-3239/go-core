package handlers

import (
	"net/http"
	"slices"
	"strings"

	"go-core/internal/utils"
)

// EmployeeHandler and ExportHandler are filled in by the employee and export
// tasks; they exist here so NewRouter has its final signature.
type EmployeeHandler struct{}

type ExportHandler struct{}

// NewRouter registers every route and wraps the mux with the JSON 404/405
// fallback.
func NewRouter(emp *EmployeeHandler, dept *DepartmentHandler, exp *ExportHandler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", utils.Handle(Health))
	mux.HandleFunc("POST /departments", utils.Handle(dept.Create))
	mux.HandleFunc("GET /departments", utils.Handle(dept.List))
	mux.HandleFunc("GET /departments/{id}/employees", utils.Handle(dept.Employees))
	// Employee and export routes are added later.
	_, _ = emp, exp
	mux.Handle("/", fallback(mux))
	return mux
}

// fallbackMethods are the methods probed to tell 405 from 404.
var fallbackMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}

// fallback handles every request no other pattern matched. It asks the mux
// which methods would match the same path: some means 405 with an Allow
// header, none means 404.
func fallback(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var allowed []string
		for _, m := range fallbackMethods {
			clone := r.Clone(r.Context())
			clone.Method = m
			if _, pattern := mux.Handler(clone); pattern != "" && pattern != "/" {
				allowed = append(allowed, m)
			}
		}
		if len(allowed) == 0 {
			utils.WriteError(w, utils.NotFound("route not found"))
			return
		}
		slices.Sort(allowed)
		w.Header().Set("Allow", strings.Join(allowed, ", "))
		_ = utils.WriteJSON(w, http.StatusMethodNotAllowed,
			utils.ErrorResponse{Error: "method not allowed", Code: http.StatusMethodNotAllowed})
	})
}
