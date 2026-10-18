# Nhân viên

[← Về tài liệu chung](README.md)

Mọi endpoint trong file này đều yêu cầu [Basic Auth](README.md#xác-thực). Lỗi 401, 500 và 504 dùng chung cho mọi API nên không nhắc lại ở từng endpoint, xem [Định dạng lỗi](README.md#định-dạng-lỗi).

## Mục lục

- [Mô hình Employee](#mô-hình-employee)
- [Mô hình EmployeePage](#mô-hình-employeepage)
- [POST /employees](#post-employees)
- [GET /employees](#get-employees)
- [GET /employees/search](#get-employeessearch)
- [GET /employees/{id}](#get-employeesid)
- [PUT /employees/{id}](#put-employeesid)
- [DELETE /employees/{id}](#delete-employeesid)

---

## Mô hình Employee

| Trường | Kiểu | Mô tả |
|---|---|---|
| `id` | integer | ID tự tăng |
| `name` | string | Họ tên. Khoảng trắng hai đầu bị cắt khi lưu. |
| `age` | integer | Tuổi, > 0 |
| `position` | string | Vị trí công việc. Khoảng trắng hai đầu bị cắt khi lưu. |
| `departmentId` | integer | ID phòng ban, phải tồn tại |
| `salary` | integer | Lương, số nguyên > 0 (không nhận số thập phân) |
| `createdAt` | string (RFC 3339) | Thời điểm tạo |
| `updatedAt` | string (RFC 3339) | Thời điểm cập nhật gần nhất |

```json
{
  "id": 1,
  "name": "Nguyễn Văn Ấn",
  "age": 28,
  "position": "Backend Engineer",
  "departmentId": 1,
  "salary": 2000,
  "createdAt": "2026-10-18T14:30:47.0800281Z",
  "updatedAt": "2026-10-18T14:30:47.0800281Z"
}
```

Nhân viên đã bị xoá (xoá mềm) không xuất hiện ở bất kỳ API đọc nào.

## Mô hình EmployeePage

Dùng cho [GET /employees](#get-employees) và [GET /employees/search](#get-employeessearch).

| Trường | Kiểu | Mô tả |
|---|---|---|
| `totalCount` | integer | Tổng số nhân viên khớp điều kiện, không tính phân trang |
| `employees` | Employee[] | Các nhân viên của trang hiện tại |

---

## POST /employees

Tạo nhân viên mới.

**Request body**

| Trường | Kiểu | Bắt buộc | Ràng buộc | `message` khi sai |
|---|---|---|---|---|
| `name` | string | Có | Không rỗng sau khi cắt khoảng trắng | `is required` |
| `age` | integer | Có | > 0 | `must be greater than 0` |
| `position` | string | Có | Không rỗng sau khi cắt khoảng trắng | `is required` |
| `departmentId` | integer | Có | > 0, và phòng ban phải tồn tại | `must be a positive integer` |
| `salary` | integer | Có | > 0 | `must be greater than 0` |

Thiếu một trường thì trường đó mang giá trị rỗng hoặc 0, nên vẫn bị báo lỗi bằng thông điệp tương ứng trong `details`.

```bash
curl -u admin:admin123 -X POST http://localhost:8080/employees \
  -H "Content-Type: application/json" \
  -d '{"name":"Nguyen Van A","age":28,"position":"Backend Engineer","departmentId":1,"salary":2000}'
```

**Response 201**: [Employee](#mô-hình-employee) vừa tạo.

```json
{
  "id": 1,
  "name": "Nguyen Van A",
  "age": 28,
  "position": "Backend Engineer",
  "departmentId": 1,
  "salary": 2000,
  "createdAt": "2026-10-18T14:30:47.0800281Z",
  "updatedAt": "2026-10-18T14:30:47.0800281Z"
}
```

**Lỗi**

| Mã | `error` | Khi nào |
|---|---|---|
| 400 | `validation failed` + `details` | Một hoặc nhiều trường không hợp lệ. Mọi trường sai đều được liệt kê. |
| 400 | `invalid request body: ...` | Body JSON sai, xem [Lỗi body JSON](README.md#lỗi-body-json-400) |
| 422 | `department 99 does not exist` | `departmentId` hợp lệ về giá trị nhưng phòng ban không tồn tại |

Ví dụ khi sai cả năm trường:

```json
{
  "error": "validation failed",
  "code": 400,
  "details": [
    { "field": "name", "message": "is required" },
    { "field": "age", "message": "must be greater than 0" },
    { "field": "position", "message": "is required" },
    { "field": "departmentId", "message": "must be a positive integer" },
    { "field": "salary", "message": "must be greater than 0" }
  ]
}
```

---

## GET /employees

Lấy danh sách nhân viên có phân trang, sắp xếp theo `id` tăng dần.

**Query parameters**

| Tham số | Kiểu | Mặc định | Ràng buộc | Lỗi 400 |
|---|---|---|---|---|
| `limit` | integer | `10` | 1 – 100 | `limit must be between 1 and 100` |
| `offset` | integer | `0` | ≥ 0 | `offset must be a non-negative integer` |
| `departmentId` | integer | (không lọc) | ≥ 1 | `invalid departmentId: "x"` |

```bash
curl -u admin:admin123 "http://localhost:8080/employees?limit=2&offset=0&departmentId=1"
```

**Response 200**: [EmployeePage](#mô-hình-employeepage).

```json
{
  "totalCount": 3,
  "employees": [
    { "id": 1, "name": "Nguyễn Văn Ấn", "age": 28, "position": "Backend Engineer", "departmentId": 1, "salary": 2000, "createdAt": "2026-10-18T14:30:47.0800281Z", "updatedAt": "2026-10-18T14:30:47.0800281Z" },
    { "id": 2, "name": "Tran Thi B", "age": 31, "position": "QA Engineer", "departmentId": 1, "salary": 1800, "createdAt": "2026-10-18T14:30:47.2329938Z", "updatedAt": "2026-10-18T14:30:47.2329938Z" }
  ]
}
```

- `totalCount` là tổng số bản ghi khớp bộ lọc, không phụ thuộc `limit` và `offset`.
- `offset` vượt quá tổng số bản ghi thì vẫn trả 200, với `employees: []` và `totalCount` là số thật.
- Lọc theo `departmentId` không tồn tại thì **không** báo lỗi, mà trả `{"totalCount":0,"employees":[]}`.

---

## GET /employees/search

Tìm nhân viên có `name` **hoặc** `position` chứa từ khoá. Kết quả sắp xếp theo `id` và không phân trang.

**Query parameters**

| Tham số | Kiểu | Bắt buộc | Ràng buộc |
|---|---|---|---|
| `keyword` | string | Có | Không rỗng sau khi cắt khoảng trắng |

- Không phân biệt hoa thường với chữ không dấu: `an` khớp `An`. Với chữ có dấu thì vẫn phân biệt: `ăn` không khớp `Ăn`.
- Ký tự `%`, `_`, `\` trong từ khoá được tìm đúng nguyên văn, không bị hiểu là ký tự đại diện.

```bash
curl -u admin:admin123 "http://localhost:8080/employees/search?keyword=engineer"
```

**Response 200**: [EmployeePage](#mô-hình-employeepage), với `totalCount` là số kết quả tìm được.

```json
{
  "totalCount": 2,
  "employees": [
    { "id": 1, "name": "Nguyễn Văn Ấn", "age": 28, "position": "Backend Engineer", "departmentId": 1, "salary": 2000, "createdAt": "2026-10-18T14:30:47.0800281Z", "updatedAt": "2026-10-18T14:30:47.0800281Z" },
    { "id": 2, "name": "Tran Thi B", "age": 31, "position": "QA Engineer", "departmentId": 1, "salary": 1800, "createdAt": "2026-10-18T14:30:47.2329938Z", "updatedAt": "2026-10-18T14:30:47.2329938Z" }
  ]
}
```

**Lỗi**

| Mã | `error` | Khi nào |
|---|---|---|
| 400 | `keyword is required` | Thiếu `keyword` hoặc chỉ có khoảng trắng |

---

## GET /employees/{id}

Lấy chi tiết một nhân viên.

**Path parameter**

| Tham số | Kiểu | Ràng buộc |
|---|---|---|
| `id` | integer | Số nguyên ≥ 1 |

```bash
curl -u admin:admin123 http://localhost:8080/employees/1
```

**Response 200**: [Employee](#mô-hình-employee).

**Lỗi**

| Mã | `error` | Khi nào |
|---|---|---|
| 400 | `invalid employee id: "abc"` | `id` không phải số nguyên ≥ 1 (ví dụ `abc`, `0`, `-1`, `1.5`) |
| 404 | `Employee not found` | Không có nhân viên này, hoặc nhân viên đã bị xoá |

---

## PUT /employees/{id}

Cập nhật **một hoặc nhiều** trường của nhân viên. Trường nào có trong body thì được đổi; trường không gửi, hoặc gửi giá trị `null`, thì giữ nguyên.

**Path parameter**: `id`, giống [GET /employees/{id}](#get-employeesid).

**Request body**: các trường giống [POST /employees](#post-employees), nhưng trường nào cũng không bắt buộc. Ràng buộc chỉ kiểm tra trên các trường được gửi.

```bash
curl -u admin:admin123 -X PUT http://localhost:8080/employees/1 \
  -H "Content-Type: application/json" \
  -d '{"salary":3000}'
```

**Response 200**: [Employee](#mô-hình-employee) sau khi cập nhật, với `updatedAt` mới.

```json
{
  "id": 1,
  "name": "Nguyễn Văn Ấn",
  "age": 28,
  "position": "Backend Engineer",
  "departmentId": 1,
  "salary": 3000,
  "createdAt": "2026-10-18T14:30:47.0800281Z",
  "updatedAt": "2026-10-18T14:30:51.0479744Z"
}
```

**Lỗi**: server kiểm tra theo thứ tự 400 → 422 → 404. Ví dụ cập nhật nhân viên không tồn tại sang một phòng ban cũng không tồn tại sẽ nhận 422 trước.

| Mã | `error` | Khi nào |
|---|---|---|
| 400 | `invalid employee id: "abc"` | `id` không hợp lệ |
| 400 | `no fields to update` | Body là `{}`, hoặc mọi trường đều `null` |
| 400 | `validation failed` + `details` | Trường được gửi có giá trị không hợp lệ |
| 400 | `invalid request body: ...` | Body JSON sai, xem [Lỗi body JSON](README.md#lỗi-body-json-400) |
| 422 | `department 99 does not exist` | Đổi sang phòng ban không tồn tại |
| 404 | `Employee not found` | Không có nhân viên này, hoặc nhân viên đã bị xoá |

---

## DELETE /employees/{id}

Xoá mềm nhân viên: bản ghi vẫn còn trong DB nhưng bị ẩn khỏi mọi API đọc và khỏi file export.

```bash
curl -u admin:admin123 -X DELETE http://localhost:8080/employees/2
```

**Response 204**: không có body.

**Lỗi**

| Mã | `error` | Khi nào |
|---|---|---|
| 400 | `invalid employee id: "abc"` | `id` không hợp lệ |
| 404 | `Employee not found` | Không có nhân viên này, hoặc đã bị xoá trước đó (xoá lần hai cũng trả 404) |
