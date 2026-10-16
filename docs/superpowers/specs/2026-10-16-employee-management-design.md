# Employee Management System: thiết kế

- **Ngày:** 2026-10-16
- **Trạng thái:** đã duyệt từng phần trong buổi brainstorming, chờ review bản spec
- **Trang review (HTML):** https://claude.ai/artifact/CJracD8H49G3pKgMfwVKkM

## 1. Mục tiêu

Đây là bài thực hành của khóa Golang thuần: xây một REST API quản lý nhân viên và phòng ban, chỉ dùng thư viện chuẩn của Go. Bài cần đáp ứng đủ các mục 1–5 của đề:

1. Kiến trúc phân tầng Handler → Service → Repository, có interface cho repository.
2. Các API CRUD nhân viên, phòng ban, tìm kiếm, và export JSON/CSV chạy đồng thời.
3. Lưu trữ bằng `database/sql`, dùng prepared statement và transaction.
4. Middleware Logging, Recover và Basic Auth.
5. Mọi lỗi trả về JSON theo một format thống nhất.

**Tiêu chí hoàn thành:**

- Mọi endpoint trong mục 8.1 chạy đúng như mô tả.
- `go vet ./...` và `go test ./...` đều pass.
- `CGO_ENABLED=1 go test -race ./...` pass, không có cảnh báo race.
- Không có logic nghiệp vụ trong handler.

## 2. Ràng buộc công nghệ

- **Go 1.27.** Dùng router `http.ServeMux` có pattern theo method, có từ Go 1.22.
- **Thư viện chuẩn:** `net/http`, `encoding/json`, `encoding/csv`, `database/sql`, `context`, `sync`, `log/slog`, `embed`, `crypto/subtle`, `crypto/sha256`, `encoding/base64`, `testing`, `net/http/httptest`.
- **Ngoại lệ duy nhất là driver `modernc.org/sqlite`.** Driver này viết thuần Go, không cần CGO.
  - `database/sql` không đi kèm driver nào, nên muốn dùng SQL thì bắt buộc phải có driver.
  - Không dùng framework hay ORM nào (Gin, GORM...).
- **Lưu ý:** đề ghi "PostgreSQL hoặc MySQL", còn ta dùng SQLite, tức là lệch đề. Cần xác nhận lại với giảng viên.
  - Repository được tách sau interface, nên nếu phải đổi DB thì chỉ cần viết thêm một implementation mới.

## 3. Cấu trúc thư mục

Module có tên là `go-core`.

```
cmd/app/main.go                 # đọc config, nối các thành phần, chạy server, tắt server êm
internal/
  models/
    employee.go                 # Employee, CreateEmployeeInput, UpdateEmployeeInput, ListFilter, EmployeePage
    department.go               # Department, DepartmentEmployees
  utils/
    apperror.go                 # Kind, AppError, FieldError, các hàm khởi tạo lỗi
    response.go                 # WriteJSON, WriteError, Handle, DecodeJSON, ErrorResponse
    params.go                   # ParseID, ParsePagination, ParseOptionalID
  repositories/
    repository.go               # interface EmployeeRepository, DepartmentRepository
    db.go                       # OpenSQLite (DSN + migrate), translateError
    schema.sql                  # nhúng bằng //go:embed
    employee_sqlite.go
    department_sqlite.go
    fakerepo/                   # implementation trong bộ nhớ, chỉ dùng cho test
  services/
    employee_service.go
    department_service.go
    export_service.go
  handlers/
    employee_handler.go
    department_handler.go
    export_handler.go
    health_handler.go
    routes.go                   # NewRouter, hàm route() với route dự phòng trả 405
    integration_test.go
  middleware/
    chain.go  logging.go  recover.go  auth.go  timeout.go  context.go
exports/                        # tạo lúc chạy
data/app.db                     # tạo lúc chạy
```

**Chiều phụ thuộc:**

