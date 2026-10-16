# Employee Management System: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Xây REST API quản lý nhân viên và phòng ban bằng Go thuần (`net/http` + `database/sql` + SQLite). Có middleware Logging/Recover/Auth/Timeout, export JSON/CSV chạy đồng thời, và mọi lỗi trả JSON theo một format chung.

**Architecture:** Ba tầng Handler → Service → Repository, chỉ gọi theo một chiều. Service chỉ biết đến interface của repository, nhờ vậy test service có thể dùng `fakerepo` (repository giả lưu trong bộ nhớ). Lỗi được đóng gói thành `*utils.AppError{Kind,...}`. Handler có dạng `func(w, r) error` và được bọc bởi `utils.Handle`; chỉ `utils.WriteError` mới ánh xạ `Kind` sang mã HTTP.

**Tech Stack:** Go 1.27, thư viện chuẩn (`net/http`, `encoding/json`, `encoding/csv`, `database/sql`, `log/slog`, `sync`, `embed`, `testing`, `httptest`), driver `modernc.org/sqlite`.

**Spec:** `docs/superpowers/specs/2026-10-16-employee-management-design.md`. Người thực hiện cần đọc spec cùng với plan này. Plan không chép lại các bảng trong spec mà chỉ dẫn về số mục tương ứng.

## Global Constraints

- **Module:** tên module là `go-core`; `go.mod` khai báo `go 1.27`.
- **Phụ thuộc:** ngoài thư viện chuẩn, chỉ được dùng `modernc.org/sqlite`. Không dùng framework, ORM, testify hay thư viện mock.
- **JSON:** tên trường viết camelCase (`departmentId`, `totalCount`, `createdAt`). Danh sách rỗng phải ra `[]`, không bao giờ ra `null`.
- **Format lỗi:** `{"error": string, "code": int}`. Riêng lỗi giá trị trường có thêm `"details": [{"field","message"}]` (spec §4, §8.4).
- **Thông điệp lỗi:** viết bằng tiếng Anh và phải khớp từng ký tự với spec §7.3, §8.2, §8.4, §8.6.
- **Mã HTTP:** Validation 400, Unauthorized 401, NotFound 404, Conflict 409, BusinessRule 422, Internal 500, Timeout 504.
- **Lưu thời gian:** dạng TEXT, định dạng `time.RFC3339Nano`, múi giờ UTC.
- **Thứ tự middleware:** Logging → Recover → Auth(users, "/health") → Timeout.
- **Lệnh test:** mỗi task dùng `go test ./...` và `go vet ./...`. Riêng `CGO_ENABLED=1 go test -race ./...` chỉ chạy ở Task 15, và cần gcc từ MSYS2.
- **Commit:** mỗi commit kết thúc bằng dòng `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

Các trường hợp dưới đây spec không nhắc đến nhưng người dùng dễ gặp. Mỗi trường hợp đã có test ở task được ghi trong ngoặc.

1. **PUT gửi giá trị `null`.** Ví dụ `{"name": null}` thì coi như trường đó không được gửi. Nếu mọi trường đều `null` thì trả 400 `no fields to update` (Task 9).
2. **Hai request tạo cùng một phòng ban gần như đồng thời.** Đúng một request thành công; các request còn lại nhận Conflict, không bị 500 hay `database is locked` (Task 4).
3. **Tên tiếng Việt có dấu.** Ví dụ "Nguyễn Văn Ấn" phải giữ nguyên khi đi qua JSON, SQLite và file CSV có BOM (Task 10).
4. **POST hoặc PUT với body rỗng.** Trả 400 `invalid request body: body must not be empty`, không được trả 500 hay im lặng tạo bản ghi rỗng (Task 2).
5. **`offset` vượt quá tổng số bản ghi.** Trả 200 với `totalCount` là số thật và `employees: []` (Task 5).

---

## File Structure

| File | Trách nhiệm |
|---|---|
| `go.mod`, `.gitignore` | Khai báo module và các thư mục bỏ qua (`data/`, `exports/`) |
| `internal/models/employee.go`, `department.go` | Các struct dữ liệu, xem spec §5 |
| `internal/utils/apperror.go` | `Kind`, `AppError`, `FieldError`, các hàm khởi tạo lỗi, `StatusFor` |
| `internal/utils/response.go` | `WriteJSON`, `WriteError`, `HandlerFunc`, `Handle`, `DecodeJSON`, `ErrorResponse` |
| `internal/utils/params.go` | `ParseID`, `ParsePagination`, `ParseOptionalID` |
| `internal/testutil/assert.go` | Hàm kiểm tra `AppError` dùng chung cho test của các package |
| `internal/repositories/repository.go` | Interface `EmployeeRepository` và `DepartmentRepository` |
| `internal/repositories/messages.go` | Các thông điệp lỗi dùng chung giữa repository, fakerepo và service |
| `internal/repositories/db.go`, `schema.sql` | `OpenSQLite`, chạy migrate, `translateError`, hàm chuyển đổi thời gian |
| `internal/repositories/department_sqlite.go` | `DepartmentSQLite` |
| `internal/repositories/employee_sqlite.go` | `EmployeeSQLite` |
| `internal/repositories/fakerepo/fakerepo.go` | `Store`, `Employees`, `Departments` lưu trong bộ nhớ |
| `internal/services/department_service.go`, `employee_service.go`, `export_service.go` | Kiểm tra dữ liệu, business rule, export |
| `internal/middleware/chain.go`, `context.go`, `logging.go`, `recover.go`, `auth.go`, `timeout.go` | Các middleware |
| `internal/handlers/routes.go`, `health_handler.go`, `department_handler.go`, `employee_handler.go`, `export_handler.go` | Tầng HTTP |
| `internal/handlers/integration_test.go` | Test chạy toàn bộ hệ thống, từ HTTP xuống SQLite thật |
| `cmd/app/main.go` | `loadConfig`, `run`, `main` |

---

### Task 1: Khởi tạo module, models, AppError, testutil

**Files:**
- Create: `go.mod`, `.gitignore`, `internal/models/employee.go`, `internal/models/department.go`, `internal/utils/apperror.go`, `internal/testutil/assert.go`
- Test: `internal/utils/apperror_test.go`

**Interfaces:**
- Produces:
  - Các struct trong models giống hệt spec §5. Thêm một struct `models.CreateDepartmentInput{Name string \`json:"name"\`}`.
  - `type Kind int` với các hằng `KindValidation, KindNotFound, KindConflict, KindBusinessRule, KindUnauthorized, KindTimeout, KindInternal`.
  - `type FieldError struct{Field, Message string}` (JSON tag `field`, `message`).
  - `type AppError struct{Kind Kind; Message string; Details []FieldError; Err error}` với hai method:
    - `Error() string` trả `Message`; nếu `Err != nil` thì trả `Message + ": " + Err.Error()`.
    - `Unwrap() error` trả `Err`.
  - Các hàm khởi tạo lỗi:
    - `Validation(msg string, details ...FieldError) *AppError`
    - `NotFound(msg string) *AppError`
    - `Conflict(msg string, err error) *AppError`
    - `BusinessRule(msg string, err error) *AppError`
    - `Unauthorized() *AppError`, với Message là `"unauthorized"`
    - `Timeout(err error) *AppError`, với Message là `"request timed out"`
    - `Internal(err error) *AppError`, với Message là `"internal server error"`
  - `StatusFor(k Kind) int`.
  - `testutil.AssertAppError(t *testing.T, err error, kind utils.Kind, msg string) *utils.AppError`: dùng `errors.As` để lấy `*AppError` từ `err`, rồi `t.Fatalf` nếu không lấy được, sai `Kind` hoặc sai `Message`. Hàm trả về `*AppError` để test kiểm tra tiếp `Details`.

- [ ] **Step 1: Khởi tạo module**

Run: `go mod init go-core`, rồi sửa dòng `go` trong `go.mod` thành `go 1.27`. Tạo file `.gitignore` gồm 2 dòng: `data/` và `exports/`.

- [ ] **Step 2: Viết test fail** (file `apperror_test.go`, khai báo `package utils`)

```go
func TestStatusFor(t *testing.T) {
	want := map[Kind]int{KindValidation: 400, KindUnauthorized: 401, KindNotFound: 404,
		KindConflict: 409, KindBusinessRule: 422, KindInternal: 500, KindTimeout: 504}
	for k, code := range want {
		if got := StatusFor(k); got != code {
			t.Errorf("StatusFor(%d) = %d, want %d", k, got, code)
		}
	}
}

func TestConstructors(t *testing.T) {
	base := errors.New("disk full")
	if e := Internal(base); e.Message != "internal server error" || !errors.Is(e, base) {
		t.Errorf("Internal: %+v", e)
	}
	if e := Timeout(nil); e.Kind != KindTimeout || e.Message != "request timed out" {
		t.Errorf("Timeout: %+v", e)
	}
	if e := Unauthorized(); e.Kind != KindUnauthorized || e.Message != "unauthorized" {
		t.Errorf("Unauthorized: %+v", e)
	}
	e := Validation("validation failed", FieldError{Field: "age", Message: "must be greater than 0"})
	if len(e.Details) != 1 || e.Details[0].Field != "age" {
		t.Errorf("Validation details: %+v", e.Details)
	}
	var target *AppError
	if !errors.As(fmt.Errorf("wrap: %w", NotFound("Employee not found")), &target) || target.Kind != KindNotFound {
		t.Error("errors.As through wrap failed")
	}
}
```

- [ ] **Step 3: Chạy test để thấy nó fail**

Run: `go test ./internal/utils/ -run 'TestStatusFor|TestConstructors' -v`
Expected: FAIL, compile error `undefined: Kind`.

- [ ] **Step 4: Viết models, `apperror.go` và `testutil/assert.go` theo Interfaces ở trên**

- [ ] **Step 5: Chạy lại test để thấy nó pass**

Run: `go test ./internal/... -v && go vet ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod .gitignore internal/models internal/utils internal/testutil
git commit -m "feat: add module skeleton, models and AppError"
```

---

### Task 2: Ghi phản hồi JSON, ghi lỗi, adapter và đọc body (`utils/response.go`)

**Files:**
- Create: `internal/utils/response.go`
- Test: `internal/utils/response_test.go`

**Interfaces:**
- Consumes: Task 1 (`AppError`, các hàm khởi tạo lỗi, `StatusFor`).
- Produces:
  - `type ErrorResponse struct{Error string; Code int; Details []FieldError}`, với JSON tag `error`, `code`, `details,omitempty`.
  - `WriteJSON(w http.ResponseWriter, status int, v any) error`: đặt `Content-Type: application/json; charset=utf-8`, gọi `WriteHeader(status)`, rồi encode `v`.
  - `WriteError(w http.ResponseWriter, err error)`: làm đúng theo spec §4. Lỗi gốc được ghi log bằng `slog.Default()` khi mã ≥ 500.
  - `type HandlerFunc func(http.ResponseWriter, *http.Request) error` và `Handle(fn HandlerFunc) http.HandlerFunc`.
  - `const MaxBodyBytes = 1 << 20` và `DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error`. Mọi lỗi hàm này trả về đều là `Validation` với Message bắt đầu bằng `"invalid request body: "`.

- [ ] **Step 1: Viết test fail**

```go
func TestWriteError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
		body string
	}{
		{"not found", NotFound("Employee not found"), 404, `{"error":"Employee not found","code":404}`},
		{"wrapped", fmt.Errorf("svc: %w", Conflict(`department "Engineering" already exists`, nil)), 409,
			`{"error":"department \"Engineering\" already exists","code":409}`},
		{"details", Validation("validation failed", FieldError{Field: "name", Message: "is required"}), 400,
			`{"error":"validation failed","code":400,"details":[{"field":"name","message":"is required"}]}`},
		{"bare deadline", fmt.Errorf("q: %w", context.DeadlineExceeded), 504, `{"error":"request timed out","code":504}`},
		{"unknown", errors.New("boom"), 500, `{"error":"internal server error","code":500}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteError(rec, tc.err)
			if rec.Code != tc.code || strings.TrimSpace(rec.Body.String()) != tc.body {
				t.Fatalf("got %d %s", rec.Code, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
				t.Fatalf("content-type %q", ct)
			}
		})
	}
}

