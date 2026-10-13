# 开发说明

面向使用者的安装和操作说明见 [README](../README.md)。

## 开发运行与构建

要求 macOS、Xcode Command Line Tools、Go 和 Bun。当前验证工具版本为 Go 1.27.1 / Bun 1.4.2；依赖版本由锁文件固定。

```sh
bun install --frozen-lockfile
go mod download
bun run dev
# 打包应用与 DMG（包含图标生成）
bun run build
# 仅生成图标
bun run icons
```

MyGo / runtime / CLI 使用 0.4.5；应用采用 Go + MyGo 与 React / TypeScript。正式构建产物位于 `build/darwin-arm64/`。设置 `ZIPGALLERY_QA=1` 构建独立标识的 QA 应用，输出到 `build/qa/`。

## 测试

```sh
go test -race ./...
go vet ./...
bun test
bun run build:web
go run ./scripts/fixtures
# 可选：生成约 3.05 GB / 1200 张合成图片的 CBZ
go run ./scripts/fixtures -large
go build -o build/benchmark ./scripts/benchmark
/usr/bin/time -l build/benchmark fixtures/large.cbz
```

真实共享的往返测试需要显式指定目录：`ZIPGALLERY_TEST_SHARE=/path/to/mounted/share go test ./internal/archiver -run TestMountedRoundTrip -v`。只在可写、允许创建合成测试目录的挂载中运行；测试会清理自己创建的目录。

验证记录和未验证项见 [VALIDATION.md](VALIDATION.md)。

## 分层与资源限制

- `internal/location`：本地 / mounted-local 来源，大小、版本、ReaderAt 和 Close。macOS 用 statfs 识别挂载的 SMB；不保存凭据，不猜测服务器或共享身份。
- `internal/archive`：archive/zip 索引、自然排序、独立条目 ID、按需读取、CRC 校验、编码修复。保留子目录，过滤 `__MACOSX`、资源叉和非图片条目。
- `internal/images`：2 个解码工作线程，当前大图优先于可见缩略图，再到邻图预读；同一会话条目合并请求。队列丢弃已取消请求；切包取消旧会话，旧请求不能覆盖新图。
- `internal/gallery`：会话生命周期、最近记录、阅读进度、只接受当前条目 ID 的二进制协议。
- `main.go`：MyGo 窗口、原生对话框、系统文件打开与单实例处理；前端通过 `OpenArchive` 打开与解锁，`OpenLocation` 保留为无显式密码的入口。
- `src`：React 目录视图与 TanStack Virtual 网格、查看器、语言配置。

缩略图最大 320×240；压缩图片解压输入上限 96 MiB，图片上限 2400 万像素，编码图片内存缓存 32 MiB，缩略图磁盘缓存 256 MiB。解码过程和 WebView 自身内存另计。当前显示链路统一输出 JPEG（质量 90），PNG / WebP 的透明度和精确无损像素不保留；JPEG EXIF 方向在后端归一化。动画不播放。内存不持久保留所有解码位图，前端按相邻图片窗口淘汰 Blob URL，并取消过期预读。

状态：`~/Library/Application Support/ZipGallery/recent.json`；缓存：`~/Library/Caches/ZipGallery/thumbs`。设置 `ZIPGALLERY_DATA_DIR=/path` 可隔离这两类测试数据。语言由 WebView 本地存储保存。


## 国际化与图标

界面文案和错误码翻译位于 `src/locales/zh-CN.json` 与 `src/locales/en.json`。增加语言时，同时更新 `src/i18n.ts` 的语言注册与语言选择项。

原始矢量图标位于 `resources/icon.svg` 和 `resources/archive.svg`。`scripts/icons.ts` 使用 resvg 生成应用 PNG 与文档 ICNS；不要只改生成文件。

## README 截图

```sh
bun scripts/readme-demo.ts
```

生成 `fixtures/ZipGallery-Demo.zip`，内含六张原创合成风景图。所有样本保持在被忽略的 `fixtures/` 中。

公开截图位于 `docs/screenshots/`，从独立 QA 应用的真实窗口直接捕获。使用隔离的数据目录和 `/tmp/ZipGallery-Demo.zip`，避免个人最近记录和路径进入截图；截图未使用界面重绘。当前截图展示英文界面，正式应用也支持简体中文。

## 发行注意事项

保留根目录 MIT 许可证和 [第三方声明](../THIRD_PARTY_NOTICES.md)。7zz 的许可证、来源和对应源码放在 `third_party/7zip/`，其他运行时依赖声明放在 `third_party/licenses/`，均随应用打包。

`scripts/register-defaults.swift` 是本机显式设置默认关联的辅助脚本，不在应用启动时自动运行。源码、测试样本生成器、构建脚本采用项目 MIT 许可证；第三方组件遵循其自身许可证。
