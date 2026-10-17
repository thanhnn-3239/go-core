package services

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"go-core/internal/models"
	"go-core/internal/repositories"
	"go-core/internal/utils"
)

// ctxCheckEvery is how many rows are written between ctx.Err() checks.
const ctxCheckEvery = 500

// ExportResult reports how many employees were exported and where.
type ExportResult struct {
	Count int               `json:"count"`
	Files map[string]string `json:"files"`
}

// ExportService writes all employees to JSON and CSV files concurrently.
type ExportService struct {
	repo repositories.EmployeeRepository
	dir  string

	// runMu lets only one export run at a time.
	runMu sync.Mutex

	// Replaceable in tests.
	writeJSON, writeCSV func(ctx context.Context, path string, emps []models.Employee) error
}

func NewExportService(repo repositories.EmployeeRepository, dir string) *ExportService {
	return &ExportService{repo: repo, dir: dir, writeJSON: writeJSONFile, writeCSV: writeCSVFile}
}

// Export loads all employees once, then writes employees.json and
// employees.csv in two goroutines. A ListAll error is returned unchanged;
// writer failures are joined into an Internal error.
func (s *ExportService) Export(ctx context.Context) (ExportResult, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	emps, err := s.repo.ListAll(ctx)
	if err != nil {
		return ExportResult{}, err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return ExportResult{}, utils.Internal(err)
	}

	res := ExportResult{Count: len(emps), Files: map[string]string{}}
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex // guards res.Files and errs
		errs []error
	)
	record := func(kind, path string, err error) {
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			errs = append(errs, err)
			return
		}
		res.Files[kind] = path
	}

	jsonPath := filepath.Join(s.dir, "employees.json")
	csvPath := filepath.Join(s.dir, "employees.csv")
	wg.Add(2)
	go func() {
		defer wg.Done()
		record("json", jsonPath, s.writeJSON(ctx, jsonPath, emps))
	}()
	go func() {
		defer wg.Done()
		record("csv", csvPath, s.writeCSV(ctx, csvPath, emps))
	}()
	wg.Wait()

	if len(errs) > 0 {
		return ExportResult{}, utils.Internal(errors.Join(errs...))
	}
	return res, nil
}

// writeAtomic writes to path+".tmp" via fill, then renames it over path.
// On any error the temporary file is closed and removed.
func writeAtomic(path string, fill func(w io.Writer) error) (err error) {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(tmp)
		}
	}()
	if err = fill(f); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func writeJSONFile(ctx context.Context, path string, emps []models.Employee) error {
	return writeAtomic(path, func(w io.Writer) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if emps == nil {
			emps = []models.Employee{}
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(emps)
	})
}

func writeCSVFile(ctx context.Context, path string, emps []models.Employee) error {
	return writeAtomic(path, func(w io.Writer) error {
		if _, err := io.WriteString(w, "\xEF\xBB\xBF"); err != nil {
			return err
		}
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"id", "name", "age", "position", "departmentId", "salary", "createdAt", "updatedAt"}); err != nil {
			return err
		}
		for i, e := range emps {
			if i%ctxCheckEvery == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			row := []string{
				strconv.FormatInt(e.ID, 10),
				e.Name,
				strconv.Itoa(e.Age),
				e.Position,
				strconv.FormatInt(e.DepartmentID, 10),
				strconv.FormatInt(e.Salary, 10),
				e.CreatedAt.Format(time.RFC3339),
				e.UpdatedAt.Format(time.RFC3339),
			}
			if err := cw.Write(row); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	})
}