func TestDecodeJSON(t *testing.T) {
	type in struct{ Name string `json:"name"`; Age int `json:"age"` }
	cases := []struct{ name, body, wantMsg string }{
		{"ok", `{"name":"A","age":1}`, ""},
		{"empty", ``, "invalid request body: body must not be empty"},
		{"unknown field", `{"agee":1}`, `invalid request body: unknown field "agee"`},
		{"trailing data", `{"name":"A"}{"x":1}`, "invalid request body: unexpected data after JSON object"},
		{"too large", `{"name":"` + strings.Repeat("a", MaxBodyBytes) + `"}`, "invalid request body: body exceeds 1MB limit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
			var dst in
			err := DecodeJSON(httptest.NewRecorder(), r, &dst)
			if tc.wantMsg == "" {
				if err != nil || dst.Name != "A" { t.Fatalf("err=%v dst=%+v", err, dst) }
				return
			}
			testutil.AssertAppError(t, err, KindValidation, tc.wantMsg)
		})
	}
	// Sai kiểu dữ liệu: chỉ kiểm tra Kind và tiền tố của thông điệp
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"age":"abc"}`))
	var ae *AppError
	if err := DecodeJSON(httptest.NewRecorder(), r, &struct{ Age int `json:"age"` }{}); !errors.As(err, &ae) ||
		ae.Kind != KindValidation || !strings.HasPrefix(ae.Message, "invalid request body: ") {
		t.Fatalf("type mismatch: %v", err)
	}
}

func TestHandle(t *testing.T) {
	h := Handle(func(w http.ResponseWriter, r *http.Request) error { return NotFound("Employee not found") })
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 404 { t.Fatalf("code %d", rec.Code) }
}
```

Lưu ý về package của file test:
- `internal/testutil` import `internal/utils`. Vì vậy `response_test.go` phải khai báo `package utils_test` thì mới import được `testutil` mà không bị vòng import.
- Để code mẫu ở trên chạy được mà không cần tiền tố `utils.`, file này import bằng dot-import: `import . "go-core/internal/utils"`.
- `apperror_test.go` của Task 1 không cần đổi, vẫn để `package utils`. Go cho phép hai package `utils` và `utils_test` cùng nằm trong một thư mục.
- Struct của package khác phải khởi tạo có tên trường (ví dụ `FieldError{Field: …, Message: …}`), nếu không `go vet` sẽ cảnh báo.

- [ ] **Step 2: Chạy test để thấy nó fail**

Run: `go test ./internal/utils/ -v`
Expected: FAIL, `undefined: WriteError`.

- [ ] **Step 3: Viết `response.go`**

Các điểm cụ thể của `DecodeJSON`:
- Bọc body bằng `http.MaxBytesReader(w, r.Body, MaxBodyBytes)`, rồi tạo `json.NewDecoder` với `DisallowUnknownFields()`.
- Ánh xạ lỗi:
  - `io.EOF` ở lần `Decode` đầu tiên → `body must not be empty`.
  - Lỗi có kiểu `*http.MaxBytesError` (kiểm tra bằng `errors.As`) → `body exceeds 1MB limit`.
  - Lỗi khác → `strings.TrimPrefix(err.Error(), "json: ")`.
- Sau khi decode thành công, gọi `dec.Decode(&struct{}{})` thêm một lần. Lần này phải trả `io.EOF`; nếu không thì trả `unexpected data after JSON object`.

- [ ] **Step 4: Chạy lại test để thấy nó pass**

Run: `go test ./internal/utils/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/utils
git commit -m "feat: add JSON response, error writer, handler adapter and body decoder"
```

---

### Task 3: Đọc tham số (`utils/params.go`)

**Files:**
- Create: `internal/utils/params.go`
- Test: `internal/utils/params_test.go`

**Interfaces:**
- Produces:
  - `ParseID(raw, entity string) (int64, error)`. Tham số `entity` là `"employee"` hoặc `"department"`. Lỗi trả về có dạng `Validation(fmt.Sprintf("invalid %s id: %q", entity, raw))`.
  - `ParsePagination(q url.Values) (limit, offset int, err error)`.
  - `ParseOptionalID(q url.Values, key string) (*int64, error)`. Trả `nil, nil` khi tham số để trống.
  - Thông điệp lỗi lấy đúng theo spec §8.2.

- [ ] **Step 1: Viết test fail**

```go
func TestParseID(t *testing.T) {
	for _, raw := range []string{"abc", "0", "-1", "1.5", "99999999999999999999", ""} {
		_, err := utils.ParseID(raw, "employee")
		testutil.AssertAppError(t, err, utils.KindValidation, fmt.Sprintf("invalid employee id: %q", raw))
	}
	if id, err := utils.ParseID("42", "department"); err != nil || id != 42 { t.Fatalf("%d %v", id, err) }
}

func TestParsePagination(t *testing.T) {
	cases := []struct{ query string; limit, offset int; wantMsg string }{
		{"", 10, 0, ""},
		{"limit=100&offset=5", 100, 5, ""},
		{"limit=0", 0, 0, "limit must be between 1 and 100"},
		{"limit=101", 0, 0, "limit must be between 1 and 100"},
		{"limit=x", 0, 0, "limit must be between 1 and 100"},
		{"limit=10abc", 0, 0, "limit must be between 1 and 100"},
		{"offset=-5", 0, 0, "offset must be a non-negative integer"},
		{"offset=x", 0, 0, "offset must be a non-negative integer"},
	}
	for _, tc := range cases {
		q, _ := url.ParseQuery(tc.query)
		l, o, err := utils.ParsePagination(q)
		if tc.wantMsg != "" { testutil.AssertAppError(t, err, utils.KindValidation, tc.wantMsg); continue }
		if err != nil || l != tc.limit || o != tc.offset { t.Errorf("%q: %d %d %v", tc.query, l, o, err) }
	}
}

func TestParseOptionalID(t *testing.T) {
	q, _ := url.ParseQuery("departmentId=3")
	if p, err := utils.ParseOptionalID(q, "departmentId"); err != nil || p == nil || *p != 3 { t.Fatal(p, err) }
	if p, err := utils.ParseOptionalID(url.Values{}, "departmentId"); err != nil || p != nil { t.Fatal(p, err) }
	for _, raw := range []string{"x", "0"} {
		_, err := utils.ParseOptionalID(url.Values{"departmentId": {raw}}, "departmentId")
		testutil.AssertAppError(t, err, utils.KindValidation, fmt.Sprintf("invalid departmentId: %q", raw))
	}
}
```

- [ ] **Step 2: Chạy test để thấy nó fail**

Run: `go test ./internal/utils/ -run 'TestParse' -v`
Expected: FAIL, `undefined: utils.ParseID`.

- [ ] **Step 3: Viết `params.go`**

