# RcloneGo 0.1.0-alpha.2

## English

This alpha improves the native interface and the public project experience.
Service logos now use simple SVG paths that render in MyGo. Add/account/workflow
actions share consistent icons, file paths have native browse buttons, the status
bar opens relevant details, and About links to source, issues and releases.
Scrollbars have reserved space; long names and narrow task details stay bounded.

The preview includes verified rclone installation/offline import, isolated
provider-driven account sessions, copy/mirror/check workflows, retained attempts,
pause/resume, tray operation, CLI JSON, OpenAPI and read-only observation tokens.
Repository guides, Issue/PR forms and Release notes support English and Chinese.
The portable executable embeds license notices, available in About or
`rclonego licenses`.

CI runs once per main push or PR, cancels stale PR jobs, and uses one verified
integration engine per job. Linux runs the race detector. Tag versions are checked
before packaging; platform hashes and archive contents are verified before a draft
release is assembled. Go dependency and action updates are grouped.

Packages: Windows amd64 portable executable/installer, Linux amd64 deb/tar.gz,
and macOS arm64 app/DMG. Windows/Linux native runtime and synthetic workflows have
been inspected; all three platforms run synthetic CI tests. macOS manual GUI and
real cloud-provider browser sign-in still need validation.

Windows is not Authenticode signed and macOS is not Developer ID notarized.
SHA256SUMS covers integrity, not OS signing. Google Drive needs your own OAuth
client. Automatic sync, separate-daemon GUI attachment and signed automatic client
updates remain future work. Existing uploaders and mounts are never taken over
automatically.

## 简体中文

这个 alpha 继续打磨原生界面和开源项目入口。
服务标志改用 MyGo 可正常渲染的简单 SVG；新增任务、账户和工作流使用统一的操作图标。
路径支持系统文件选择器，状态栏可以进入相关详情，“关于”提供源码、Issue 和 Release 入口。
滚动条预留空间，长名称和窄窗口详情不会挤到操作区域。

预览版包括官方引擎校验与离线导入、独立账户配置会话、复制/镜像/校验工作流、
尝试记录、暂停恢复、托盘运行、CLI JSON、OpenAPI 和只读观察 token。
README、贡献说明、Issue/PR 表单与 Release 说明支持中英文。
便携 exe 已嵌入许可文本，可通过“关于”或 `rclonego licenses` 查看。

CI 只对 main push 和 PR 验证，取消过时 PR 任务；每个 job 只准备一次校验过的测试引擎。
Linux 运行 race 检查。tag 与版本一致后才打包，平台校验和与归档内容检查通过后组装草稿发行。
Go 依赖和 Action 更新按组管理。

提供 Windows amd64 单文件/安装器、Linux amd64 deb/tar.gz 和 macOS arm64 app/DMG。
Windows、Linux 原生运行与合成工作流已有验收；三个平台均运行合成 CI 测试。
macOS 人工界面验收与真实云账户浏览器授权仍待验证。

Windows 尚无 Authenticode 签名，macOS 尚无 Developer ID 公证。
SHA256SUMS 只校验完整性，不替代系统签名。Google Drive 使用自己的 OAuth 客户端。
自动同步、独立 daemon 的 GUI 接入及签名自动更新属于后续工作。
客户端不自动接管已有上传器或挂载。
