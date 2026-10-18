# Phòng ban

[← Về tài liệu chung](README.md)

Mọi endpoint trong file này đều yêu cầu [Basic Auth](README.md#xác-thực). Lỗi 401, 500 và 504 dùng chung cho mọi API nên không nhắc lại ở từng endpoint, xem [Định dạng lỗi](README.md#định-dạng-lỗi).

## Mục lục

- [Mô hình Department](#mô-hình-department)
- [POST /departments](#post-departments)
- [GET /departments](#get-departments)
- [GET /departments/{id}/employees](#get-departmentsidemployees)

---

## Mô hình Department

| Trường | Kiểu | Mô tả |
|---|---|---|
| `id` | integer | ID tự tăng |
| `name` | string | Tên phòng ban. Duy nhất, không phân biệt hoa thường. |
| `createdAt` | string (RFC 3339) | Thời điểm tạo |

```json
{
  "id": 1,
  "name": "Engineering",
  "createdAt": "2026-10-18T14:30:46.344697Z"
}
```

---

## POST /departments

Tạo phòng ban mới.

**Request body**

| Trường | Kiểu | Bắt buộc | Ràng buộc |
|---|---|---|---|
| `name` | string | Có | Không rỗng sau khi cắt khoảng trắng. Không trùng tên đã có (không phân biệt hoa thường). |

```bash
curl -u admin:admin123 -X POST http://localhost:8080/departments \
  -H "Content-Type: application/json" \
  -d '{"name":"Engineering"}'
```

**Response 201**: [Department](#mô-hình-department) vừa tạo. Tên được cắt khoảng trắng hai đầu trước khi lưu.

```json
{
  "id": 1,
  "name": "Engineering",
  "createdAt": "2026-10-18T14:30:46.344697Z"
}
```

**Lỗi**

| Mã | `error` | Khi nào |
|---|---|---|
| 400 | `validation failed` + `details: [{"field":"name","message":"is required"}]` | Tên rỗng hoặc chỉ có khoảng trắng |
| 400 | `invalid request body: ...` | Body JSON sai, xem [Lỗi body JSON](README.md#lỗi-body-json-400) |
| 409 | `department "engineering" already exists` | Đã có phòng ban cùng tên, kể cả khác hoa thường (`Engineering` và `engineering`) |

---

## GET /departments

Lấy danh sách tất cả phòng ban, sắp xếp theo `id` tăng dần.

```bash
curl -u admin:admin123 http://localhost:8080/departments
```

**Response 200**

```json
{
  "departments": [
    { "id": 1, "name": "Engineering", "createdAt": "2026-10-18T14:30:46.344697Z" },
    { "id": 2, "name": "Sales", "createdAt": "2026-10-18T14:30:46.4790327Z" }
  ]
}
```

Chưa có phòng ban nào thì trả `{"departments":[]}`.

---

## GET /departments/{id}/employees

Lấy thông tin phòng ban kèm danh sách nhân viên (chưa bị xoá) thuộc phòng ban đó, sắp xếp theo `id` nhân viên.

**Path parameter**

| Tham số | Kiểu | Ràng buộc |
|---|---|---|
| `id` | integer | Số nguyên ≥ 1 |

```bash
curl -u admin:admin123 http://localhost:8080/departments/1/employees
```

**Response 200**: các trường của [Department](#mô-hình-department), cộng thêm mảng `employees` gồm các [Employee](employees.md#mô-hình-employee).

```json
{
  "id": 1,
  "name": "Engineering",
  "createdAt": "2026-10-18T14:30:46.344697Z",
  "employees": [
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
  ]
}
```

Phòng ban tồn tại nhưng chưa có nhân viên thì `employees` là `[]`.

**Lỗi**

| Mã | `error` | Khi nào |
|---|---|---|
| 400 | `invalid department id: "abc"` | `id` không phải số nguyên ≥ 1 |
| 404 | `Department not found` | Không có phòng ban với `id` này |
