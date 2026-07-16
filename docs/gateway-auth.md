1. Gateway verify JWT signature + expiry (không cần gọi auth-service)

Dùng RS256/ES256 thay vì HS256: auth-service giữ private key để ký token, gateway chỉ cần public key để verify → gateway không cần biết secret thật sự, giảm rủi ro bảo mật đáng kể so với việc share secret key HS256

2. Nếu cần revoke ngay lập tức hoặc check thêm quyền động (business permission)

Gateway verify JWT xong (đã biết user hợp lệ, chưa hết hạn)
Nhưng nếu cần check thêm business scope/permission phức tạp (như cái api_scopes bạn đang thiết kế), backend service mới gọi tiếp sang 1 service khác (không nhất thiết là auth-service) qua gRPC để lấy permission chi tiết, hoặc cache permission ở Redis để tránh gọi trực tiếp mỗi request

3. Auth-service dùng gRPC để phân phối public key/JWKS

Thay vì gateway hardcode public key, gateway có thể gọi gRPC GetPublicKey() từ auth-service lúc khởi động và cache lại, refresh định kỳ — vừa tách biệt, vừa không tạo bottleneck vì không gọi mỗi request