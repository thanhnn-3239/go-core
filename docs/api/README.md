# Employee Management System: tài liệu API

REST API quản lý nhân viên và phòng ban, viết bằng Go thuần (`net/http`) và lưu dữ liệu bằng SQLite.

File này chứa phần dùng chung cho mọi API. Chi tiết từng nhóm API nằm ở các file riêng:

| File | Nội dung |
|---|---|
| [health.md](health.md) | Kiểm tra server còn hoạt động |
| [departments.md](departments.md) | Phòng ban, mô hình `Department` |
| [employees.md](employees.md) | Nhân viên, mô hình `Employee` và `EmployeePage` |
| [export.md](export.md) | Xuất nhân viên ra file JSON và CSV |

## Mục lục

- [Thông tin chung](#thông-tin-chung)
- [Bảng tổng hợp endpoint](#bảng-tổng-hợp-endpoint)
- [Xác thực](#xác-thực)
- [Định dạng lỗi](#định-dạng-lỗi)
  - [Mã trạng thái](#mã-trạng-thái)
  - [Lỗi body JSON (400)](#lỗi-body-json-400)
  - [Route lạ và sai method](#route-lạ-và-sai-method)

---

## Thông tin chung

| Mục | Giá trị |
|---|---|
| Base URL | `http://localhost:8080`. Đổi cổng bằng biến môi trường `PORT`. |
| Định dạng | Request và response đều là JSON (`Content-Type: application/json; charset=utf-8`). |
| Kích thước body | Tối đa 1 MB. |
| Tên trường JSON | camelCase: `departmentId`, `totalCount`, `createdAt`... |
| Danh sách rỗng | Luôn trả `[]`, không bao giờ trả `null`. |
| Thời gian | Chuỗi RFC 3339, giờ UTC, ví dụ `2026-10-18T14:30:47.0800281Z`. |
| Thời gian xử lý tối đa | Mỗi request có `REQUEST_TIMEOUT` (mặc định 5 giây). Quá hạn khi đang truy vấn DB thì trả 504. |

## Bảng tổng hợp endpoint

| Method | Path | Mô tả | Thành công | Tài liệu |
|---|---|---|---|---|
| GET | `/health` | Kiểm tra server còn sống (không cần đăng nhập) | 200 | [health.md](health.md#get-health) |
| POST | `/departments` | Tạo phòng ban | 201 | [departments.md](departments.md#post-departments) |
| GET | `/departments` | Danh sách phòng ban | 200 | [departments.md](departments.md#get-departments) |
| GET | `/departments/{id}/employees` | Phòng ban kèm danh sách nhân viên | 200 | [departments.md](departments.md#get-departmentsidemployees) |
| POST | `/employees` | Tạo nhân viên | 201 | [employees.md](employees.md#post-employees) |
| GET | `/employees` | Danh sách nhân viên, có phân trang và lọc | 200 | [employees.md](employees.md#get-employees) |
| GET | `/employees/search` | Tìm nhân viên theo tên hoặc vị trí | 200 | [employees.md](employees.md#get-employeessearch) |
| GET | `/employees/{id}` | Chi tiết nhân viên | 200 | [employees.md](employees.md#get-employeesid) |
| PUT | `/employees/{id}` | Cập nhật một hoặc nhiều trường | 200 | [employees.md](employees.md#put-employeesid) |
| DELETE | `/employees/{id}` | Xoá mềm nhân viên | 204 | [employees.md](employees.md#delete-employeesid) |
| POST | `/employees/export` | Xuất toàn bộ nhân viên ra file JSON và CSV | 200 | [export.md](export.md#post-employeesexport) |

---

## Xác thực

Mọi endpoint **trừ** `GET /health` đều yêu cầu HTTP Basic Auth:

```
Authorization: Basic <base64("username:password")>
```

| Username | Password |
|---|---|
| `admin` | `admin123` |

Ví dụ với curl:

```bash
curl -u admin:admin123 http://localhost:8080/employees
```

Thiếu header, sai định dạng header, sai tên đăng nhập hoặc sai mật khẩu đều trả về **cùng một** phản hồi, để client không đoán được tài khoản nào tồn tại:

```http
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Basic realm="ems"
Content-Type: application/json; charset=utf-8

{"error":"unauthorized","code":401}
```

Server kiểm tra xác thực trước khi tìm route, nên gọi một đường dẫn không tồn tại mà chưa đăng nhập cũng nhận 401 chứ không phải 404.

---

## Định dạng lỗi

Mọi lỗi đều có dạng:

```json
{
  "error": "Employee not found",
  "code": 404
}
```

Riêng lỗi **giá trị trường không hợp lệ** có thêm mảng `details`, mỗi phần tử ứng với một trường:

```json
{
  "error": "validation failed",
  "code": 400,
  "details": [
    { "field": "name", "message": "is required" },
    { "field": "age", "message": "must be greater than 0" }
  ]
}
```

Trường `details` chỉ xuất hiện ở lỗi `validation failed`. Các trường trong `details` luôn theo thứ tự `name`, `age`, `position`, `departmentId`, `salary`.

### Mã trạng thái

| Mã | Khi nào |
|---|---|
| 400 | ID không hợp lệ, tham số query sai, body JSON sai hoặc thiếu trường, giá trị trường không hợp lệ |
| 401 | Chưa đăng nhập hoặc sai tài khoản |
| 404 | Không tìm thấy tài nguyên, hoặc route không tồn tại |
| 405 | Path có tồn tại nhưng sai method. Header `Allow` liệt kê các method hợp lệ. |
| 409 | Trùng dữ liệu (tên phòng ban đã tồn tại) |
| 422 | Vi phạm business rule (phòng ban được tham chiếu không tồn tại) |
| 500 | Lỗi DB hoặc lỗi nội bộ. Client chỉ nhận `"internal server error"`, chi tiết chỉ ghi vào log. |
| 504 | Hết thời gian xử lý request khi đang làm việc với DB |

### Lỗi body JSON (400)

Áp dụng cho mọi endpoint nhận body (`POST /departments`, `POST /employees`, `PUT /employees/{id}`). Khi gặp lỗi, server dừng ở lỗi đầu tiên và không trả `details`.

| Tình huống | `error` |
|---|---|
| Body rỗng | `invalid request body: body must not be empty` |
| Có trường không khai báo | `invalid request body: unknown field "agee"` |
| Sai kiểu dữ liệu | `invalid request body: field "age" must be a number` (hoặc `a string`, `a boolean`, `an object`, `an array`) |
| Số thập phân ở trường số nguyên (ví dụ `"salary": 2000.5`) | `invalid request body: field "salary" must be a number` |
| Body không phải object (ví dụ `[]`) | `invalid request body: body must be a JSON object` |
| Có dữ liệu thừa sau object JSON | `invalid request body: unexpected data after JSON object` |
| Body lớn hơn 1 MB | `invalid request body: body exceeds 1MB limit` |
| JSON sai cú pháp | `invalid request body: <mô tả lỗi cú pháp>`, ví dụ `invalid request body: unexpected EOF` |

### Route lạ và sai method

```http
GET /nope
→ 404 {"error":"route not found","code":404}

PATCH /employees/1
→ 405 {"error":"method not allowed","code":405}
  Allow: DELETE, GET, PUT
```
