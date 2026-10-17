package handlers

import (
	"net/http"

	"go-core/internal/models"
	"go-core/internal/services"
	"go-core/internal/utils"
)

// DepartmentHandler serves the /departments endpoints.
type DepartmentHandler struct {
	svc *services.DepartmentService
}

func NewDepartmentHandler(svc *services.DepartmentService) *DepartmentHandler {
	return &DepartmentHandler{svc: svc}
}

func (h *DepartmentHandler) Create(w http.ResponseWriter, r *http.Request) error {
	var in models.CreateDepartmentInput
	if err := utils.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	d, err := h.svc.Create(r.Context(), in)
	if err != nil {
		return err
	}
	return utils.WriteJSON(w, http.StatusCreated, d)
}

func (h *DepartmentHandler) List(w http.ResponseWriter, r *http.Request) error {
	list, err := h.svc.List(r.Context())
	if err != nil {
		return err
	}
	if list == nil {
		list = []models.Department{} // encode as [], never null
	}
	return utils.WriteJSON(w, http.StatusOK, map[string]any{"departments": list})
}

func (h *DepartmentHandler) Employees(w http.ResponseWriter, r *http.Request) error {
	id, err := utils.ParseID(r.PathValue("id"), "department")
	if err != nil {
		return err
	}
	d, err := h.svc.GetWithEmployees(r.Context(), id)
	if err != nil {
		return err
	}
	return utils.WriteJSON(w, http.StatusOK, d)
}
