# Hướng dẫn build file IPA (sing-box cho iOS) từ Windows

> **Sự thật cần nói rõ:** dự án `sing-box-for-android-byedpi` trong máy bạn là ứng dụng
> **Android thuần** — nó *không thể* build thành IPA. File IPA dưới đây là bản dựng của
> **ứng dụng sing-box chính thức cho iOS (SFI)**, được build trên máy ảo macOS miễn phí
> của GitHub, rồi ký & cài về iPhone ngay trên Windows.
>
> ⚠️ Bản này **KHÔNG có tính năng ByeDPI** (ByeDPI chỉ tồn tại trong bản Android fork này).

---

## Tổng quan quy trình

```
GitHub Actions (macOS ảo, miễn phí)
   ├─ build Libbox.xcframework từ mã Go (gomobile)
   ├─ biên dịch app SwiftUI "SFI" bằng Xcode (chưa ký số)
   └─ xuất SFI-unsigned.ipa  →  bạn tải về
Windows của bạn
   └─ Sideloadly ký IPA bằng Apple ID của bạn → cài vào iPhone bằng cáp
```

Repo công khai dùng runner macOS **miễn phí** và **không giới hạn** (repo riêng tư thì bị trừ phút).

---

## Bước 1 — Tạo tài khoản GitHub (bỏ qua nếu đã có)

1. Vào https://github.com/signup , đăng ký miễn phí.

## Bước 2 — Tạo repo chứa kịch bản build

1. Đăng nhập GitHub → nhấn dấu **+** góc phải → **New repository**.
2. Đặt tên ví dụ: `build-sfi-ipa`.
3. Chọn **Public** (bắt buộc để dùng macOS runner miễn phí) → **Create repository**.

## Bước 3 — Thêm file workflow

1. Trong repo vừa tạo: nhấn **"creating a new file"** hoặc **Add file → Create new file**.
2. Ô tên file gõ: `.github/workflows/build-ipa.yml`
   (gõ dấu chấm đầu tiên là GitHub tự tạo thư mục).
3. Mở file `ipa-build/build-ipa.yml` trong thư mục này, **sao chép toàn bộ nội dung**, dán vào.
4. Nhấn **Commit changes**.

## Bước 4 — Chạy build

1. Vào tab **Actions** của repo → nếu được hỏi, nhấn **I understand my workflows, go ahead and enable them**.
2. Chọn workflow **Build unsigned SFI IPA** ở cột trái → nút **Run workflow** ▼
3. Giữ mặc định:
   - `sing_box_ref`: `v1.14.0-rc.1`
   - `libbox_platforms`: `ios`
4. Nhấn **Run workflow**. Thời gian: **~30–60 phút** (giai đoạn gomobile là lâu nhất).
5. Xong xuôi, mở đúng lần chạy đó → mục **Artifacts** → tải **SFI-unsigned-ipa**
   (máy sẽ tải một file .zip, bên trong là `SFI-unsigned.ipa`).

### Nếu build thất bại (lỗi lệch phiên bản API giữa app và libbox)

Hai nhánh phát triển (`apple: dev` ↔ `sing-box: testing`) đi cùng nhau. Tùy lúc chạy:

- Lỗi Swift kiểu *"cannot find ... in scope"* / sai chữ ký hàm Libbox
  → **chạy lại** với `sing_box_ref` = `testing`.
- Ngược lại nếu lỗi nằm ở giai đoạn Go/gomobile với `testing`
  → thử quay lại tag ổn định mới nhất (ví dụ `v1.14.0-rc.1`).

## Bước 5 — Ký và cài vào iPhone bằng Sideloadly (trên chính Windows này)

1. Cài 2 phần mềm của Apple (**bản tải trực tiếp từ apple.com, KHÔNG dùng bản Microsoft Store**):
   - iTunes: https://www.apple.com/itunes/
   - iCloud: https://support.apple.com/downloads/icloud
