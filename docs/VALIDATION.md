# 验证记录 — 2026-10-12

环境：Apple M4，16 GiB 内存，macOS 27.0.1（26A434），Go 1.27.1 darwin/arm64，Bun 1.4.2。MyGo v0.4.5。

## 自动检查

- `go test -race ./...`：通过。
- `go vet ./...`：通过。
- `bun test`：滚轮累积阈值、惯性抑制、反向翻页测试通过。
- `bun run build`：TypeScript、Vite、Go 编译及 macOS app/DMG 构建通过。
- ZIP 索引：1、2、10 自然排序，中文/日文路径，重复完整文件名 ID 不冲突，过滤资源叉和 __MACOSX，关闭后不可读取。
- 图片协议：真实缩略图不超过 320×240，大图可解码，损坏图片独立返回错误，旧会话 ID 返回 410，失败打开不替换现有会话。
- 进度：按条目身份保存、重新打开和重新实例化后恢复；磁盘缓存可清理。
- EXIF orientation=6：40×20 输入正确旋转为 20×40；1×1 WebP 解码通过。
- 错误：加密标志、不支持压缩方法、解压上限、CRC 错误被拒绝。
- 编码：GB18030、Shift-JIS、CP437 转 UTF-8；原包字节不变，条目压缩数据逐字节相等，拒绝覆盖原包/已有目标，错误编码被拒绝。
- 原始非 UTF-8 文件名转换为安全展示文本，内部身份保持唯一，避免 MyGo 严格 JSON 编码失败。

## 原生窗口实际操作

使用 `ZipGallery QA.app` 与隔离的数据目录，在 MyGo WKWebView 中验证，而非静态网页：

- 生成的 ZIP 打开、真实棋盘格缩略图及不同色彩 JPEG/PNG 显示成功。
- 根目录显示两个子目录，图片顺序为 1.jpg、2.png、10.jpg，broken.jpg 显示解码错误。
- 点击 1.jpg 进入大图，方向键到 2.png，按钮到 10.jpg；Esc 返回网格。
- 进入日文子目录，显示其子文件夹和 1、2、10 图片。
- 使用原生文件选择器打开相同内容 CBZ；重启后确认 CBZ 正常显示，连续按右键翻页。
- 从最近文件进入 ZIP，后退恢复 CBZ，前进恢复 ZIP 的日文子目录。
- 最近文件与阅读进度在重启后存在。
- 1200 张大包的可视网格位于 0092–0140 区域时，进入 0110.jpg、翻页、返回后可视区域不变；切到 sample.cbz 再后退，仍恢复同一区域。
- 切换 English 后，侧栏、工具栏、查看器和修复面板文案切换成功。
- 第二进程启动参数交给已有窗口（旧编码样本），不产生第二个主窗口。
- GBK 样本打开后进入修复预览，正确显示“测试图片/第一页.jpg”；通过原生保存对话框生成 legacy-gbk.utf8.zip，并在持久记录确认副本自动打开。随后在窗口重新打开副本，正确显示“测试图片”目录与“第一页.jpg”缩略图。

桌面自动化工具在关闭原生打开/保存对话框后发生 AX timeout，重启 QA 可恢复读取；文件选择/保存本身已完成（状态文件与产物验证）。工具的原生滚动调用返回 `noWindowsAvailable`，因此不将滚轮端到端操作列为已通过；滚轮处理逻辑已有单元测试。拖入、实际网络 SMB、全窗口首屏绘制耗时及包含 WebKit 的峰值内存尚未测量。

## 大包后端实测

`fixtures/large.cbz`：1200 个 JPEG 条目，每张 1800×1200、随机 RGB 噪声、JPEG 质量 95、ZIP Store，无子目录；同一合成图复制为不同条目，约 3.05 GB。用于规模测试，不代表真实漫画的压缩率、视觉内容或所有磁盘条件。

