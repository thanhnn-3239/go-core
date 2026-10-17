package handlers

import (
	"strings"
	"testing"
)

func TestExportEndpoint(t *testing.T) {
	h, _ := newTestRouter(t) // newTestRouter uses t.TempDir() as the export dir
	rec := do(t, h, "POST", "/employees/export", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"count":0`) || !strings.Contains(rec.Body.String(), `"json":`) || !strings.Contains(rec.Body.String(), `"csv":`) {
		t.Fatal(rec.Body)
	}
}
