# RcloneGo 0.1.0-alpha.3

### English

- Preview a legacy task import and continue remaining steps after the previous
  client's active transfer finishes, preserving filters and final verification.
- Refine file browsing with bounded search/sort, keyboard selection and path
  details. Unify the runtime brand mark with the generated assets.
- Inspect provider capacity and backend capabilities, browse files with sizes,
  and request cancellable directory totals through the GUI, CLI and API.
- Identify the engine with the official rclone symbol and show external rclone
  processes separately from client resources and per-attempt diagnostics.
- Use MyGo's native update window, signed archive verification and stable/preview
  channels. Active transfers and owned mounts defer update-triggered relaunch.
- Verify generated update archives and delta signatures before drafting a release.
- Recover durable task state, stop dispatch when storage fails, verify process
  ownership and bound task logs.
- Browse remote directories and inspect structured transfer previews.
- Edit, clone, import and export workflows, page through history and verify only
  selected entries.
- Add opt-in directory watching and recurring workflows to the shared queue.
- Use official MyGo Glass and language catalogs, command search and background
  refresh throttling.
- Keep appearance, language, login startup and updates in one Settings page,
  with engine management and new-task defaults collapsed until needed.
- Follow the system theme and language on first launch, show manual language
  overrides explicitly, and keep profile selection consistent across GUI and CLI.
- Install per user on Windows without stopping running transfers; preserve
  configuration on upgrade/uninstall and remove only owned startup entries.
- Build releases only through manual Actions dispatch, with one pinned source
  commit, native platform checks and an unpublished draft.
- Align page headings and primary actions, move secondary workflow actions to
  More, and use Mica for the Windows main window.
- Clarify activity phases, use file counts for verification, show mount routes
  and keep empty history and task details concise.
- Keep task actions visible, explain input issues at their fields and present
  results with verification evidence and recovery. Retain critical engine errors.
- Observe owned VFS cache/writeback state and queue persistent offline copies.
- Extend Agent contracts, event streaming and revocable task permissions.

### 简体中文

- 预览导入旧客户端任务，当前上传完成后接续剩余步骤，保留过滤条件与最终校验。
- 完善文件浏览搜索排序、键盘选择和路径详情，运行时标志与生成资产共用品牌源。
- GUI、CLI、API 共用云盘容量、后端能力、文件大小与可取消的目录统计。
- 使用官方 rclone 标识，单独展示外部 rclone 资源与本客户端尝试诊断。
- 接入 MyGo 官方原生更新窗口、签名归档校验和稳定版/预览版通道。
  活动传输与本实例拥有的挂载会推迟更新重启。
- 创建发行草稿前核验更新归档与差量包签名。
- 完善持久化恢复、存储失败时停止调度、进程归属校验与任务日志限制。
- 新增云端目录浏览、结构化预览、工作流编辑/复制/导入导出和历史分页。
- 最终校验遵循所选步骤范围；目录监听和周期工作流复用同一队列。
- 使用 MyGo 官方 Glass 和语言目录，加入命令搜索与后台刷新降频。
- 设置页统一管理主题、语言、登录启动与更新，引擎和新任务默认参数按需展开。
- 首次启动跟随系统主题和语言，明确显示手动语言选择，GUI 与 CLI 使用同一配置目录。
- Windows 按当前用户安装，运行中拒绝覆盖或卸载；保留升级与卸载后的配置，只清理所属启动项。
- 发行仅由手动 Actions 触发，固定同一提交，完成原生平台检查后生成未公开草稿。
- 统一页面标题与主操作，工作流低频操作收进“更多”，Windows 主窗口采用 Mica。
- 区分活动阶段，校验按文件数显示进度；补齐映射路径，精简空历史与任务详情。
- 固定任务操作区，就地显示输入问题；结果窗口展示校验依据与恢复入口，保留引擎关键错误。
- 展示本实例挂载的 VFS 缓存与写回状态，支持独立离线副本任务。
- 完善 Agent 契约、事件流和可撤销的任务权限。

## Validation and distribution

Build packages target Windows amd64/arm64, Linux amd64 and macOS amd64/arm64. Runtime
validation is reported in README; macOS manual GUI, actual cloud sign-in,
sleep/wake and OS signing are separate checks. Windows is not Authenticode
signed and macOS is not Developer ID notarized. Signed update feeds must be
published before they can serve updates. Renamed standalone Windows executables
use manual updates. Existing uploaders, accounts and mounts are not taken over
automatically.

## 验证与发行

构建目标为 Windows amd64/arm64、Linux amd64 和 macOS amd64/arm64，实际运行证据见 README。
macOS 人工界面、真实云账户登录、休眠唤醒和系统签名分别验收。
Windows 尚无 Authenticode 签名，macOS 尚无 Developer ID 公证。
签名更新源发布后才能提供更新；重命名的 Windows 单独 exe 使用手动更新。
客户端不会自动接管已有上传器、账户或挂载。
