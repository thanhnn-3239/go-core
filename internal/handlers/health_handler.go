// Package handlers holds the HTTP handlers and the router.
package handlers

import (
	"net/http"

	"go-core/internal/utils"
)

// Health reports that the server is up.
func Health(w http.ResponseWriter, _ *http.Request) error {
	return utils.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
