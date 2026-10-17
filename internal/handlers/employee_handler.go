package handlers

import (
	"net/http"

	"go-core/internal/models"
	"go-core/internal/services"
	"go-core/internal/utils"
)

// EmployeeHandler serves the /employees endpoints.
type EmployeeHandler struct {
	svc *services.EmployeeService
}

func NewEmployeeHandler(svc *services.EmployeeService) *EmployeeHandler {
	return &EmployeeHandler{svc: svc}
}

func (h *EmployeeHandler) Create(w http.ResponseWriter, r *http.Request) error {
	var in models.CreateEmployeeInput
	if err := utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	e, err := h.svc.Create(r.Context(), in)
	if err != nil {
		return err
	}
	return utils.WriteJSON(w, http.StatusCreated, e)
}

func (h *EmployeeHandler) List(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	limit, offset, err := utils.ParsePagination(q)
	if err != nil {
		return err
	}
	deptID, err := utils.ParseOptionalID(q, "departmentId")
	if err != nil {
		return err
	}
	page, err := h.svc.List(r.Context(), models.ListFilter{Limit: limit, Offset: offset, DepartmentID: deptID})
	if err != nil {
		return err
	}
	return utils.WriteJSON(w, http.StatusOK, page)
}

func (h *EmployeeHandler) Search(w http.ResponseWriter, r *http.Request) error {
	page, err := h.svc.Search(r.Context(), r.URL.Query().Get("keyword"))
	if err != nil {
		return err
	}
	return utils.WriteJSON(w, http.StatusOK, page)
}

func (h *EmployeeHandler) Get(w http.ResponseWriter, r *http.Request) error {
	id, err := utils.ParseID(r.PathValue("id"), "employee")
	if err != nil {
		return err
	}
	e, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		return err
	}
	return utils.WriteJSON(w, http.StatusOK, e)
}

func (h *EmployeeHandler) Update(w http.ResponseWriter, r *http.Request) error {
	id, err := utils.ParseID(r.PathValue("id"), "employee")
	if err != nil {
		return err
	}
	var in models.UpdateEmployeeInput
	if err := utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	e, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		return err
	}
	return utils.WriteJSON(w, http.StatusOK, e)
}

func (h *EmployeeHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	id, err := utils.ParseID(r.PathValue("id"), "employee")
	if err != nil {
		return err
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