Dùng `strconv.ParseInt(raw, 10, 64)` và `strconv.Atoi`. Số không parse được hoặc nằm ngoài khoảng cho phép đều trả về cùng một thông điệp lỗi của tham số đó.

- [ ] **Step 4: Chạy lại test để thấy nó pass**

Run: `go test ./internal/utils/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/utils
git commit -m "feat: add path and query parameter parsing"
```

---

### Task 4: Nền tảng repository (DB, schema, chuyển lỗi) và `DepartmentSQLite`

**Files:**
- Create: `internal/repositories/repository.go`, `messages.go`, `db.go`, `schema.sql`, `department_sqlite.go`
- Test: `internal/repositories/helpers_test.go`, `db_test.go`, `department_sqlite_test.go`

**Interfaces:**
- Consumes: models, utils (Task 1).
- Produces:
  - Hai interface giống hệt spec §7.1.
  - `messages.go`:
    - `const MsgEmployeeNotFound = "Employee not found"`
    - `const MsgDepartmentNotFound = "Department not found"`
    - `func MsgDepartmentExists(name string) string`, trả về `fmt.Sprintf("department %q already exists", name)`
    - `func MsgDepartmentMissing(id int64) string`, trả về `fmt.Sprintf("department %d does not exist", id)`
  - `OpenSQLite(ctx context.Context, path string) (*sql.DB, error)`: chạy `os.MkdirAll(filepath.Dir(path), 0o755)`, mở DB với DSN theo spec §6.2, chạy `schema.sql` (nhúng bằng `//go:embed`), rồi `PingContext`.
  - Hàm và struct nội bộ:
    - `type errMsgs struct{ notFound, conflict, foreignKey string }`
    - `translateError(err error, m errMsgs) error`: làm theo bảng spec §7.3. Lỗi `nil` thì trả `nil`.
    - `formatTime(time.Time) string` và `parseTime(string) (time.Time, error)`.
  - `NewDepartmentSQLite(ctx context.Context, db *sql.DB) (*DepartmentSQLite, error)`: prepare sẵn các câu lệnh. Có thêm method `(*DepartmentSQLite) Close() error`.
  - Test helpers (package `repositories`):
    - `newTestDB(t) *sql.DB`: mở DB tại `filepath.Join(t.TempDir(), "sub", "test.db")` và đăng ký `t.Cleanup` để đóng DB.
    - `newDeptRepo(t) *DepartmentSQLite`, chỉ là `newDeptRepoWithDB` bỏ giá trị trả về thứ hai.
    - `newDeptRepoWithDB(t) (*DepartmentSQLite, *sql.DB)`: tạo repo trên `newTestDB(t)` và đăng ký `t.Cleanup` gọi `Close`.
    - `insertEmployeeRow(t, db, deptID int64, name string, deleted bool) int64`: chèn thẳng một dòng bằng SQL, dùng cho test JOIN trước khi có `EmployeeSQLite`.

- [ ] **Step 1: Viết test fail**

```go
func TestOpenSQLiteCreatesDirAndEnablesFK(t *testing.T) {
	db := newTestDB(t)
	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 { t.Fatalf("fk=%d err=%v", fk, err) }
	var mode string
	db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if mode != "wal" { t.Fatalf("journal_mode=%s", mode) }
}

func TestTranslateError(t *testing.T) {
	m := errMsgs{notFound: MsgEmployeeNotFound}
	if translateError(nil, m) != nil { t.Fatal("nil") }
	testutil.AssertAppError(t, translateError(sql.ErrNoRows, m), utils.KindNotFound, "Employee not found")
	testutil.AssertAppError(t, translateError(fmt.Errorf("q: %w", context.DeadlineExceeded), m), utils.KindTimeout, "request timed out")
	testutil.AssertAppError(t, translateError(errors.New("conn reset"), m), utils.KindInternal, "internal server error")
}

func TestDepartmentCreateListExists(t *testing.T) {
	repo := newDeptRepo(t) // helper: NewDepartmentSQLite(ctx, newTestDB(t)) + Cleanup Close
	ctx := context.Background()
	d := &models.Department{Name: "Engineering"}
	if err := repo.Create(ctx, d); err != nil || d.ID == 0 || d.CreatedAt.IsZero() { t.Fatalf("%+v %v", d, err) }
	err := repo.Create(ctx, &models.Department{Name: "engineering"})
	testutil.AssertAppError(t, err, utils.KindConflict, `department "engineering" already exists`)
	repo.Create(ctx, &models.Department{Name: "Sales"})
	list, _ := repo.List(ctx)
	if len(list) != 2 || list[0].Name != "Engineering" || list[1].Name != "Sales" { t.Fatalf("%+v", list) }
	if ok, _ := repo.Exists(ctx, d.ID); !ok { t.Fatal("exists") }
	if ok, _ := repo.Exists(ctx, 999); ok { t.Fatal("not exists") }
}

func TestDepartmentConcurrentDuplicate(t *testing.T) { // Review Focus #2
	repo := newDeptRepo(t)
	var wg sync.WaitGroup
	var mu sync.Mutex
	okCount, conflicts := 0, 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := repo.Create(context.Background(), &models.Department{Name: "Ops"})
			var ae *utils.AppError
			mu.Lock(); defer mu.Unlock()
			switch {
			case err == nil: okCount++
			case errors.As(err, &ae) && ae.Kind == utils.KindConflict: conflicts++
			default: t.Errorf("unexpected: %v", err)
			}
		}()
	}
	wg.Wait()
	if okCount != 1 || conflicts != 9 { t.Fatalf("ok=%d conflicts=%d", okCount, conflicts) }
}

func TestDepartmentGetWithEmployees(t *testing.T) {
	repo, db := newDeptRepoWithDB(t)
	ctx := context.Background()
	empty := &models.Department{Name: "Empty"}; repo.Create(ctx, empty)
	full := &models.Department{Name: "Full"}; repo.Create(ctx, full)
	a := insertEmployeeRow(t, db, full.ID, "An", false)
	insertEmployeeRow(t, db, full.ID, "Gone", true)
	got, err := repo.GetWithEmployees(ctx, empty.ID)
	if err != nil || got.Employees == nil || len(got.Employees) != 0 { t.Fatalf("empty: %+v %v", got, err) }
	got, _ = repo.GetWithEmployees(ctx, full.ID)
	if len(got.Employees) != 1 || got.Employees[0].ID != a || got.Name != "Full" { t.Fatalf("full: %+v", got) }
	_, err = repo.GetWithEmployees(ctx, 999)
	testutil.AssertAppError(t, err, utils.KindNotFound, "Department not found")
}
```

- [ ] **Step 2: Chạy test để thấy nó fail**

Run: `go get modernc.org/sqlite && go test ./internal/repositories/ -v`
Expected: FAIL, `undefined: OpenSQLite`.

- [ ] **Step 3: Viết `repository.go`, `messages.go`, `schema.sql`, `db.go`**

Hằng số mã lỗi nằm trong `modernc.org/sqlite/lib`: `sqlite3.SQLITE_CONSTRAINT_UNIQUE` và `sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY`. Dùng `errors.As(err, &se)` với `se *sqlite.Error`, rồi so `se.Code()` với hai hằng số đó.

- [ ] **Step 4: Viết `department_sqlite.go`**

- Câu JOIN lấy đúng theo spec §7.2.
- Các cột của bảng `e` được scan vào `sql.NullInt64` và `sql.NullString`. Dòng nào có `e.id` NULL thì không thêm nhân viên vào kết quả.
- `Employees` được khởi tạo bằng `[]models.Employee{}`, để JSON ra `[]` chứ không ra `null`.
- `Create` gọi `translateError` với `errMsgs{conflict: MsgDepartmentExists(d.Name)}`.

- [ ] **Step 5: Chạy lại test để thấy nó pass**

Run: `go test ./internal/repositories/ -v`
Expected: PASS.

Nếu test Conflict hoặc FK fail vì `se.Code()` chỉ trả mã chính (`SQLITE_CONSTRAINT`) thay vì mã mở rộng, so mã bằng `se.Code() & 0xff` với `SQLITE_CONSTRAINT`, rồi phân biệt UNIQUE và FK bằng mã mở rộng. Test ở trên là căn cứ để biết cách nào đúng.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/repositories
git commit -m "feat: add SQLite setup, error translation and department repository"
```

---

### Task 5: `EmployeeSQLite`: tạo, đọc, danh sách, tìm kiếm, xoá mềm

**Files:**
- Create: `internal/repositories/employee_sqlite.go`
- Test: `internal/repositories/employee_sqlite_test.go`

**Interfaces:**
- Consumes: Task 4 (`OpenSQLite`, `translateError`, `messages.go`, `DepartmentSQLite` dùng để tạo dữ liệu mẫu).
- Produces:
  - `NewEmployeeSQLite(ctx context.Context, db *sql.DB) (*EmployeeSQLite, error)` và `(*EmployeeSQLite) Close() error`.
  - Các method `Create`, `GetByID`, `List`, `Search`, `SoftDelete`, `ListAll` theo spec §7.1. `Update` làm ở Task 6; trong task này viết stub trả `utils.Internal(errors.New("not implemented"))` để struct vẫn thỏa interface.
  - `EmployeeSQLite` có trường `db *sql.DB`, vì `List` và `Update` cần gọi `BeginTx`.
  - Test helpers:
    - `newEmpRepo(t) (*EmployeeSQLite, int64)`: tạo DB và phòng ban "Eng"; trả về repo cùng id của phòng ban đó.
    - `seedDept(t, emps *EmployeeSQLite, name string) int64`: tạo phòng ban bằng `NewDepartmentSQLite(ctx, emps.db)`.
    - `seedEmployee(t, emps *EmployeeSQLite, deptID int64, name, position string) *models.Employee`: tạo nhân viên với `Age: 30` và `Salary: 1000`.

- [ ] **Step 1: Viết test fail**

```go
func TestEmployeeCreateGet(t *testing.T) {
	emps, deptID := newEmpRepo(t) // helper: tạo DB, tạo phòng ban "Eng", trả về repo và id của phòng ban
	ctx := context.Background()
	e := &models.Employee{Name: "Nguyễn Văn Ấn", Age: 28, Position: "Backend Engineer", DepartmentID: deptID, Salary: 2000}
	if err := emps.Create(ctx, e); err != nil || e.ID == 0 || e.CreatedAt.IsZero() || !e.CreatedAt.Equal(e.UpdatedAt) { t.Fatal(e, err) }
	got, err := emps.GetByID(ctx, e.ID)
	if err != nil || got.Name != "Nguyễn Văn Ấn" || got.Salary != 2000 { t.Fatal(got, err) }
	_, err = emps.GetByID(ctx, 999)
	testutil.AssertAppError(t, err, utils.KindNotFound, "Employee not found")
	err = emps.Create(ctx, &models.Employee{Name: "X", Age: 1, Position: "P", DepartmentID: 777, Salary: 1})
	testutil.AssertAppError(t, err, utils.KindBusinessRule, "department 777 does not exist")
}

