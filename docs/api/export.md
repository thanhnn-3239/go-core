# Export

[← Về tài liệu chung](README.md)

Endpoint này yêu cầu [Basic Auth](README.md#xác-thực).

## POST /employees/export

Xuất toàn bộ nhân viên (chưa bị xoá) ra hai file cùng lúc. Server chạy hai goroutine song song, một ghi JSON và một ghi CSV, rồi chờ cả hai xong mới trả kết quả.

| File | Nội dung |
|---|---|
| `<EXPORT_DIR>/employees.json` | Mảng [Employee](employees.md#mô-hình-employee), có thụt lề |
| `<EXPORT_DIR>/employees.csv` | Có BOM UTF-8 để Excel hiển thị đúng tiếng Việt. Dòng tiêu đề: `id,name,age,position,departmentId,salary,createdAt,updatedAt` |

`EXPORT_DIR` mặc định là thư mục `exports` nằm cạnh nơi chạy server.

- Mỗi lần export ghi đè file cũ. File được ghi ra file tạm `.tmp` rồi mới đổi tên, nên không bao giờ đọc phải file ghi dở.
- Nếu có hai request export cùng lúc, request sau chờ request trước xong rồi mới chạy.

```bash
curl -u admin:admin123 -X POST http://localhost:8080/employees/export
```

**Response 200**

| Trường | Kiểu | Mô tả |
|---|---|---|
| `count` | integer | Số nhân viên đã xuất |
| `files.json` | string | Đường dẫn file JSON trên máy chạy server |
| `files.csv` | string | Đường dẫn file CSV trên máy chạy server |

```json
{
  "count": 3,
  "files": {
    "json": "exports\\employees.json",
    "csv": "exports\\employees.csv"
  }
}
```

Trên Windows dấu phân cách đường dẫn là `\`.

**Lỗi**

| Mã | `error` | Khi nào |
|---|---|---|
| 401 | `unauthorized` | Chưa đăng nhập, xem [Xác thực](README.md#xác-thực) |
| 500 | `internal server error` | Không ghi được file (ví dụ hết dung lượng, không có quyền ghi) |
| 504 | `request timed out` | Hết thời gian xử lý trong lúc đọc DB hoặc ghi file |

`GET /employees/export` **không** trả 405: nó khớp route `GET /employees/{id}` với `id = "export"`, nên trả 400 `invalid employee id: "export"`.
