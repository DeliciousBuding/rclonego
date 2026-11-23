# RcloneGo 0.1.0-alpha.4

## English

This preview brings the native Files workspace, workflow improvements and signed
MyGo updates to the published release, with MyGo 0.10 and its native SQLite module.

- Browse local and remote files with breadcrumbs, keyboard selection, previews
  and context actions. Queue file operations with clear partial results and retry
  only items that were not submitted.
- Use compact, virtualized task tables and a directory-only folder picker.
  Refresh affected folders after writes while preserving unrelated selections.
- Edit and reuse workflows, inspect provider capacity and backend capabilities,
  observe owned VFS cache/writeback state and queue independent offline copies.
- Use the official MyGo update window and stable/preview channels. Active
  transfers and owned mounts defer update-triggered relaunch.
- Upgrade MyGo from 0.8.3 to 0.10.0, use the current Go toolchain and update
  packaging dependencies with upstream security fixes.
- Correct Windows/macOS filesystem identity assertions in release tests.

All five native targets passed their full product tests, `go vet`, packaged CLI
checks and signed update archive checks: Windows amd64/arm64, Linux amd64 and
macOS amd64/arm64. Linux also passed the race detector. Windows standalone
execution without an adjacent SQLite DLL, local copying, hash verification and
state recovery were checked. Installer lifecycle checks used an isolated
registration and synthetic profile.

The workflow's final draft-upload job failed on draft lookup. The same checked
artifacts were uploaded with the corrected helper; no packages were rebuilt.

The release includes Windows EXE installers, ZIPs and standalone executables;
Linux DEB and tar.gz packages; macOS DMGs and application ZIPs; and signed MyGo
update manifests and archives for all five targets. `SHA256SUMS` and dependency
inventories accompany the packages.

Windows packages are not Authenticode signed. macOS uses ad-hoc signing without
Developer ID notarization. Manual Windows ARM64/macOS GUI installation, real cloud
sign-in, sleep/wake and end-to-end update/relaunch remain separate acceptance
checks. Renamed standalone Windows executables use manual updates. No stable
release is available yet.

## 简体中文

本预览版将原生文件工作页、工作流改进与 MyGo 签名更新交付到公开发行，
并升级到 MyGo 0.10 及其原生 SQLite 模块。

- 本机与云端文件支持目录导航、键盘选择、预览和右键操作；文件更改进入共享队列，
  批量结果明确部分成功，只重试尚未提交的条目。
- 使用紧凑的虚拟化任务表与只选目录的文件夹选择器；写入后刷新受影响目录，
  保留无关选择。
- 编辑和复用工作流，读取云盘容量与后端能力，观察所属挂载的 VFS 缓存与写回，
  创建独立离线副本。
- 使用 MyGo 官方更新窗口与稳定版/预览版通道；活动传输及所属挂载推迟更新重启。
- MyGo 从 0.8.3 升至 0.10.0，更新 Go 工具链与打包依赖，纳入上游安全修复。
- 修正 Windows/macOS 发布测试中的文件系统身份断言。

Windows amd64/arm64、Linux amd64、macOS amd64/arm64 五个原生目标均通过完整产品测试、
`go vet`、打包后 CLI 检查及签名更新归档检查；Linux 同时通过 race 检查。
Windows 单独 exe 已验证无需相邻 SQLite DLL 启动、本地复制、哈希校验与状态恢复；
安装器生命周期验证使用独立注册标识和合成配置。

工作流最后的草稿上传 job 因草稿查询失败；使用修正后的执行器上传了同一批已核验产物，
没有重新构建发行包。

发行包含 Windows EXE 安装器、ZIP 与单独 exe，Linux DEB 与 tar.gz，
macOS DMG 与 app ZIP，以及五个平台的 MyGo 签名更新 manifest 和归档。
同时提供 `SHA256SUMS` 与依赖清单。

Windows 尚无 Authenticode 签名；macOS 使用 ad-hoc 签名，尚无 Developer ID 公证。
Windows ARM64/macOS 人工界面安装、真实云账户登录、休眠唤醒与完整更新重启流程仍待独立验收。
重命名的 Windows 单独 exe 使用手动更新；目前尚无稳定版。