func TestEmployeeListPagination(t *testing.T) {
	emps, d1 := newEmpRepo(t)
	d2 := seedDept(t, emps, "Sales")
	for i := 0; i < 5; i++ { seedEmployee(t, emps, d1, fmt.Sprintf("E%d", i), "Dev") }
	seedEmployee(t, emps, d2, "S0", "Sales")
	ctx := context.Background()
	page, _ := emps.List(ctx, models.ListFilter{Limit: 2, Offset: 2})
	if page.TotalCount != 6 || len(page.Employees) != 2 || page.Employees[0].Name != "E2" { t.Fatalf("%+v", page) }
	page, _ = emps.List(ctx, models.ListFilter{Limit: 10, DepartmentID: &d2})
	if page.TotalCount != 1 || page.Employees[0].Name != "S0" { t.Fatalf("filter: %+v", page) }
	page, _ = emps.List(ctx, models.ListFilter{Limit: 10, Offset: 1000}) // Review Focus #5
	if page.TotalCount != 6 || page.Employees == nil || len(page.Employees) != 0 { t.Fatalf("beyond: %+v", page) }
	missing := int64(999)
	page, _ = emps.List(ctx, models.ListFilter{Limit: 10, DepartmentID: &missing})
	if page.TotalCount != 0 || page.Employees == nil { t.Fatalf("missing dept: %+v", page) }
}

func TestEmployeeSearch(t *testing.T) {
	emps, d := newEmpRepo(t)
	seedEmployee(t, emps, d, "An Nguyen", "Dev")
	seedEmployee(t, emps, d, "Binh", "Android Engineer")
	seedEmployee(t, emps, d, "100% Pure", "QA")
	seedEmployee(t, emps, d, "Chi", "Tester")
	ctx := context.Background()
	got, _ := emps.Search(ctx, "AN")
	if len(got) != 2 { t.Fatalf("an: %+v", got) } // khớp name "An Nguyen" và position "Android"
	got, _ = emps.Search(ctx, "%")
	if len(got) != 1 || got[0].Name != "100% Pure" { t.Fatalf("percent: %+v", got) }
	got, _ = emps.Search(ctx, "_")
	if len(got) != 0 { t.Fatalf("underscore: %+v", got) }
}

func TestEmployeeSoftDelete(t *testing.T) {
	emps, d := newEmpRepo(t)
	e := seedEmployee(t, emps, d, "Gone", "Dev")
	keep := seedEmployee(t, emps, d, "Keep", "Dev")
	ctx := context.Background()
	if err := emps.SoftDelete(ctx, e.ID); err != nil { t.Fatal(err) }
	testutil.AssertAppError(t, emps.SoftDelete(ctx, e.ID), utils.KindNotFound, "Employee not found")
	_, err := emps.GetByID(ctx, e.ID)
	testutil.AssertAppError(t, err, utils.KindNotFound, "Employee not found")
	page, _ := emps.List(ctx, models.ListFilter{Limit: 10})
	all, _ := emps.ListAll(ctx)
	found, _ := emps.Search(ctx, "Gone")
	if page.TotalCount != 1 || len(all) != 1 || all[0].ID != keep.ID || len(found) != 0 { t.Fatal(page, all, found) }
}

func TestEmployeeExpiredContext(t *testing.T) {
	emps, _ := newEmpRepo(t)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, err := emps.GetByID(ctx, 1)
	testutil.AssertAppError(t, err, utils.KindTimeout, "request timed out")
}
```

- [ ] **Step 2: Chạy test để thấy nó fail**

Run: `go test ./internal/repositories/ -run TestEmployee -v`
Expected: FAIL, `undefined: NewEmployeeSQLite`.

- [ ] **Step 3: Viết `employee_sqlite.go`**

- **Create:** gọi `translateError(err, errMsgs{foreignKey: MsgDepartmentMissing(e.DepartmentID)})` để lỗi FK ra đúng thông điệp.
- **List:** prepare sẵn hai câu lệnh, cả hai lọc phòng ban bằng `(? IS NULL OR department_id = ?)`:
  - câu `COUNT(*)`;
  - câu `SELECT … ORDER BY id LIMIT ? OFFSET ?`.

  Chạy cả hai trong cùng một transaction, mở bằng `db.BeginTx(ctx, nil)`, và lấy statement theo transaction bằng `tx.StmtContext`.
- **Search:** escape trước các ký tự `\`, `%`, `_` trong từ khoá, rồi mới bọc thành `"%"+kw+"%"`.
- **Kết quả rỗng:** mọi slice trả về được khởi tạo bằng `[]models.Employee{}` thay vì để `nil`.

- [ ] **Step 4: Chạy lại test để thấy nó pass**

Run: `go test ./internal/repositories/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repositories
git commit -m "feat: add employee repository create, get, list, search and soft delete"
```

---

### Task 6: `EmployeeSQLite.Update` (cập nhật trong transaction)

**Files:**
- Modify: `internal/repositories/employee_sqlite.go` (thay stub `Update`)
- Test: `internal/repositories/employee_sqlite_test.go`

**Interfaces:**
- Produces: `Update(ctx, id int64, in models.UpdateEmployeeInput) (*models.Employee, error)`. Hàm trả về bản ghi sau khi cập nhật, với `UpdatedAt` mới.

- [ ] **Step 1: Viết test fail**

```go
func TestEmployeeUpdatePartial(t *testing.T) {
	emps, d := newEmpRepo(t)
	e := seedEmployee(t, emps, d, "An", "Dev")
	ctx := context.Background()
	salary := int64(3000)
	got, err := emps.Update(ctx, e.ID, models.UpdateEmployeeInput{Salary: &salary})
	if err != nil || got.Salary != 3000 || got.Name != "An" || got.Position != "Dev" || got.UpdatedAt.Before(e.UpdatedAt) {
		t.Fatalf("%+v %v", got, err)
	}
	reread, _ := emps.GetByID(ctx, e.ID)
	if reread.Salary != 3000 || reread.Age != e.Age { t.Fatalf("persisted: %+v", reread) }
	_, err = emps.Update(ctx, 999, models.UpdateEmployeeInput{Salary: &salary})
	testutil.AssertAppError(t, err, utils.KindNotFound, "Employee not found")
	emps.SoftDelete(ctx, e.ID)
	_, err = emps.Update(ctx, e.ID, models.UpdateEmployeeInput{Salary: &salary})
	testutil.AssertAppError(t, err, utils.KindNotFound, "Employee not found")
	missing := int64(777)
	other := seedEmployee(t, emps, d, "B", "Dev")
	_, err = emps.Update(ctx, other.ID, models.UpdateEmployeeInput{DepartmentID: &missing})
	testutil.AssertAppError(t, err, utils.KindBusinessRule, "department 777 does not exist")
}
```

- [ ] **Step 2: Chạy test để thấy nó fail**

Run: `go test ./internal/repositories/ -run TestEmployeeUpdatePartial -v`
Expected: FAIL, lỗi `not implemented`.

- [ ] **Step 3: Viết `Update`**

Làm theo trình tự transaction ở spec §7.2. Gọi `defer tx.Rollback()` ngay sau `BeginTx`; sau khi `Commit` thành công thì `Rollback` không còn tác dụng gì.

- [ ] **Step 4: Chạy lại test để thấy nó pass**

Run: `go test ./internal/repositories/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repositories
git commit -m "feat: add transactional partial employee update"
```

---

### Task 7: `fakerepo`: repository giả lưu trong bộ nhớ

**Files:**
- Create: `internal/repositories/fakerepo/fakerepo.go`
- Test: `internal/repositories/fakerepo/fakerepo_test.go`

**Interfaces:**
- Consumes: interface và `messages.go` từ Task 4, models.
- Produces:
  - `type Store struct{ FailWith map[string]error; ... }`, có một `sync.Mutex` bảo vệ toàn bộ dữ liệu bên trong.
  - `New() *Store`.
  - `(*Store) Employees() *Employees` thỏa `repositories.EmployeeRepository`.
  - `(*Store) Departments() *Departments` thỏa `repositories.DepartmentRepository`.
  - `New()` khởi tạo sẵn `FailWith` thành map rỗng, để test gán khoá được ngay.
  - Khoá của `FailWith` có dạng `"Employee.<Method>"` hoặc `"Department.<Method>"`. Ví dụ `FailWith["Employee.List"] = utils.Timeout(nil)`. Khi có khoá tương ứng, method trả ngay lỗi đó.
  - Hành vi phải giống SQLite, kể cả thông điệp lỗi:
    - ID tự tăng.
    - Tên phòng ban trùng (không phân biệt hoa thường) → Conflict.
    - `departmentId` không tồn tại ở Create/Update → BusinessRule.
    - Bản ghi đã xoá mềm bị ẩn khỏi mọi thao tác.
    - List/Search sắp theo id.
    - Search không phân biệt hoa thường với chữ ASCII.
    - Slice rỗng trả `[]`, không trả `nil`.
    - Có gán `CreatedAt` và `UpdatedAt`.

- [ ] **Step 1: Viết test fail**

```go
var (
	_ repositories.EmployeeRepository   = (*fakerepo.Employees)(nil)
	_ repositories.DepartmentRepository = (*fakerepo.Departments)(nil)
)