一次运行结果（新建独立缓存目录，系统文件缓存未清空）：

- 目录索引：3.345 ms。
- 首批 12 张缩略图，应用冷缓存：382.874 ms。
- 同批 12 张，内存热缓存：0.125 ms。
- 20 张完整图片顺序解码：中位 108.825 ms，最大 131.620 ms。
- 此次缩略图磁盘占用：242,196 字节。
- `/usr/bin/time -l` 记录后端进程最大 resident set：90,537,984 字节（约 86.3 MiB）。

上述是 Go 后端指标，排除 WebView 绘制及其进程内存；不是冷启动“秒开”承诺。仅请求了首批缩略图和 20 张大图，没有预先完整解包。真实网络 SMB 性能未测。

## 2026-10-12 general archiver extension

- Bundled official 7-Zip 26.04 universal macOS helper, license and corresponding source.
- Automated local round trips: ZIP and 7z with/without passwords; tar.gz creation and two-stage extraction; wrong password; unsafe parent traversal; output collision protection.
- Synthetic mounted SMB test: create password-protected 7z on mounted share, list/extract on that share, compare contents, remove owned test directory. Passed. Server requires exclusive-create/copy fallback instead of renamex_np.
- Passwords go via stdin, not argv; no persistent password storage.
- Extraction process writes restricted to its owned temporary directory using macOS sandbox-exec; links rejected; existing outputs refused.
- Mouse notch and trackpad momentum unit tests cover removed fixed 280 ms dead time. Native physical-wheel feel still requires user feedback; automated native scrolling was unavailable in the previous CUA session.
- Real running-job cancellation test passed: 128 MiB synthetic input, cancellation while 7zz runs, no final output or temporary directory remains.
- Native QA: sample ZIP image/file toggle; native extraction dialog wrote the expected synthetic tree; encrypted-header 7z prompted for password and displayed its file list after typing the correct synthetic password; image next-page key displayed the next image.
- Native QA limitations: AX reads time out after some file dialogs. Native wheel dispatch still reports noWindowsAvailable; the wheel logic is unit-tested, but physical scroll feel is not claimed as verified.
- Final checks: Go race suite, vet, Bun wheel tests, TypeScript/Vite build, signed macOS arm64 app and DMG build.

## Continuous wheel paging revision

Replaced per-gesture suppression with the same event-driven repeat sampler used for held arrow keys: immediate first/reversed input, 90 ms sustained cadence, fresh gesture after 160 ms idle. No timer or accumulated backlog advances pages after input ends. A synchronous live cursor prevents successive events from targeting the same stale React index; the wheel listener remains attached across page changes. Five Bun tests (48 assertions) cover continuous input, sampling-rate independence, held-key parity, reversal, fresh presses and stalls. Physical mouse/trackpad feel still requires user validation because native automation cannot dispatch wheel events in this app.

## 2026-10-13 / v0.1.1 navigation and encrypted previews

- Frontend DOM interaction regression test: parent-folder navigation, back/forward and Alt+Left, scroll restoration, forward-history branching, cross-archive reopening, failed navigation, wrong-password retry, cancelled/retried history unlock, and extraction dialogs without redundant password fields. Bridge responses are mocked; this is not native-window verification.
- Backend integration tests use real 7zz for encrypted ZIP, 7z with visible headers, and 7z with encrypted headers. Empty/wrong passwords preserve the previous view; verified passwords allow real thumbnail/full-image responses, history restoration and extraction. Changed source versions invalidate remembered credentials.
- Encrypted thumbnails stay out of the persistent disk cache. Temporary preview directories are removed on close and when no previewable images exist.
- Native QA window inspection could not complete because the desktop tool returned `cgWindowNotFound`; no new native-window acceptance is claimed for this patch.
- Release checks passed: `go test -race ./...`, `go vet ./...`, `bun test` (6 tests / 79 assertions), and the production arm64 app/DMG build with TypeScript checking.