- `handlers` → `services` → `repositories` (interface) ← các implementation SQLite.
- Mọi tầng đều được dùng `models` và `utils`.
- Handler gọi trực tiếp service thật, không có interface cho service.
- Test handler dùng service thật kết hợp với `fakerepo`.

### 3.1 Khởi tạo trong `main.go`

1. **Đọc biến môi trường.** Thiếu biến nào thì dùng giá trị mặc định:

   | Biến | Mặc định |
   |---|---|
   | `PORT` | `8080` |
   | `DB_PATH` | `data/app.db` |
   | `EXPORT_DIR` | `exports` |
   | `REQUEST_TIMEOUT` | `5s` |

   Giá trị không hợp lệ (ví dụ `REQUEST_TIMEOUT=abc`) thì báo lỗi rồi thoát với mã khác 0.
2. **Mở DB.** Gọi `repositories.OpenSQLite(ctx, path)`: tạo thư mục cha nếu chưa có, mở DB với DSN ở mục 6.2, rồi chạy `schema.sql`.
3. **Tạo các thành phần theo thứ tự** repository → service → handler, rồi gọi `handlers.NewRouter(...)`.
4. **Bọc router bằng middleware** theo đúng thứ tự `Chain(router, Logging, Recover, Auth(users, "/health"), Timeout(REQUEST_TIMEOUT))`.
5. **Tạo `http.Server`** với các timeout sau:

   | Thuộc tính | Giá trị |
   |---|---|
   | `ReadHeaderTimeout` | 5s |
   | `ReadTimeout` | 10s |
   | `WriteTimeout` | 15s |
   | `IdleTimeout` | 60s |

6. **Tắt server êm.** Dùng `signal.NotifyContext(ctx, os.Interrupt)` để bắt Ctrl+C, sau đó gọi `srv.Shutdown` với thời gian chờ tối đa 10s, rồi đóng các repository và DB.

User đăng nhập được khai báo cố định trong `main.go`: `map[string]string{"admin": "admin123"}`.

## 4. Mô hình xử lý lỗi

Mỗi tầng xử lý lỗi theo một cách:

- **Repository:** chuyển lỗi DB thành `*AppError` (bảng ở mục 7.3).
- **Service:** trả về `*AppError`.
- **Handler:** có dạng `func(w, r) error`. Adapter `utils.Handle` bọc handler và gọi `WriteError` khi handler trả về lỗi. `WriteError` là nơi duy nhất ánh xạ `Kind` sang mã HTTP.

```go
type Kind int
const (KindValidation Kind = iota + 1; KindNotFound; KindConflict; KindBusinessRule;
       KindUnauthorized; KindTimeout; KindInternal)

type FieldError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}

type AppError struct {
    Kind    Kind
    Message string       // thông điệp gửi cho client, viết bằng tiếng Anh
    Details []FieldError // chỉ dùng cho lỗi giá trị trường
    Err     error        // lỗi gốc, chỉ ghi log; Unwrap() trả về Err
}

type ErrorResponse struct {
    Error   string       `json:"error"`
    Code    int          `json:"code"`
    Details []FieldError `json:"details,omitempty"`
}
```

**Bảng ánh xạ `Kind` → HTTP:**

| Kind | HTTP |
|---|---|
| Validation | 400 |
| Unauthorized | 401 |
| NotFound | 404 |
| Conflict | 409 |
| BusinessRule | 422 |
| Internal | 500 |
| Timeout | 504 |

**`WriteError` hoạt động như sau:**

1. Dùng `errors.As` để lấy `*AppError` từ lỗi nhận được.
2. Nếu không lấy được:
   - lỗi khớp `errors.Is(err, context.DeadlineExceeded)` → coi là Timeout;
   - mọi lỗi khác → coi là Internal.
3. Nếu mã HTTP ≥ 500, ghi `Err` (lỗi gốc) vào log.
4. Lỗi Internal chỉ trả về cho client thông điệp `"internal server error"`.

**`WriteJSON`** đặt `Content-Type: application/json; charset=utf-8`, sau đó gọi `WriteHeader`, rồi encode body.