func TestFakeMirrorsSQLiteSemantics(t *testing.T) {
	s := fakerepo.New()
	ctx := context.Background()
	d := &models.Department{Name: "Eng"}
	s.Departments().Create(ctx, d)
	testutil.AssertAppError(t, s.Departments().Create(ctx, &models.Department{Name: "eng"}), utils.KindConflict, `department "eng" already exists`)
	e := &models.Employee{Name: "An", Age: 1, Position: "Dev", DepartmentID: d.ID, Salary: 1}
	s.Employees().Create(ctx, e)
	testutil.AssertAppError(t, s.Employees().Create(ctx, &models.Employee{DepartmentID: 9}), utils.KindBusinessRule, "department 9 does not exist")
	s.Employees().SoftDelete(ctx, e.ID)
	got, _ := s.Departments().GetWithEmployees(ctx, d.ID)
	if got.Employees == nil || len(got.Employees) != 0 { t.Fatalf("%+v", got) }
	s.FailWith["Employee.List"] = utils.Timeout(nil)
	_, err := s.Employees().List(ctx, models.ListFilter{Limit: 10})
	testutil.AssertAppError(t, err, utils.KindTimeout, "request timed out")
}
```

- [ ] **Step 2: Chạy test để thấy nó fail**

Run: `go test ./internal/repositories/fakerepo/ -v`
Expected: FAIL, `undefined: fakerepo.New`.

- [ ] **Step 3: Viết `fakerepo.go`**

Mỗi method trả về **bản sao** của bản ghi (copy struct), không trả con trỏ vào dữ liệu bên trong. Như vậy test không thể vô tình sửa dữ liệu của store.

- [ ] **Step 4: Chạy lại test để thấy nó pass**

Run: `go test ./internal/repositories/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repositories/fakerepo
git commit -m "test: add in-memory fake repositories"
```

---

### Task 8: `DepartmentService`

**Files:**
- Create: `internal/services/department_service.go`
- Test: `internal/services/department_service_test.go`

**Interfaces:**
- Consumes: `repositories.DepartmentRepository`, `fakerepo` (khi test).
- Produces:
  - `NewDepartmentService(repo repositories.DepartmentRepository) *DepartmentService`.
  - `Create(ctx, in models.CreateDepartmentInput) (*models.Department, error)`.
  - `List(ctx) ([]models.Department, error)`.
  - `GetWithEmployees(ctx, id int64) (*models.DepartmentEmployees, error)`.

- [ ] **Step 1: Viết test fail**

```go
func TestDepartmentServiceCreate(t *testing.T) {
	svc := services.NewDepartmentService(fakerepo.New().Departments())
	ctx := context.Background()
	d, err := svc.Create(ctx, models.CreateDepartmentInput{Name: "  Engineering  "})
	if err != nil || d.Name != "Engineering" { t.Fatal(d, err) }
	_, err = svc.Create(ctx, models.CreateDepartmentInput{Name: "   "})
	ae := testutil.AssertAppError(t, err, utils.KindValidation, "validation failed")
	if len(ae.Details) != 1 || ae.Details[0] != (utils.FieldError{Field: "name", Message: "is required"}) { t.Fatal(ae.Details) }
	_, err = svc.Create(ctx, models.CreateDepartmentInput{Name: "engineering"})
	testutil.AssertAppError(t, err, utils.KindConflict, `department "engineering" already exists`)
}

func TestDepartmentServicePassesRepoErrors(t *testing.T) {
	s := fakerepo.New()
	s.FailWith["Department.List"] = utils.Internal(errors.New("db down"))
	_, err := services.NewDepartmentService(s.Departments()).List(context.Background())
	testutil.AssertAppError(t, err, utils.KindInternal, "internal server error")
}
```

- [ ] **Step 2: Chạy test để thấy nó fail**

Run: `go test ./internal/services/ -v`
Expected: FAIL, `undefined: services.NewDepartmentService`.

- [ ] **Step 3: Viết `department_service.go`**

- [ ] **Step 4: Chạy lại test để thấy nó pass**

Run: `go test ./internal/services/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/services
git commit -m "feat: add department service"
```

---

### Task 9: `EmployeeService`: kiểm tra dữ liệu và business rule

**Files:**
- Create: `internal/services/employee_service.go`
- Test: `internal/services/employee_service_test.go`

**Interfaces:**
- Consumes: hai interface repository, `repositories.MsgDepartmentMissing`.
- Produces:
  - `NewEmployeeService(emps repositories.EmployeeRepository, depts repositories.DepartmentRepository) *EmployeeService`.
  - `Create(ctx, in models.CreateEmployeeInput) (*models.Employee, error)`.
  - `GetByID(ctx, id int64) (*models.Employee, error)`.
  - `List(ctx, f models.ListFilter) (models.EmployeePage, error)`.
  - `Search(ctx, keyword string) (models.EmployeePage, error)`. Trả `TotalCount = len(Employees)`. Từ khoá rỗng sau khi `TrimSpace` → `Validation("keyword is required")`.
  - `Update(ctx, id int64, in models.UpdateEmployeeInput) (*models.Employee, error)`.
  - `Delete(ctx, id int64) error`.

- [ ] **Step 1: Viết test fail**

```go
func newEmpSvc(t *testing.T) (*services.EmployeeService, int64) {
	t.Helper()
	s := fakerepo.New()
	d := &models.Department{Name: "Eng"}
	s.Departments().Create(context.Background(), d)
	return services.NewEmployeeService(s.Employees(), s.Departments()), d.ID
}

func TestEmployeeServiceCreateValidation(t *testing.T) {
	svc, _ := newEmpSvc(t)
	_, err := svc.Create(context.Background(), models.CreateEmployeeInput{Name: " ", Age: 0, Position: "", DepartmentID: 0, Salary: -5})
	ae := testutil.AssertAppError(t, err, utils.KindValidation, "validation failed")
	fe := func(f, m string) utils.FieldError { return utils.FieldError{Field: f, Message: m} }
	want := []utils.FieldError{
		fe("name", "is required"), fe("age", "must be greater than 0"), fe("position", "is required"),
		fe("departmentId", "must be a positive integer"), fe("salary", "must be greater than 0"),
	}
	if !reflect.DeepEqual(ae.Details, want) { t.Fatalf("details %+v", ae.Details) }
}

func TestEmployeeServiceCreate(t *testing.T) {
	svc, d := newEmpSvc(t)
	ctx := context.Background()
	e, err := svc.Create(ctx, models.CreateEmployeeInput{Name: " An ", Age: 28, Position: " Dev ", DepartmentID: d, Salary: 2000})
	if err != nil || e.ID == 0 || e.Name != "An" || e.Position != "Dev" { t.Fatal(e, err) }
	_, err = svc.Create(ctx, models.CreateEmployeeInput{Name: "B", Age: 1, Position: "P", DepartmentID: 99, Salary: 1})
	testutil.AssertAppError(t, err, utils.KindBusinessRule, "department 99 does not exist")
}

func TestEmployeeServiceUpdate(t *testing.T) {
	svc, d := newEmpSvc(t)
	ctx := context.Background()
	e, _ := svc.Create(ctx, models.CreateEmployeeInput{Name: "An", Age: 28, Position: "Dev", DepartmentID: d, Salary: 2000})
	_, err := svc.Update(ctx, e.ID, models.UpdateEmployeeInput{})
	testutil.AssertAppError(t, err, utils.KindValidation, "no fields to update")
	age, salary := 0, int64(-1)
	_, err = svc.Update(ctx, e.ID, models.UpdateEmployeeInput{Age: &age, Salary: &salary})
	ae := testutil.AssertAppError(t, err, utils.KindValidation, "validation failed")
	if len(ae.Details) != 2 || ae.Details[0].Field != "age" || ae.Details[1].Field != "salary" { t.Fatal(ae.Details) }
	missing := int64(99)
	_, err = svc.Update(ctx, 12345, models.UpdateEmployeeInput{DepartmentID: &missing}) // 422 xảy ra trước 404
	testutil.AssertAppError(t, err, utils.KindBusinessRule, "department 99 does not exist")
	name := "  Binh "
	got, err := svc.Update(ctx, e.ID, models.UpdateEmployeeInput{Name: &name})
	if err != nil || got.Name != "Binh" || got.Salary != 2000 { t.Fatal(got, err) }
}

