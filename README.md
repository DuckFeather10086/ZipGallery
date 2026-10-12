# ZipGallery

[MIT License](LICENSE) · macOS Apple Silicon

[下载发行版](https://github.com/DuckFeather10086/ZipGallery/releases)

macOS 上的 ZIP / CBZ 图片浏览器，使用 MyGo + Go + React / TypeScript。直接浏览包内目录与真实缩略图，按需读取单张图片，不完整解压压缩包。

## 启动

已构建的 Apple Silicon 应用：`build/darwin-arm64/ZipGallery.app`。双击即可打开，或：

```sh
open build/darwin-arm64/ZipGallery.app
```

开发和重新构建（本轮实测 Go 1.27.1、Bun 1.4.2、macOS 27.0.1 / Apple M4）：

```sh
bun install --frozen-lockfile
go mod download
bun run dev
# 发布构建，生成 .app 和本地 ad-hoc 签名的 .dmg
bun run build
# 只生成应用，不生成 DMG
go run github.com/egoist/mygo/cmd/mygo build -skip-dmg
```

`go.mod` / `go.sum`、`bun.lock` 与精确 npm 版本锁定依赖。MyGo / runtime / CLI 均为 0.4.5。应用未做 Developer ID 签名或 Apple 公证，仅本机开发验证。

## 使用

- “打开压缩包…”或 `⌘O` 选择文件；支持系统打开文件事件和文件拖入的统一入口。
- 首屏是包内目录：点击文件夹进入，通过面包屑或 `↑` 返回上级。“全部图片”跨目录浏览。
- 点击图片查看大图，左右/上下方向键、PageUp / PageDown 或按钮翻页。适应窗口模式下滚轮翻页；实际大小和放大模式下滚轮滚动图片。
- “适应窗口” / `0`、 “实际大小” / `1`；`＋` / `−` 缩放。`Esc` 返回网格，保留目录、选中项及滚动位置。
- 左上角 `←` / `→`（`Alt+←` / `Alt+→`）切换本轮打开过的压缩包，恢复之前的浏览状态。
- 侧栏保存最近 20 个文件和阅读进度。进度使用条目身份，不只记录排序下标；重启后选择最近文件，再点“继续阅读”。跨压缩包前进/后退栈不跨重启保存。
- 左下角切换简体中文 / English，选择自动保存。界面与错误文案位于 `src/locales/*.json`，Go 返回稳定错误码；添加语言时扩展 `src/i18n.ts` 的语言注册和语言选择项。
- “刷新”清理图片缓存并重新读取压缩包；“清理缓存”只清理图片缓存。

## 文件名编码修复

打开乱码包后，点击“修复文件名编码…”，选择 GB18030 / GBK、Shift-JIS 或 CP437，预览后保存 UTF-8 副本。保存后自动打开副本。

- 保留原压缩包，拒绝覆盖任何已有文件。
- 复制原始压缩数据，不解压重压图片；更新文件名、注释编码和 UTF-8 标志，移除过期的 Unicode 名称扩展字段。
- 已经有效的 UTF-8 名称保持不变。错误编码可能产生看似有效的乱码，必须检查预览；不宣称能自动识别所有旧编码或修复已经丢失的名称信息。
- 解码失败、转换后名称冲突、源文件版本变化会中止。修复的是名称编码，不是损坏的压缩数据；加密包等未列入修复兼容性承诺。
- 副本需要与原包相近的磁盘空间；这是显式的修复操作，普通打开不会复制整个压缩包。

## 分层与资源限制

- `internal/location`：本地 / mounted-local 来源，大小、版本、ReaderAt 和 Close。macOS 用 statfs 识别挂载的 SMB；不保存凭据，不猜测服务器或共享身份。
- `internal/archive`：archive/zip 索引、自然排序、独立条目 ID、按需读取、CRC 校验、编码修复。保留子目录，过滤 `__MACOSX`、资源叉和非图片条目。
- `internal/images`：2 个解码工作线程，当前大图优先于可见缩略图，再到邻图预读；同一会话条目合并请求。队列丢弃已取消请求；切包取消旧会话，旧请求不能覆盖新图。
- `internal/gallery`：会话生命周期、最近记录、阅读进度、只接受当前条目 ID 的二进制协议。
- `main.go`：MyGo 窗口、原生对话框、系统文件打开与单实例处理；前端统一调用 `OpenLocation`。
- `src`：React 目录视图与 TanStack Virtual 网格、查看器、语言配置。

缩略图最大 320×240；压缩图片解压输入上限 96 MiB，图片上限 2400 万像素，编码图片内存缓存 32 MiB，缩略图磁盘缓存 256 MiB。解码过程和 WebView 自身内存另计。当前显示链路统一输出 JPEG（质量 90），PNG / WebP 的透明度和精确无损像素不保留；JPEG EXIF 方向在后端归一化。动画不播放。内存不持久保留所有解码位图，前端 Blob URL 离开视图后释放。

状态：`~/Library/Application Support/ZipGallery/recent.json`；缓存：`~/Library/Caches/ZipGallery/thumbs`。设置 `ZIPGALLERY_DATA_DIR=/path` 可隔离这两类测试数据。语言由 WebView 本地存储保存。

## 已挂载 SMB 与限制

系统已挂载的 SMB 文件使用同一随机读取接口，无需先复制到本地。检测到 SMB 或 `/Volumes` 路径时记录为 `mounted/local`；记录依赖当前挂载路径，挂载位置变化后需要重新选择文件。离线时保留历史记录。应用内 SMB 直连、自动重连、凭据管理、网络超时控制均未实现。

已在实际挂载的 SMB 共享上用独立合成样本验证加密压缩、文件列表与解压往返，测试目录已清理。没有读取共享中的个人图片。断线恢复和网络性能尚未系统测试。

当前仅验证 macOS / arm64。图片预览使用 ZIP Store / Deflate 与标准库 ZIP64 路径；加密包、RAR、7z 等走文件列表和解压流程。图片预览暂不支持 HEIC、RAW 或超大图分块。文件版本由大小和修改时间识别；保持两者不变的外部修改需要显式刷新。

应用为 ZIP、CBZ、7z、RAR、tar、gz、bz2、xz、tgz 声明打开能力及文件图标。应用 SVG 源文件位于 `resources/icon.svg`，文件图标源文件位于 `resources/archive.svg`；`bun run build` 自动生成 PNG/ICNS 并打包。QA 版本仍使用 Alternate 排名。用户已明确要求设置本机默认打开程序，安装位置为 `~/Applications/ZipGallery.app`。其他机器可在 Finder 中设置默认打开程序。

## 验证与合成样本

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

样本全部由代码生成；包含自然排序、中文/日文子目录、重复 basename、损坏图片、空图片包及 GBK 文件名。`fixtures/` 和构建产物不进入 Git。详细实测及未验证项见 `docs/VALIDATION.md`。

### General archives

The app bundles official 7zz 26.04 (including licenses and corresponding source) in its Resources directory. ZIP/CBZ retain the image preview. Other archives and ZIPs without supported images open as file lists. Create ZIP, 7z and tar.gz; extract formats supported by 7zz, including RAR (RAR creation is not supported). ZIP/7z creation supports passwords; encrypted archives use the file list/extraction workflow. Jobs expose progress and cancellation. Outputs always require a new name. Extraction rejects links and unsafe paths and writes under a restricted temporary directory.

Mounted SMB shares are available through “Mounted shares” and native open/save panels. Connections and credentials remain managed by macOS. A synthetic encrypted round trip on an actual mounted share is covered in validation. Server disconnection can delay the underlying operating-system file calls.

7-Zip provenance and corresponding source: `third_party/7zip/PROVENANCE.txt`. The app executes a replaceable standalone helper, without copying MacPacker code.

## License

ZipGallery 自身代码、文档和原创 SVG 图标采用 [MIT License](LICENSE)。内置 7zz 和其他第三方依赖保留各自许可证，详见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