## 5. Model

```go
type Employee struct {
    ID           int64     `json:"id"`
    Name         string    `json:"name"`
    Age          int       `json:"age"`
    Position     string    `json:"position"`
    DepartmentID int64     `json:"departmentId"`
    Salary       int64     `json:"salary"`
    CreatedAt    time.Time `json:"createdAt"`
    UpdatedAt    time.Time `json:"updatedAt"`
}

type CreateEmployeeInput struct {
    Name string `json:"name"`; Age int `json:"age"`; Position string `json:"position"`
    DepartmentID int64 `json:"departmentId"`; Salary int64 `json:"salary"`
}

type UpdateEmployeeInput struct { // nil = giữ nguyên trường đó
    Name *string `json:"name"`; Age *int `json:"age"`; Position *string `json:"position"`
    DepartmentID *int64 `json:"departmentId"`; Salary *int64 `json:"salary"`
}

type ListFilter struct { Limit, Offset int; DepartmentID *int64 }

type EmployeePage struct {
    TotalCount int        `json:"totalCount"`
    Employees  []Employee `json:"employees"` // luôn là [], không bao giờ là null
}

type Department struct {
    ID int64 `json:"id"`; Name string `json:"name"`; CreatedAt time.Time `json:"createdAt"`
}

type DepartmentEmployees struct {
    Department
    Employees []Employee `json:"employees"`
}
```

- `salary` dùng kiểu số nguyên `int64`. Nếu client gửi số thập phân như `2000.5` thì giải mã JSON thất bại và trả 400.
- `deleted_at` chỉ có trong DB, không xuất hiện trong JSON trả về.

## 6. Lưu trữ SQLite

### 6.1 Schema (`schema.sql`, chạy mỗi lần khởi động)

```sql
CREATE TABLE IF NOT EXISTS departments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    created_at TEXT    NOT NULL
);
CREATE TABLE IF NOT EXISTS employees (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT    NOT NULL,
    age           INTEGER NOT NULL CHECK (age > 0),
    position      TEXT    NOT NULL,
    department_id INTEGER NOT NULL REFERENCES departments(id),
    salary        INTEGER NOT NULL CHECK (salary > 0),
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL,
    deleted_at    TEXT
);
CREATE INDEX IF NOT EXISTS idx_employees_department
    ON employees(department_id) WHERE deleted_at IS NULL;
```

- **Thời gian:** lưu dạng TEXT theo định dạng `time.RFC3339Nano`, giờ UTC. Code Go tự format khi ghi và tự parse khi đọc.
- **Gán thời gian:** repository tự gán `created_at` và `updated_at` bằng `time.Now().UTC()`.
- **Ràng buộc `CHECK` và `REFERENCES`:** đóng vai trò lớp bảo vệ cuối cùng. Service đã kiểm tra dữ liệu trước khi gọi repository.

### 6.2 Kết nối

DSN dùng để mở DB:

```
file:<DB_PATH>?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)
```

- **Đặt PRAGMA trong DSN:** khi đặt ở đây, PRAGMA được áp dụng cho mọi kết nối trong pool.
- **`foreign_keys(1)`:** SQLite mặc định tắt khoá ngoại, nên phải bật thì `REFERENCES` mới có tác dụng.
- **WAL:** cho phép đọc song song trong lúc đang ghi.
- **`busy_timeout(5000)`:** khi DB đang bận, lệnh ghi chờ tối đa 5s thay vì báo lỗi `database is locked` ngay.

## 7. Repository

### 7.1 Interface