func TestEmployeeServiceUpdateNullFields(t *testing.T) { // Review Focus #1
	svc, d := newEmpSvc(t)
	e, _ := svc.Create(context.Background(), models.CreateEmployeeInput{Name: "An", Age: 1, Position: "P", DepartmentID: d, Salary: 1})
	var in models.UpdateEmployeeInput
	json.Unmarshal([]byte(`{"name":null,"age":null}`), &in)
	_, err := svc.Update(context.Background(), e.ID, in)
	testutil.AssertAppError(t, err, utils.KindValidation, "no fields to update")
}

func TestEmployeeServiceSearch(t *testing.T) {
	svc, d := newEmpSvc(t)
	svc.Create(context.Background(), models.CreateEmployeeInput{Name: "An", Age: 1, Position: "Dev", DepartmentID: d, Salary: 1})
	_, err := svc.Search(context.Background(), "   ")
	testutil.AssertAppError(t, err, utils.KindValidation, "keyword is required")
	page, err := svc.Search(context.Background(), " an ")
	if err != nil || page.TotalCount != 1 || len(page.Employees) != 1 { t.Fatal(page, err) }
}
```

- [ ] **Step 2: Chạy test để thấy nó fail**

Run: `go test ./internal/services/ -run TestEmployeeService -v`
Expected: FAIL, `undefined: services.NewEmployeeService`.

- [ ] **Step 3: Viết `employee_service.go`**

- **Hàm validate dùng chung:** viết một hàm nội bộ nhận các con trỏ trường, ví dụ `validateFields(name, position *string, age *int, deptID, salary *int64) []utils.FieldError`. Create truyền vào con trỏ của mọi trường; Update chỉ truyền những trường khác `nil`. Hàm luôn kiểm tra theo thứ tự cố định `name, age, position, departmentId, salary` (spec §8.4).
- **Thứ tự kiểm tra trong `Update`:**
  1. Mọi trường đều `nil` → `no fields to update`.
  2. `TrimSpace` các trường chuỗi, rồi validate.
  3. Nếu có gửi `DepartmentID`, gọi `depts.Exists`.
  4. Gọi `emps.Update`.

- [ ] **Step 4: Chạy lại test để thấy nó pass**

Run: `go test ./internal/services/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/services
git commit -m "feat: add employee service with validation details and business rules"
```

---

### Task 10: `ExportService`: export đồng thời

**Files:**
- Create: `internal/services/export_service.go`
- Test: `internal/services/export_service_test.go`. Đặt file này trong `package services` (white-box) để test thay được các hàm ghi file.

**Interfaces:**
- Consumes: `repositories.EmployeeRepository`.
- Produces:
  - `type ExportResult struct{ Count int \`json:"count"\`; Files map[string]string \`json:"files"\` }`.
  - `NewExportService(repo repositories.EmployeeRepository, dir string) *ExportService`.
  - `Export(ctx context.Context) (ExportResult, error)`. `Files["json"]` là `filepath.Join(dir, "employees.json")`, `Files["csv"]` là `filepath.Join(dir, "employees.csv")`.
  - Hai trường nội bộ có thể thay khi test: `writeJSON, writeCSV func(ctx context.Context, path string, emps []models.Employee) error`. Mỗi hàm tự ghi `path + ".tmp"`, rồi `os.Rename` sang `path`, và xoá file `.tmp` khi có lỗi.

- [ ] **Step 1: Viết test fail**

```go
func seededExport(t *testing.T, names ...string) (*ExportService, string) {
	t.Helper()
	s := fakerepo.New()
	d := &models.Department{Name: "Eng"}
	s.Departments().Create(context.Background(), d)
	for _, n := range names {
		s.Employees().Create(context.Background(), &models.Employee{Name: n, Age: 30, Position: "Dev", DepartmentID: d.ID, Salary: 1000})
	}
	dir := filepath.Join(t.TempDir(), "exports")
	return NewExportService(s.Employees(), dir), dir
}

func TestExportWritesBothFiles(t *testing.T) { // Review Focus #3
	svc, dir := seededExport(t, "Nguyễn Văn Ấn", "Bình, \"Quote\"")
	res, err := svc.Export(context.Background())
	if err != nil || res.Count != 2 || res.Files["json"] != filepath.Join(dir, "employees.json") { t.Fatal(res, err) }
	var fromJSON []models.Employee
	b, _ := os.ReadFile(res.Files["json"])
	if json.Unmarshal(b, &fromJSON); len(fromJSON) != 2 || fromJSON[0].Name != "Nguyễn Văn Ấn" { t.Fatal(string(b)) }
	raw, _ := os.ReadFile(res.Files["csv"])
	if !bytes.HasPrefix(raw, []byte("\xEF\xBB\xBF")) { t.Fatal("missing BOM") }
	rows, err := csv.NewReader(bytes.NewReader(raw[3:])).ReadAll()
	if err != nil || len(rows) != 3 { t.Fatal(rows, err) }
	if strings.Join(rows[0], ",") != "id,name,age,position,departmentId,salary,createdAt,updatedAt" { t.Fatal(rows[0]) }
	if rows[1][1] != "Nguyễn Văn Ấn" || rows[2][1] != `Bình, "Quote"` { t.Fatal(rows) }
}

func TestExportOneWriterFails(t *testing.T) {
	svc, dir := seededExport(t, "A")
	svc.writeCSV = func(context.Context, string, []models.Employee) error { return errors.New("disk full") }
	_, err := svc.Export(context.Background())
	testutil.AssertAppError(t, err, utils.KindInternal, "internal server error")
	matches, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(matches) != 0 { t.Fatal(matches) }
}

func TestExportConcurrentCalls(t *testing.T) {
	svc, _ := seededExport(t, "A", "B", "C")
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if res, err := svc.Export(context.Background()); err != nil || res.Count != 3 { t.Error(res, err) }
		}()
	}
	wg.Wait()
}

func TestExportCanceledContext(t *testing.T) {
	svc, dir := seededExport(t, "A")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.Export(ctx); err == nil { t.Fatal("want error") }
	matches, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(matches) != 0 { t.Fatal(matches) }
}
```

Lưu ý: `fakerepo` phải trả lỗi khi context đã bị huỷ. Thêm vào đầu mọi method của fakerepo đoạn kiểm tra `if err := ctx.Err(); err != nil { return …utils.Timeout(err) }`. Đây là một sửa nhỏ ở Task 7, commit chung với task này.

- [ ] **Step 2: Chạy test để thấy nó fail**

Run: `go test ./internal/services/ -run TestExport -v`
Expected: FAIL, `undefined: NewExportService`.

- [ ] **Step 3: Viết `export_service.go`**

Phần thân của `Export` làm theo code mẫu ở spec §10, trang review mục 4.6. Hai hàm ghi file mặc định:
- Gọi `ctx.Err()` sau mỗi 500 dòng để dừng sớm khi context bị huỷ.
- JSON dùng `SetIndent("", "  ")`.
- CSV ghi BOM UTF-8 trước, sau đó ghi bằng `csv.Writer`. Nhớ gọi `Flush()` rồi kiểm tra `Error()`.
- `ID`, `Age`, `DepartmentID`, `Salary` chuyển sang chuỗi bằng `strconv`. Thời gian ghi theo `time.RFC3339`.

- [ ] **Step 4: Chạy lại test để thấy nó pass**

Run: `go test ./internal/services/ ./internal/repositories/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/services internal/repositories/fakerepo
git commit -m "feat: add concurrent JSON/CSV export with WaitGroup and Mutex"
```

---

### Task 11: Middleware Chain, context, Logging, Recover

**Files:**
- Create: `internal/middleware/chain.go`, `context.go`, `logging.go`, `recover.go`
- Test: `internal/middleware/chain_test.go`, `logging_test.go`, `recover_test.go`

**Interfaces:**
- Consumes: `utils.WriteError`, `utils.Internal`.
- Produces:
  - `type Middleware func(http.Handler) http.Handler`.
  - `Chain(h http.Handler, mws ...Middleware) http.Handler`. Phần tử đầu tiên của `mws` là lớp ngoài cùng.
  - `WithUser(ctx context.Context, name string) context.Context` và `UserFrom(ctx context.Context) (string, bool)`.
  - Nội bộ: struct `requestInfo{User string}`, cùng hai hàm `withRequestInfo` và `requestInfoFrom`.
  - `Logging(logger *slog.Logger) Middleware`: ghi log theo spec §9. Các key là `method`, `url` (giá trị `r.URL.RequestURI()`), `status`, `bytes`, `duration`, `user`.
  - `Recover(logger *slog.Logger) Middleware`.

- [ ] **Step 1: Viết test fail**

```go
func TestChainOrder(t *testing.T) {
	var order []string
	mk := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { order = append(order, name); next.ServeHTTP(w, r) })
		}
	}
	Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { order = append(order, "h") }), mk("a"), mk("b")).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if strings.Join(order, ",") != "a,b,h" { t.Fatal(order) }
}

func TestLoggingRecordsStatusAndUser(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestInfoFrom(r.Context()).User = "admin" // mô phỏng việc Auth ghi tên user
		w.WriteHeader(201); w.Write([]byte("hello"))
	})
	Logging(logger)(inner).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/employees?x=1", nil))
	line := buf.String()
	for _, want := range []string{"level=INFO", "msg=request", "method=POST", `url="/employees?x=1"`, "status=201", "bytes=5", "duration=", "user=admin"} {
		if !strings.Contains(line, want) { t.Errorf("missing %q in %s", want, line) }
	}
}

func TestLoggingThenRecoverLogs500(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }), Logging(logger), Recover(logger))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 500 || strings.TrimSpace(rec.Body.String()) != `{"error":"internal server error","code":500}` { t.Fatal(rec.Code, rec.Body) }
	if !strings.Contains(buf.String(), "status=500") || !strings.Contains(buf.String(), "level=ERROR") { t.Fatal(buf.String()) }
}

func TestRecoverAbortHandlerRepanics(t *testing.T) {
	defer func() { if v := recover(); v != http.ErrAbortHandler { t.Fatalf("got %v", v) } }()
	Recover(slog.New(slog.NewTextHandler(io.Discard, nil)))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
}

func TestRecoverAfterHeaderWritten(t *testing.T) {
	rec := httptest.NewRecorder()
	Recover(slog.New(slog.NewTextHandler(io.Discard, nil)))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(202); panic("late")
	})).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 202 || rec.Body.Len() != 0 { t.Fatal(rec.Code, rec.Body) }
}
```

- [ ] **Step 2: Chạy test để thấy nó fail**

Run: `go test ./internal/middleware/ -v`
Expected: FAIL, `undefined: Middleware`.

- [ ] **Step 3: Viết bốn file**

- **`statusRecorder` (dùng trong Logging):** status mặc định là 200 và cộng dồn số byte ở mỗi lần `Write`.
- **Recover:** bọc `w` bằng một wrapper riêng để biết header đã được ghi hay chưa.
- **Mức log:** Logging chọn `slog.LevelError` khi status ≥ 500, các trường hợp còn lại dùng `slog.LevelInfo`.

- [ ] **Step 4: Chạy lại test để thấy nó pass**

Run: `go test ./internal/middleware/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/middleware
git commit -m "feat: add middleware chain, request context, logging and recover"
```

---

### Task 12: Middleware Auth và Timeout

**Files:**
- Create: `internal/middleware/auth.go`, `timeout.go`
- Test: `internal/middleware/auth_test.go`, `timeout_test.go`

**Interfaces:**
- Consumes: `utils.WriteError`, `utils.Unauthorized`, và `context.go` từ Task 11.
- Produces:
  - `Auth(users map[string]string, skipPaths ...string) Middleware`.
  - `Timeout(d time.Duration) Middleware`.

- [ ] **Step 1: Viết test fail**

```go
func basic(s string) string { return "Basic " + base64.StdEncoding.EncodeToString([]byte(s)) }

func TestAuth(t *testing.T) {
	users := map[string]string{"admin": "admin123"}
	var seenUser string
	h := Auth(users, "/health")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seenUser, _ = UserFrom(r.Context()) }))
	cases := []struct{ name, path, header string; code int; user string }{
		{"no header", "/employees", "", 401, ""},
		{"wrong prefix", "/employees", "Bearer abc", 401, ""},
		{"bad base64", "/employees", "Basic !!!", 401, ""},
		{"no colon", "/employees", basic("admin"), 401, ""},
		{"unknown user", "/employees", basic("bob:admin123"), 401, ""},
		{"wrong password", "/employees", basic("admin:nope"), 401, ""},
		{"ok", "/employees", basic("admin:admin123"), 200, "admin"},
		{"skip health", "/health", "", 200, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seenUser = ""
			r := httptest.NewRequest("GET", tc.path, nil)
			if tc.header != "" { r.Header.Set("Authorization", tc.header) }
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			if rec.Code != tc.code || seenUser != tc.user { t.Fatalf("code %d user %q", rec.Code, seenUser) }
			if tc.code == 401 {
				if rec.Header().Get("WWW-Authenticate") != `Basic realm="ems"` ||
					strings.TrimSpace(rec.Body.String()) != `{"error":"unauthorized","code":401}` { t.Fatal(rec.Header(), rec.Body) }
			}
		})
	}
}

func TestTimeoutSetsDeadline(t *testing.T) {
	var deadline time.Time; var ok bool
	Timeout(2*time.Second)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { deadline, ok = r.Context().Deadline() })).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if !ok || time.Until(deadline) > 2*time.Second || time.Until(deadline) < time.Second { t.Fatal(deadline, ok) }
}
```

- [ ] **Step 2: Chạy test để thấy nó fail**

Run: `go test ./internal/middleware/ -run 'TestAuth|TestTimeout' -v`
Expected: FAIL, `undefined: Auth`.

- [ ] **Step 3: Viết `auth.go` và `timeout.go`**

Làm đúng theo spec §9. Trước khi gọi `utils.WriteError(w, utils.Unauthorized())`, đặt header `WWW-Authenticate`.

- [ ] **Step 4: Chạy lại test để thấy nó pass**

Run: `go test ./internal/middleware/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/middleware
git commit -m "feat: add basic auth and request timeout middleware"
```

---

### Task 13: Router (bắt route lạ và sai method), health, department handlers

**Files:**
- Create: `internal/handlers/routes.go`, `health_handler.go`, `department_handler.go`
- Test: `internal/handlers/helpers_test.go`, `routes_test.go`, `department_handler_test.go`

**Interfaces:**
- Consumes: services (Task 8–10), các hàm trong utils.
- Produces:
  - `Health(w http.ResponseWriter, r *http.Request) error`.
  - `NewDepartmentHandler(svc *services.DepartmentService) *DepartmentHandler`, với các method `Create`, `List`, `Employees`, đều có kiểu `utils.HandlerFunc`.
  - `NewRouter(emp *EmployeeHandler, dept *DepartmentHandler, exp *ExportHandler) http.Handler`. Task này đăng ký route health và department. Các route employee và export để trống, Task 14 sẽ thêm vào. Vẫn khai báo hai struct rỗng `EmployeeHandler` và `ExportHandler` để chữ ký hàm compile được.
  - Hàm nội bộ `fallback(mux *http.ServeMux) http.Handler`: làm theo spec §8.1.
  - Test helper `newTestRouter(t) (http.Handler, *fakerepo.Store)` và `do(t, h, method, path, body string) *httptest.ResponseRecorder`.

- [ ] **Step 1: Viết test fail**

```go
func TestRouterFallbacks(t *testing.T) {
	h, _ := newTestRouter(t)
	rec := do(t, h, "PATCH", "/departments", "")
	if rec.Code != 405 || rec.Header().Get("Allow") != "GET, POST" ||
		strings.TrimSpace(rec.Body.String()) != `{"error":"method not allowed","code":405}` { t.Fatal(rec.Code, rec.Header(), rec.Body) }
	for _, p := range []string{"/nope", "/departments/"} {
		rec = do(t, h, "GET", p, "")
		if rec.Code != 404 || strings.TrimSpace(rec.Body.String()) != `{"error":"route not found","code":404}` { t.Fatal(p, rec.Code, rec.Body) }
	}
	rec = do(t, h, "GET", "/health", "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` { t.Fatal(rec.Body) }
}

