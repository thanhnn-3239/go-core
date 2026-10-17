package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"go-core/internal/models"
	"go-core/internal/repositories/fakerepo"
	"go-core/internal/testutil"
	"go-core/internal/utils"
)

func seededExport(t *testing.T, names ...string) (*ExportService, string) {
	t.Helper()
	s := fakerepo.New()
	d := &models.Department{Name: "Eng"}
	if err := s.Departments().Create(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		e := &models.Employee{Name: n, Age: 30, Position: "Dev", DepartmentID: d.ID, Salary: 1000}
		if err := s.Employees().Create(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(t.TempDir(), "exports")
	return NewExportService(s.Employees(), dir), dir
}

func TestExportWritesBothFiles(t *testing.T) { // Review Focus #3
	svc, dir := seededExport(t, "Nguyễn Văn Ấn", "Bình, \"Quote\"")
	res, err := svc.Export(context.Background())
	if err != nil || res.Count != 2 || res.Files["json"] != filepath.Join(dir, "employees.json") {
		t.Fatal(res, err)
	}
	if res.Files["csv"] != filepath.Join(dir, "employees.csv") {
		t.Fatal(res)
	}
	var fromJSON []models.Employee
	b, err := os.ReadFile(res.Files["json"])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &fromJSON); err != nil || len(fromJSON) != 2 || fromJSON[0].Name != "Nguyễn Văn Ấn" {
		t.Fatal(string(b), err)
	}
	raw, err := os.ReadFile(res.Files["csv"])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, []byte("\xEF\xBB\xBF")) {
		t.Fatal("missing BOM")
	}
	rows, err := csv.NewReader(bytes.NewReader(raw[3:])).ReadAll()
	if err != nil || len(rows) != 3 {
		t.Fatal(rows, err)
	}
	if strings.Join(rows[0], ",") != "id,name,age,position,departmentId,salary,createdAt,updatedAt" {
		t.Fatal(rows[0])
	}
	if rows[1][1] != "Nguyễn Văn Ấn" || rows[2][1] != `Bình, "Quote"` {
		t.Fatal(rows)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(matches) != 0 {
		t.Fatal(matches)
	}
}

func TestExportOneWriterFails(t *testing.T) {
	svc, dir := seededExport(t, "A")
	errDiskFull := errors.New("disk full")
	svc.writeCSV = func(context.Context, string, []models.Employee) error { return errDiskFull }
	_, err := svc.Export(context.Background())
	testutil.AssertAppError(t, err, utils.KindInternal, "internal server error")
	if !errors.Is(err, errDiskFull) {
		t.Fatalf("err = %v, want it to wrap errDiskFull", err)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(matches) != 0 {
		t.Fatal(matches)
	}
}

func TestExportConcurrentCalls(t *testing.T) {
	svc, _ := seededExport(t, "A", "B", "C")
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if res, err := svc.Export(context.Background()); err != nil || res.Count != 3 {
				t.Error(res, err)
			}
		}()
	}
	wg.Wait()
}

func TestExportCanceledContext(t *testing.T) {
	svc, dir := seededExport(t, "A")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.Export(ctx); err == nil {
		t.Fatal("want error")
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(matches) != 0 {
		t.Fatal(matches)
	}
}

func TestDefaultWritersCleanUpTmpOnCancel(t *testing.T) {
	writers := map[string]func(context.Context, string, []models.Employee) error{
		"json": writeJSONFile,
		"csv":  writeCSVFile,
	}
	for name, w := range writers {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "out."+name)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := w(ctx, path, []models.Employee{{ID: 1, Name: "A"}}); !errors.Is(err, context.Canceled) {
				t.Fatalf("err = %v, want context.Canceled", err)
			}
			for _, p := range []string{path, path + ".tmp"} {
				if _, err := os.Stat(p); !os.IsNotExist(err) {
					t.Fatalf("%s should not exist (err = %v)", p, err)
				}
			}
		})
	}
}

func TestExportDeadlineExceededIsTimeout(t *testing.T) {
	svc, _ := seededExport(t, "A")
	svc.writeJSON = func(context.Context, string, []models.Employee) error {
		return fmt.Errorf("write: %w", context.DeadlineExceeded)
	}
	_, err := svc.Export(context.Background())
	testutil.AssertAppError(t, err, utils.KindTimeout, "request timed out")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want it to wrap DeadlineExceeded", err)
	}
}

func TestExportEmptyList(t *testing.T) {
	svc, _ := seededExport(t)
	res, err := svc.Export(context.Background())
	if err != nil || res.Count != 0 {
		t.Fatal(res, err)
	}
	b, err := os.ReadFile(res.Files["json"])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(strings.Fields(string(b)), "") != "[]" {
		t.Fatalf("json = %q, want []", b)
	}
	var got []models.Employee
	if err := json.Unmarshal(b, &got); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("unmarshal = %#v, %v; want empty non-nil slice", got, err)
	}
	raw, err := os.ReadFile(res.Files["csv"])
	if err != nil {
		t.Fatal(err)
	}
	want := "\xEF\xBB\xBFid,name,age,position,departmentId,salary,createdAt,updatedAt\n"
	if string(raw) != want {
		t.Fatalf("csv = %q, want %q", raw, want)
	}
}

// flakyCtx reports a non-nil Err after okCalls calls to Err.
type flakyCtx struct {
	context.Context
	mu      sync.Mutex
	okCalls int
}

func (c *flakyCtx) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.okCalls > 0 {
		c.okCalls--
		return nil
	}
	return context.Canceled
}

func TestWritersStopMidwayWhenContextCancels(t *testing.T) {
	emps := make([]models.Employee, 1200)
	for i := range emps {
		emps[i] = models.Employee{ID: int64(i + 1), Name: "A"}
	}
	writers := map[string]func(context.Context, string, []models.Employee) error{
		"json": writeJSONFile,
		"csv":  writeCSVFile,
	}
	for name, w := range writers {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "out."+name)
			// Passes the early checks, then fails at a later 500-row boundary.
			ctx := &flakyCtx{Context: context.Background(), okCalls: 2}
			if err := w(ctx, path, emps); !errors.Is(err, context.Canceled) {
				t.Fatalf("err = %v, want context.Canceled", err)
			}
			for _, p := range []string{path, path + ".tmp"} {
				if _, err := os.Stat(p); !os.IsNotExist(err) {
					t.Fatalf("%s should not exist (err = %v)", p, err)
				}
			}
			if ctx.okCalls != 0 {
				t.Fatalf("ctx.Err called too few times: %d ok calls left", ctx.okCalls)
			}
		})
	}
}

func TestWriteJSONFileLayoutMatchesEncoder(t *testing.T) {
	emps := []models.Employee{{ID: 1, Name: "A"}, {ID: 2, Name: "B<&>"}}
	path := filepath.Join(t.TempDir(), "out.json")
	if err := writeJSONFile(context.Background(), path, emps); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var want bytes.Buffer
	enc := json.NewEncoder(&want)
	enc.SetIndent("", "  ")
	if err := enc.Encode(emps); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("got %q\nwant %q", got, want.Bytes())
	}
}