```go
type EmployeeRepository interface {
    Create(ctx context.Context, e *models.Employee) error // gán ID, CreatedAt, UpdatedAt vào e
    GetByID(ctx context.Context, id int64) (*models.Employee, error)
    List(ctx context.Context, f models.ListFilter) (models.EmployeePage, error)
    Search(ctx context.Context, keyword string) ([]models.Employee, error)
    Update(ctx context.Context, id int64, in models.UpdateEmployeeInput) (*models.Employee, error)
    SoftDelete(ctx context.Context, id int64) error
    ListAll(ctx context.Context) ([]models.Employee, error)
}

type DepartmentRepository interface {
    Create(ctx context.Context, d *models.Department) error
    List(ctx context.Context) ([]models.Department, error)
    Exists(ctx context.Context, id int64) (bool, error)
    GetWithEmployees(ctx context.Context, id int64) (*models.DepartmentEmployees, error)
}
```

Mọi phương thức đều bỏ qua nhân viên đã soft-delete (điều kiện `deleted_at IS NULL`).

### 7.2 Cách đáp ứng các yêu cầu SQL của đề

- **Prepared statement:**
  - `NewEmployeeSQLite(ctx, db)` và `NewDepartmentSQLite(ctx, db)` gọi `PrepareContext` cho mọi câu lệnh một lần lúc khởi tạo. Chúng có thêm `Close() error` để giải phóng các statement này.
  - Bộ lọc tuỳ chọn được viết dạng `(? IS NULL OR department_id = ?)`. Nhờ vậy câu lệnh List vẫn cố định và prepare trước được.
- **Transaction:**
  - **Update:** trong một transaction, chạy `SELECT` bản ghi (không có hoặc đã xoá thì trả NotFound) → gán các trường khác nil → `UPDATE` toàn bộ bản ghi kèm `updated_at` mới → `COMMIT`. Statement đã prepare được dùng lại trong transaction qua `tx.StmtContext`.
  - **List:** chạy `COUNT(*)` và `SELECT … ORDER BY id LIMIT ? OFFSET ?` trong cùng một transaction chỉ đọc, để `totalCount` khớp với dữ liệu của trang.
- **JOIN (`GetWithEmployees`):**
  - Câu truy vấn: `FROM departments d LEFT JOIN employees e ON e.department_id = d.id AND e.deleted_at IS NULL WHERE d.id = ? ORDER BY e.id`.
  - Không có dòng nào → NotFound.
  - Có một dòng nhưng `e.id` là NULL → phòng ban tồn tại nhưng không có nhân viên, trả `employees: []`.