func TestDepartmentEndpoints(t *testing.T) {
	h, _ := newTestRouter(t)
	rec := do(t, h, "POST", "/departments", `{"name":"Engineering"}`)
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), `"name":"Engineering"`) { t.Fatal(rec.Code, rec.Body) }
	if rec = do(t, h, "POST", "/departments", `{"name":"engineering"}`); rec.Code != 409 { t.Fatal(rec.Code) }
	if rec = do(t, h, "POST", "/departments", `{"name":""}`); rec.Code != 400 || !strings.Contains(rec.Body.String(), `"details"`) { t.Fatal(rec.Body) }
	rec = do(t, h, "GET", "/departments", "")
	if rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), `{"departments":[`) { t.Fatal(rec.Body) }
	rec = do(t, h, "GET", "/departments/1/employees", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"employees":[]`) { t.Fatal(rec.Body) }
	if rec = do(t, h, "GET", "/departments/99/employees", ""); rec.Code != 404 { t.Fatal(rec.Code) }
	rec = do(t, h, "GET", "/departments/abc/employees", "")
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), `invalid department id: \"abc\"`) { t.Fatal(rec.Body) }
}
```

- [ ] **Step 2: Chạy test để thấy nó fail**

Run: `go test ./internal/handlers/ -v`
Expected: FAIL, `undefined: NewRouter`.

- [ ] **Step 3: Viết `routes.go`, `health_handler.go`, `department_handler.go`**

- `GET /departments` trả `map[string]any{"departments": list}`; `list` phải luôn là `[]`, không được là `null`.
- Handler fallback không đi qua `Handle`: nó tự gọi `utils.WriteJSON` (cho 405) hoặc `utils.WriteError(w, utils.NotFound("route not found"))` (cho 404).

- [ ] **Step 4: Chạy lại test để thấy nó pass**

Run: `go test ./internal/handlers/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/handlers
git commit -m "feat: add router with JSON 404/405 fallback, health and department handlers"
```

---

### Task 14: Employee handlers và export handler

**Files:**
- Create: `internal/handlers/employee_handler.go`, `export_handler.go`
- Modify: `internal/handlers/routes.go` (đăng ký route employee và export, bỏ hai struct tạm của Task 13)
- Test: `internal/handlers/employee_handler_test.go`, `export_handler_test.go`

**Interfaces:**
- Produces:
  - `NewEmployeeHandler(svc *services.EmployeeService) *EmployeeHandler`, với các method `Create`, `List`, `Search`, `Get`, `Update`, `Delete`.
  - `NewExportHandler(svc *services.ExportService) *ExportHandler`, với method `Export`.
  - Mọi method đều có kiểu `utils.HandlerFunc`.
  - Routes đăng ký đúng như bảng spec §8.1.

- [ ] **Step 1: Viết test fail**

```go
func TestEmployeeEndpoints(t *testing.T) {
	h, store := newTestRouter(t)
	d := &models.Department{Name: "Eng"}
	store.Departments().Create(context.Background(), d)
	body := fmt.Sprintf(`{"name":"Nguyen Van A","age":28,"position":"Backend Engineer","departmentId":%d,"salary":2000}`, d.ID)
	rec := do(t, h, "POST", "/employees", body)
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), `"departmentId":1`) { t.Fatal(rec.Code, rec.Body) }
	if rec = do(t, h, "POST", "/employees", `{"agee":1}`); rec.Code != 400 ||
		!strings.Contains(rec.Body.String(), `invalid request body: unknown field \"agee\"`) { t.Fatal(rec.Body) }
	if rec = do(t, h, "POST", "/employees", `{"name":"B","age":1,"position":"P","departmentId":99,"salary":1}`); rec.Code != 422 { t.Fatal(rec.Code) }
	rec = do(t, h, "GET", "/employees?limit=10&offset=0", "")
	if rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), `{"totalCount":1,"employees":[`) { t.Fatal(rec.Body) }
	if rec = do(t, h, "GET", "/employees?limit=101", ""); rec.Code != 400 { t.Fatal(rec.Code) }
	rec = do(t, h, "GET", "/employees/search?keyword=nguyen", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"totalCount":1`) { t.Fatal(rec.Body) }
	if rec = do(t, h, "GET", "/employees/search", ""); rec.Code != 400 { t.Fatal(rec.Code) } // không bị hiểu là {id}="search"
	if rec = do(t, h, "GET", "/employees/1", ""); rec.Code != 200 { t.Fatal(rec.Code) }
	rec = do(t, h, "GET", "/employees/abc", "")
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), `invalid employee id: \"abc\"`) { t.Fatal(rec.Body) }
	rec = do(t, h, "PUT", "/employees/1", `{"salary":3000}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"salary":3000`) { t.Fatal(rec.Body) }
	rec = do(t, h, "DELETE", "/employees/1", "")
	if rec.Code != 204 || rec.Body.Len() != 0 { t.Fatal(rec.Code, rec.Body) }
	rec = do(t, h, "GET", "/employees/1", "")
	if rec.Code != 404 || strings.TrimSpace(rec.Body.String()) != `{"error":"Employee not found","code":404}` { t.Fatal(rec.Body) }
	if rec = do(t, h, "PATCH", "/employees/1", ""); rec.Code != 405 || rec.Header().Get("Allow") != "DELETE, GET, PUT" { t.Fatal(rec.Header()) }
}

