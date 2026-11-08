# RcloneGo 0.1.0-alpha.3

### English

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
- Align page headings and primary actions, move secondary workflow actions to
  More, and use Mica for the Windows main window.
- Clarify activity phases, use file counts for verification, show mount routes
  and keep empty history and task details concise.
- Observe owned VFS cache/writeback state and queue persistent offline copies.
- Extend Agent contracts, event streaming and revocable task permissions.

### 简体中文

- 接入 MyGo 官方原生更新窗口、签名归档校验和稳定版/预览版通道。
  活动传输与本实例拥有的挂载会推迟更新重启。
- 创建发行草稿前核验更新归档与差量包签名。
- 完善持久化恢复、存储失败时停止调度、进程归属校验与任务日志限制。
- 新增云端目录浏览、结构化预览、工作流编辑/复制/导入导出和历史分页。
- 最终校验遵循所选步骤范围；目录监听和周期工作流复用同一队列。
- 使用 MyGo 官方 Glass 和语言目录，加入命令搜索与后台刷新降频。
- 设置页统一管理主题、语言、登录启动与更新，引擎和新任务默认参数按需展开。
- 统一页面标题与主操作，工作流低频操作收进“更多”，Windows 主窗口采用 Mica。
- 区分活动阶段，校验按文件数显示进度；补齐映射路径，精简空历史与任务详情。
- 展示本实例挂载的 VFS 缓存与写回状态，支持独立离线副本任务。
- 完善 Agent 契约、事件流和可撤销的任务权限。

## Validation and distribution

Build packages target Windows amd64, Linux amd64 and macOS arm64. Runtime
validation is reported in README; macOS manual GUI, actual cloud sign-in,
sleep/wake and OS signing are separate checks. Windows is not Authenticode
signed and macOS is not Developer ID notarized. Signed update feeds must be
published before they can serve updates. Renamed standalone Windows executables
use manual updates. Existing uploaders, accounts and mounts are not taken over
automatically.

## 验证与发行

构建目标为 Windows amd64、Linux amd64 和 macOS arm64，实际运行证据见 README。
macOS 人工界面、真实云账户登录、休眠唤醒和系统签名分别验收。
Windows 尚无 Authenticode 签名，macOS 尚无 Developer ID 公证。
签名更新源发布后才能提供更新；重命名的 Windows 单独 exe 使用手动更新。
客户端不会自动接管已有上传器、账户或挂载。