- **Search:**
  - Câu truy vấn: `name LIKE ? ESCAPE '\' OR position LIKE ? ESCAPE '\'`, sắp xếp theo `id`.
  - Từ khoá được bọc thành `%keyword%`. Các ký tự `\`, `%`, `_` trong từ khoá được escape để không bị hiểu là ký tự đại diện.
  - Không phân biệt hoa thường với chữ ASCII. Với chữ có dấu (ví dụ "ăn" và "Ăn") thì vẫn phân biệt; đây là giới hạn đã chấp nhận.
- **SoftDelete:**
  - Câu lệnh: `UPDATE employees SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`.
  - Nếu `RowsAffected() == 0` thì trả NotFound.

### 7.3 Chuyển lỗi DB (`translateError`)

| Lỗi từ DB | Kind | Thông điệp |
|---|---|---|
| `sql.ErrNoRows` | NotFound | `Employee not found` / `Department not found` |
| `context.DeadlineExceeded` | Timeout | `request timed out` |
| `SQLITE_CONSTRAINT_UNIQUE` | Conflict | `department "<name>" already exists` |
| `SQLITE_CONSTRAINT_FOREIGNKEY` | BusinessRule | `department <id> does not exist` |
| lỗi khác | Internal | `internal server error` |

Mã lỗi được đọc từ `*sqlite.Error` của driver, không dò bằng cách so chuỗi thông báo lỗi.

Thông điệp phụ thuộc vào ngữ cảnh: tên entity cho NotFound, tên phòng ban cho Conflict, id phòng ban cho FK. Vì vậy mỗi phương thức repository tự gọi `translateError(err, entity, detail)` với ngữ cảnh của mình.

## 8. API

### 8.1 Route

| Method | Path | Thành công | Lỗi riêng |
|---|---|---|---|
| GET | `/health` | 200 `{"status":"ok"}` (không cần auth) | |
| POST | `/employees` | 201 + Employee | 400, 422 |
| GET | `/employees?limit&offset&departmentId` | 200 `EmployeePage` | 400 |
| GET | `/employees/search?keyword=` | 200 `EmployeePage` (`totalCount` = số kết quả) | 400 |
| GET | `/employees/{id}` | 200 + Employee | 400, 404 |
| PUT | `/employees/{id}` | 200 + Employee sau khi cập nhật | 400, 422, 404 |
| DELETE | `/employees/{id}` | 204, không có body | 400, 404 |
| POST | `/employees/export` | 200 `{"count":N,"files":{"json":"…","csv":"…"}}` | 500 |
| POST | `/departments` | 201 + Department | 400, 409 |
| GET | `/departments` | 200 `{"departments":[…]}` | |
| GET | `/departments/{id}/employees` | 200 `DepartmentEmployees` | 400, 404 |

Ngoài các lỗi riêng ở trên, mọi route trừ `/health` còn có thể trả 401, 500 và 504.

**Route lạ và sai method cũng trả lỗi JSON:**

- **Không đăng ký route dự phòng riêng cho từng path.** Nếu làm vậy, ServeMux sẽ panic khi khởi động. Ví dụ `/employees/search` (không kèm method) và `GET /employees/{id}` cùng khớp được một số request, nhưng không pattern nào cụ thể hơn pattern nào, nên mux coi là xung đột.
- **Dùng một route `/` duy nhất để bắt mọi thứ còn lại.** Handler này nhân bản request, đổi method lần lượt thành `GET`, `POST`, `PUT`, `DELETE`, rồi hỏi `mux.Handler(clone)` xem với method đó thì có route nào khớp (pattern khác `/`) không.
  - Có ít nhất một method khớp → trả 405 `{"error":"method not allowed","code":405}`, kèm header `Allow` liệt kê các method đó (sắp xếp theo bảng chữ cái, ngăn bằng `, `).
  - Không có method nào khớp → trả 404 `{"error":"route not found","code":404}`.

### 8.2 Tham số

| Tham số | Hợp lệ | Lỗi 400 |
|---|---|---|
| `{id}` | số nguyên ≥ 1 | `invalid employee id: "abc"` / `invalid department id: "abc"` |
| `limit` | 1–100, bỏ trống thì mặc định 10 | `limit must be between 1 and 100` |
| `offset` | ≥ 0, bỏ trống thì mặc định 0 | `offset must be a non-negative integer` |
| `departmentId` (lọc) | số nguyên ≥ 1, bỏ trống thì không lọc | `invalid departmentId: "x"` |
| `keyword` | không rỗng sau khi cắt khoảng trắng | `keyword is required` |

Lọc theo `departmentId` không tồn tại thì không báo lỗi, mà trả `totalCount: 0` và `employees: []`.

### 8.3 Đọc body JSON (`utils.DecodeJSON`)

- **Giới hạn kích thước:** body tối đa 1 MB, áp dụng bằng `http.MaxBytesReader`.
- **Trường lạ:** dùng `DisallowUnknownFields`, nên body có trường không khai báo sẽ bị từ chối.
- **Dữ liệu thừa:** sau khi đọc xong object JSON, gọi `dec.Decode(&struct{}{})` thêm một lần. Lần gọi này phải trả về `io.EOF`, nếu không thì body có dữ liệu thừa.
- **Khi có lỗi:** trả 400 `invalid request body: <mô tả>`, không kèm `details`. `json.Decoder` dừng ở lỗi đầu tiên nên chỉ báo được một lỗi.

### 8.4 Kiểm tra dữ liệu (ở service)

Các trường bị lỗi được gom vào `details` theo thứ tự cố định `name, age, position, departmentId, salary`. Khi có lỗi, service trả 400 `"validation failed"` kèm `details`.

| Trường | Điều kiện | `message` |
|---|---|---|
| `name` | không rỗng sau khi cắt khoảng trắng | `is required` |
| `age` | > 0 | `must be greater than 0` |
| `position` | không rỗng sau khi cắt khoảng trắng | `is required` |
| `departmentId` | > 0 | `must be a positive integer` |
| `salary` | > 0 | `must be greater than 0` |

Ví dụ một phản hồi lỗi kiểm tra dữ liệu:

```json
{"error":"validation failed","code":400,
 "details":[{"field":"name","message":"is required"},{"field":"age","message":"must be greater than 0"}]}