func TestEmployeeTimeoutFromRepo(t *testing.T) {
	h, store := newTestRouter(t)
	store.FailWith["Employee.List"] = utils.Timeout(nil)
	if rec := do(t, h, "GET", "/employees", ""); rec.Code != 504 { t.Fatal(rec.Code) }
}

func TestExportEndpoint(t *testing.T) {
	h, _ := newTestRouter(t) // newTestRouter dùng t.TempDir() làm thư mục export
	rec := do(t, h, "POST", "/employees/export", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"count":0`) || !strings.Contains(rec.Body.String(), `"csv":`) { t.Fatal(rec.Body) }
}
```

- [ ] **Step 2: Chạy test để thấy nó fail**

Run: `go test ./internal/handlers/ -run 'TestEmployee|TestExport' -v`
Expected: FAIL, `undefined: NewEmployeeHandler`.

- [ ] **Step 3: Viết hai handler và đăng ký route**

- `List` đọc `limit`/`offset` bằng `utils.ParsePagination`, đọc `departmentId` bằng `utils.ParseOptionalID(q, "departmentId")`.
- `Delete` gọi `w.WriteHeader(http.StatusNoContent)` và không ghi body.

- [ ] **Step 4: Chạy lại test để thấy nó pass**

Run: `go test ./internal/handlers/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/handlers
git commit -m "feat: add employee and export handlers"
```

---

### Task 15: `main.go`, test tích hợp và kiểm tra cuối

**Files:**
- Create: `cmd/app/main.go`, `cmd/app/main_test.go`, `internal/handlers/integration_test.go`

**Interfaces:**
- Consumes: mọi package ở các task trước.
- Produces:
  - `type config struct{ Port, DBPath, ExportDir string; RequestTimeout time.Duration }`.
  - `loadConfig(getenv func(string) string) (config, error)`: giá trị mặc định lấy theo spec §3.1.
  - `run(ctx context.Context, cfg config, logger *slog.Logger) error`: khởi tạo đầy đủ và tắt server êm theo spec §3.1.
  - `main()`: chỉ gọi `loadConfig(os.Getenv)`, tạo context bằng `signal.NotifyContext`, rồi gọi `run`. Có lỗi thì ghi log và `os.Exit(1)`.

- [ ] **Step 1: Viết test fail**

```go
// cmd/app/main_test.go
func TestLoadConfig(t *testing.T) {
	env := map[string]string{}
	get := func(k string) string { return env[k] }
	cfg, err := loadConfig(get)
	if err != nil || cfg.Port != "8080" || cfg.DBPath != "data/app.db" || cfg.ExportDir != "exports" || cfg.RequestTimeout != 5*time.Second { t.Fatal(cfg, err) }
	env["PORT"], env["REQUEST_TIMEOUT"] = "9090", "250ms"
	if cfg, _ = loadConfig(get); cfg.Port != "9090" || cfg.RequestTimeout != 250*time.Millisecond { t.Fatal(cfg) }
	env["REQUEST_TIMEOUT"] = "abc"
	if _, err = loadConfig(get); err == nil { t.Fatal("want error") }
}
```

```go
// internal/handlers/integration_test.go
func TestFullFlow(t *testing.T) {
	// Khởi tạo giống main.run: OpenSQLite(t.TempDir()) → repo SQLite → service → handler → NewRouter
	// → Chain(Logging(io.Discard logger), Recover, Auth({"admin":"admin123"}, "/health"), Timeout(5s)) → httptest.NewServer
	srv := newIntegrationServer(t)
	// call: tạo http.NewRequest tới srv.URL+path; khi auth=true thì gọi SetBasicAuth("admin", "admin123");
	// gửi bằng srv.Client(), rồi trả về status code và body dạng chuỗi.
	call := func(method, path, body string, auth bool) (int, string)
	if code, _ := call("GET", "/employees", "", false); code != 401 { t.Fatal(code) }
	if code, _ := call("GET", "/health", "", false); code != 200 { t.Fatal(code) }
	code, body := call("POST", "/departments", `{"name":"Engineering"}`, true)
	if code != 201 { t.Fatal(code, body) }
	code, body = call("POST", "/employees", `{"name":"Nguyễn Văn Ấn","age":28,"position":"Backend Engineer","departmentId":1,"salary":2000}`, true)
	if code != 201 { t.Fatal(code, body) }
	if code, body = call("GET", "/employees?limit=10&offset=0&departmentId=1", "", true); code != 200 || !strings.Contains(body, `"totalCount":1`) { t.Fatal(body) }
	if code, _ = call("PUT", "/employees/1", `{"position":"Lead"}`, true); code != 200 { t.Fatal(code) }
	if code, body = call("GET", "/employees/search?keyword=lead", "", true); !strings.Contains(body, `"totalCount":1`) { t.Fatal(body) }
	if code, body = call("GET", "/departments/1/employees", "", true); code != 200 || !strings.Contains(body, "Nguyễn Văn Ấn") { t.Fatal(body) }
	if code, body = call("POST", "/employees/export", "", true); code != 200 || !strings.Contains(body, `"count":1`) { t.Fatal(body) }
	if code, _ = call("DELETE", "/employees/1", "", true); code != 204 { t.Fatal(code) }
	if code, _ = call("GET", "/employees/1", "", true); code != 404 { t.Fatal(code) }
}
```

- [ ] **Step 2: Chạy test để thấy nó fail**

Run: `go test ./cmd/app/ ./internal/handlers/ -run 'TestLoadConfig|TestFullFlow' -v`
Expected: FAIL, `undefined: loadConfig` và `undefined: newIntegrationServer`.

- [ ] **Step 3: Viết `main.go` và helper `newIntegrationServer` trong `integration_test.go`**

`run` phải đóng các repository và DB theo thứ tự ngược với lúc mở. Cách làm: dùng `defer` ngay sau mỗi bước mở thành công.

- [ ] **Step 4: Chạy lại test để thấy nó pass**

Run: `go vet ./... && go test ./... -cover`
Expected: mọi package PASS. Coverage của `services`, `utils`, `middleware` nên ≥ 80%; đây chỉ là mục tiêu, nếu thấp hơn thì ghi lại để báo, không chặn task.

- [ ] **Step 5: Chạy race detector**

Run: `CGO_ENABLED=1 go test -race ./...`
Expected: PASS, không có dòng `WARNING: DATA RACE`.

Nếu báo `-race requires cgo` hoặc không tìm thấy `gcc`, nghĩa là người dùng chưa cài gcc qua MSYS2. Khi đó ghi lại tình trạng này để báo cho người dùng và không đánh dấu bước này là đã xong.

- [ ] **Step 6: Chạy thử server thật (smoke test)**

Run: `go run ./cmd/app`. Mở terminal khác và chạy `curl -s -u admin:admin123 localhost:8080/employees`.
Expected: `{"totalCount":0,"employees":[]}`, và log ở terminal chạy server có dòng `msg=request method=GET url=/employees status=200`. Bấm Ctrl+C để dừng; server phải tự thoát êm.

- [ ] **Step 7: Commit**

```bash
git add cmd internal/handlers
git commit -m "feat: wire application entrypoint and add end-to-end test"
```
