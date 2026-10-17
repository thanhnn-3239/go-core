package handlers

import (
	"net/http"

	"go-core/internal/services"
	"go-core/internal/utils"
)

// ExportHandler serves POST /employees/export.
type ExportHandler struct {
	svc *services.ExportService
}

func NewExportHandler(svc *services.ExportService) *ExportHandler {
	return &ExportHandler{svc: svc}
}

func (h *ExportHandler) Export(w http.ResponseWriter, r *http.Request) error {
	res, err := h.svc.Export(r.Context())
	if err != nil {
		return err
	}
	return utils.WriteJSON(w, http.StatusOK, res)
}