```

- **Create:** kiểm tra mọi trường.
- **Update:**
  - Chỉ kiểm tra các trường được gửi lên.
  - Body `{}` trả 400 `no fields to update`, không kèm `details`.
  - `name` và `position` được cắt khoảng trắng trước khi kiểm tra và lưu.
- **Department create:** `name` được cắt khoảng trắng. Tên rỗng trả 400 `validation failed` với `details` là `[{"field":"name","message":"is required"}]`.

### 8.5 Business rule và thứ tự kiểm tra

**Các business rule:**

- `departmentId` khi tạo hoặc cập nhật nhân viên phải tồn tại. Service gọi `deptRepo.Exists` để kiểm tra; nếu không tồn tại thì trả 422 `department <id> does not exist`.
- Tên phòng ban không được trùng, không phân biệt hoa thường. Vi phạm thì trả 409.

**Thứ tự kiểm tra áp dụng cho mọi endpoint:**

1. Dữ liệu request: path, query, body → lỗi 400.
2. Business rule → lỗi 422 hoặc 409.
3. Thực thi thao tác → lỗi 404, 500 hoặc 504.

Ví dụ: `PUT /employees/999` với `departmentId` không tồn tại sẽ trả 422, không phải 404.

### 8.6 Lỗi theo 5 trường hợp của đề

| Trường hợp | HTTP | Ví dụ |
|---|---|---|
| ID không hợp lệ | 400 | `{"error":"invalid employee id: \"abc\"","code":400}` |
| Request sai (JSON) | 400 | `{"error":"invalid request body: unknown field \"agee\"","code":400}` |
| Request sai (giá trị) | 400 | `{"error":"validation failed","code":400,"details":[…]}` |
| Không tìm thấy | 404 | `{"error":"Employee not found","code":404}` |
| Lỗi DB | 500 | `{"error":"internal server error","code":500}` |
| DB timeout | 504 | `{"error":"request timed out","code":504}` |
| Business rule | 422 | `{"error":"department 99 does not exist","code":422}` |
| Trùng dữ liệu | 409 | `{"error":"department \"Engineering\" already exists","code":409}` |

## 9. Middleware

Thứ tự từ ngoài vào trong: **Logging → Recover → Auth → Timeout.**

Logging nằm ngoài Recover để vẫn ghi log đúng mã 500 khi handler panic.

```go
type Middleware func(http.Handler) http.Handler
func Chain(h http.Handler, mws ...Middleware) http.Handler // Chain(h, A, B) == A(B(h))
```

- **Logging:**
  - Bọc `ResponseWriter` bằng `statusRecorder` để ghi lại status (mặc định 200) và số byte đã ghi.
  - Tạo một `*requestInfo` và đặt vào context. Auth sẽ ghi tên user vào đó.
  - Sau khi handler chạy xong, ghi log bằng `slog`. Log level là INFO khi status < 500 và ERROR khi status ≥ 500:
    `msg=request method=GET url="/employees?limit=10" status=200 bytes=812 duration=1.84ms user=admin`.
- **Recover:**
  - Dùng `defer` kết hợp `recover()`.
  - Nếu giá trị panic là `http.ErrAbortHandler` thì panic lại, theo quy ước của `net/http`.
  - Các panic khác: ghi log giá trị panic cùng `debug.Stack()`. Nếu header chưa được ghi thì gọi `WriteError(Internal)`, client nhận 500 JSON.
- **Auth:**
  - Path nằm trong danh sách bỏ qua (`/health`) thì cho đi tiếp, không kiểm tra.
  - Các path khác kiểm tra theo trình tự:
    1. Lấy header `Authorization` và cắt tiền tố bằng `strings.CutPrefix(h, "Basic ")`.
    2. Giải mã bằng `base64.StdEncoding.DecodeString`.
    3. Tách user và password bằng `strings.Cut(s, ":")`.
    4. Tìm user trong map.
    5. So password bằng `subtle.ConstantTimeCompare(sha256(got), sha256(want))`, để thời gian so sánh không phụ thuộc vào nội dung password.
  - Không dùng `r.BasicAuth()`.
  - Mọi trường hợp thất bại đều trả cùng một phản hồi: 401, header `WWW-Authenticate: Basic realm="ems"` và body `{"error":"unauthorized","code":401}`.
  - Khi thành công: ghi tên user vào `requestInfo` và vào context bằng `WithUser`. Handler đọc lại bằng `UserFrom(ctx)`.
- **Timeout:**
  - `context.WithTimeout(r.Context(), d)` rồi gọi `next.ServeHTTP(w, r.WithContext(ctx))`.
  - Không dùng `http.TimeoutHandler`.
  - Deadline đi theo context xuống `QueryContext`/`ExecContext`. Khi hết giờ, client nhận 504 JSON.

## 10. Export đồng thời (`ExportService`)

`ExportService` giữ các trường sau:

- `repo` (`EmployeeRepository`): nguồn dữ liệu.
- `dir` (`EXPORT_DIR`): thư mục ghi file.
- `runMu sync.Mutex`: khoá để mỗi lúc chỉ có một lần export chạy.

`Export(ctx)` trả về `(ExportResult, error)` và chạy theo các bước:

1. `runMu.Lock()`. Nếu có request export khác đang chạy, request này chờ đến khi request kia xong.
2. Gọi `repo.ListAll(ctx)` một lần. Slice kết quả được cả hai goroutine dùng chung nhưng chỉ đọc.
3. `os.MkdirAll(dir)` để chắc chắn thư mục tồn tại.
4. `wg.Add(2)`, rồi chạy song song `writeJSON` và `writeCSV`, mỗi tác vụ trong một goroutine riêng. Khi xong, mỗi goroutine gọi `record(kind, path, err)`. Hàm `record` khoá `mu` trước khi ghi vào `res.Files` (map) và `errs` (slice), vì hai goroutine có thể ghi cùng lúc.
5. `wg.Wait()`. Nếu `errs` có lỗi thì trả `Internal(errors.Join(errs...))`.

**Chi tiết ghi file:**

- **Ghi an toàn:** mỗi goroutine ghi ra `<file>.tmp` trước. Ghi xong mới đổi tên bằng `os.Rename` sang `employees.json` hoặc `employees.csv`. Nếu có lỗi hoặc context bị huỷ thì xoá file `.tmp`. Hai goroutine kiểm tra `ctx.Err()` sau mỗi 500 dòng.
- **File JSON:** dùng `json.Encoder` với `SetIndent("", "  ")`, ghi ra một mảng `Employee`.
- **File CSV:**
  - Đầu file là BOM UTF-8 (`\xEF\xBB\xBF`).
  - Dòng đầu tiên là header `id,name,age,position,departmentId,salary,createdAt,updatedAt`.
  - Thời gian ghi theo định dạng RFC3339.
  - Ghi bằng `encoding/csv`.
- **Kết quả trả về:** `ExportResult{Count int; Files map[string]string}`, với khoá `json` và `csv`.

## 11. Test

**Nguyên tắc chung:**

- Chỉ dùng `testing` và `httptest`.
- Viết test dạng bảng (table-driven) với `t.Run`. Các hàm hỗ trợ gọi `t.Helper()`.
- Áp dụng TDD khi triển khai.

**Repository giả `fakerepo`:**

- Lưu dữ liệu trong bộ nhớ, có `sync.Mutex` bảo vệ.
- Có trường `FailWith map[string]error` (khoá là tên method) để giả lập lỗi khi cần.

**Nội dung cần test theo từng tầng:**

| Tầng | Nội dung chính |
|---|---|
| `utils` | `ParseID`, `ParsePagination`, `DecodeJSON` (trường lạ, dữ liệu thừa, quá 1 MB), `WriteError` (mỗi Kind, `details` omitempty, lỗi lạ → 500, `DeadlineExceeded` → 504) |
| `services` | thứ tự và nội dung `details`, 422 khi phòng ban không tồn tại, PUT kiểm tra từng phần, `{}` → 400, `Kind` của repository được giữ nguyên, cắt khoảng trắng; Export: tạo 2 file và đọc lại để kiểm tra, một goroutine lỗi → 500 và không còn `.tmp`, gọi `Export` song song 10 lần |
| `repositories` | SQLite thật trong `t.TempDir()`: CRUD, soft-delete bị ẩn khỏi mọi phương thức đọc, xoá hai lần → NotFound, `totalCount` có và không có bộ lọc, thứ tự giữa các trang ổn định, tìm kiếm có `%`/`_`, LEFT JOIN (phòng ban rỗng / không tồn tại), Conflict không phân biệt hoa thường, FK → BusinessRule, Update giữ các trường không gửi, ctx hết hạn → Timeout |
| `handlers` | mã và body của từng route, 405 JSON + `Allow`, 404 route lạ, `/employees/search` không bị hiểu là `{id}`, DELETE 204 không có body |
| `middleware` | Logging (`slog` ghi vào `bytes.Buffer`), Recover (panic → 500, `ErrAbortHandler`, header đã ghi), Auth (không có header, sai tiền tố, base64 lỗi, thiếu `:`, sai user, sai password, đúng, bỏ qua `/health`), Timeout (có deadline), thứ tự của `Chain` |
| Tích hợp | `httptest.NewServer` + SQLite thật + đủ chuỗi middleware: tạo phòng ban → tạo nhân viên → list → update → search → export → delete → GET 404 |

**Lệnh chạy:**

- `go vet ./...`
- `go test ./...`
- `go test -cover ./...`
- `CGO_ENABLED=1 go test -race ./...`

**Coverage:** mục tiêu ≥ 80% cho `services`, `utils` và `middleware`. Đây chỉ là mục tiêu, không phải điều kiện để test pass.

**Race detector:**

- `go test -race` cần CGO và trình biên dịch C.
- Máy hiện tại có `CGO_ENABLED=0` và chưa có gcc. Người dùng sẽ tự cài gcc qua MSYS2 bằng lệnh `pacman -S mingw-w64-ucrt-x86_64-gcc`.
- App vẫn build với `CGO_ENABLED=0`, vì `modernc.org/sqlite` viết thuần Go.

## 12. Ngoài phạm vi

- Phân quyền theo vai trò.
- Cập nhật và xoá phòng ban.
- Phân trang cho kết quả tìm kiếm.
- Tìm kiếm không phân biệt hoa thường với chữ tiếng Việt có dấu.
- ID dạng UUID.
- Migration có đánh version.
- File cấu hình (chỉ dùng biến môi trường).
- Rate limiting và HTTPS.

## 13. Rủi ro và lưu ý

- **SQLite lệch đề.** Đề ghi PostgreSQL/MySQL. Cần hỏi giảng viên. Phương án dự phòng là viết thêm repository Postgres dùng driver `pgx` mà không phải đổi service.
- **Race detector chưa chạy được** cho đến khi cài gcc (xem mục 11).
- **Ghi đồng thời trên SQLite.** Tại một thời điểm chỉ có một tiến trình được ghi. `busy_timeout(5000)` giúp các lệnh ghi chờ lần lượt thay vì báo lỗi ngay. Với tải của bài thực hành thì như vậy là đủ.