2. Tải Sideloadly tại https://sideloadly.io → cài đặt.
3. Kết nối iPhone với máy tính bằng cáp, tin tưởng máy tính nếu iPhone hỏi.
4. Mở Sideloadly:
   - Kéo thả file `SFI-unsigned.ipa` vào cửa sổ.
   - Nhập **Apple ID** của bạn (nên dùng **App-Specific Password**:
     tạo tại https://appleid.apple.com → Sign-In and Security → App-Specific Passwords).
   - Nhấn **Start** → đợi ký xong.
5. Trên iPhone: **Cài đặt → Cài đặt chung → Quản lý thiết bị & VPN (VPN & Device Management)**
   → chọn chứng chỉ Apple ID của bạn → **Tin tưởng (Trust)**.
6. Mở app sing-box → cấp quyền VPN → thêm profile JSON/link subscription như bên Android → bật kết nối.

### Giới hạn ký miễn phí (Apple ID thường)

| Loại tài khoản | Thời hạn | Ghi chú |
|---|---|---|
| Miễn phí | **7 ngày** / lần ký | Hết hạn phải cắm cáp ký lại; tối đa 3 ứng dụng |
| Developer ($99/năm) | **365 ngày** | Không giới hạn số thiết bị thường dùng |

---

## ⚡ Tích hợp ByeDPI (mới)

Workflow bản mới có input **`enable_byedpi`** (mặc định **bật**). Khi bật, engine
**ciadpi/ByeDPI** được nhúng trực tiếp vào `Libbox.xcframework` (tầng Go/cgo) — tức là
IPA thu được có sẵn khả năng phá DPI giống hệt cơ chế detour của bản Android fork,
**mà không cần sửa bất kỳ dòng Swift hay file Xcode nào** của upstream.

### Cách bật ByeDPI trong profile (trên iPhone)

Mở profile trong app (nút sửa nội dung JSON) và thêm khối `experimental.byedpi`:

```json
{
  "log": { "level": "info" },
  "outbounds": [ "...như cũ..." ],
  "experimental": {
    "byedpi": {
      "enabled": true,
      "listen_port": 1080,
      "command_line": "-Ku -a1 -An -o1 -At,r,s -d1"
    }
  }
}
```

- `enabled`: bật/tắt. Khi `false`, cấu hình gốc giữ nguyên và proxy bị dừng.
- `listen_port`: cổng SOCKS cục bộ (mặc định `1080`). Nếu chuỗi lệnh đã chứa
  `--port/-p` thì trường này bị bỏ qua.
- `command_line`: tham số thô của ciadpi (mặc định như trên). Các cờ
  `--ip/-i` tự thêm nếu thiếu; cờ `--protect-path/-P` **bị loại bỏ** vì iOS không cần
  (socket của extension không bao giờ bị route ngược vào tunnel của chính nó).
- Eligibility giống hệt Android: mọi outbound TCP thực (vmess/vless/trojan/ss/socks/
  http/ssh...) nhận `detour = "byedpi"`; bỏ qua selector/urltest/direct/block/dns/
  hysteria/hysteria2/tuic/wireguard; server dạng hostname được thêm
  `domain_resolver: "local"`.

### Log khi chạy

Xem log trong app (hoặc Console.app lọc theo process tunnel) tìm tiền tố `[byedpi]`:

- `started successfully on port 1080` → proxy đã sẵn sàng
- `listen port ... not ready` → ciadpi thoát sớm (thường do sai cờ hoặc trùng cổng)

### Build KHÔNG ByeDPI

Chạy workflow với `enable_byedpi = false` để thu được IPA libbox thuần upstream.

## Lưu ý quan trọng

- **ByeDPI chỉ có ở bản build từ workflow này** (nhúng trong libbox, xem mục trên).
  Bản "sing-box VT" trên App Store và mọi bản chính thức khác KHÔNG có ByeDPI.
  Engine ciadpi chạy trong tiến trình extension; nếu server của bạn bị chặn DPI ở tầng
  giao thức mà ByeDPI không vượt qua nổi, hãy xử lý thêm phía server.
- App Store region: nếu Apple ID của bạn là Trung Quốc đại lục sẽ không cài được
  (cả bản store lẫn sideload đều ảnh hưởng — nên dùng Apple ID khu vực khác).
- Phương án thay thế đơn giản hơn: cài thẳng **"sing-box VT"** trên App Store
  (https://apps.apple.com/app/sing-box-vt/id6673731168) — nhưng bản store hiện đang bị
  đóng băng cập nhật theo thông báo chính thức.

## Nguồn đã kiểm chứng (thời điểm thực hiện)

- Ứng dụng Apple chính thức + yêu cầu iOS 15+: https://sing-box.sagernet.org/clients/apple/
- Repo app: https://github.com/SagerNet/sing-box-for-apple (scheme iOS = `SFI`, cần `Libbox.xcframework`)
- Chuỗi build libbox: https://github.com/SagerNet/sing-box → `make lib_install` + `cmd/internal/build_libbox -target apple`
  (gomobile sagernet v0.1.13, tự copy framework sang `../sing-box-for-apple/`)
