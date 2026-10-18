# Health

[← Về tài liệu chung](README.md)

## GET /health

Kiểm tra server còn hoạt động. **Không cần đăng nhập.**

```bash
curl http://localhost:8080/health
```

**Response 200**

```json
{ "status": "ok" }
```

Endpoint này không trả lỗi nghiệp vụ nào. Gọi sai method (ví dụ `POST /health`) thì nhận 405, xem [Route lạ và sai method](README.md#route-lạ-và-sai-method).
