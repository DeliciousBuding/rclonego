# Changelog

## 0.1.0-alpha.3 - 2026-11-08

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
- Follow system preferences on first launch and clearly show language overrides.
- Unify profile selection across GUI/CLI and keep profile-specific startup disabled.
- Add safe per-user Windows installation, owned-startup cleanup and data retention.
- Run release checks and native packaging only on manual Actions dispatch, pinned
  to one commit and producing an unpublished draft.
- Align page headings and primary actions, move secondary workflow actions to
  More, and use Mica for the Windows main window.
- Clarify activity phases, use file counts for verification, show mount routes
  and keep empty history and task details concise.
- Keep task actions visible, explain input issues at their fields and present
  results with verification evidence and recovery. Retain critical engine errors.
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
- 首次启动跟随系统偏好，语言选择明确区分自动匹配和手动指定。
- GUI/CLI 使用统一配置目录，独立配置不误登记默认配置的登录启动。
- Windows 按当前用户安全安装，清理所属启动项并保留用户数据。
- 只有手动 Actions 发行才运行完整检查与原生打包，固定同一提交，生成未公开草稿。
- 统一页面标题与主操作，工作流低频操作收进“更多”，Windows 主窗口采用 Mica。
- 区分活动阶段，校验按文件数显示进度；补齐映射路径，精简空历史与任务详情。
- 固定任务操作区，就地显示输入问题；结果窗口展示校验依据与恢复入口，保留引擎关键错误。
- 展示本实例挂载的 VFS 缓存与写回状态，支持独立离线副本任务。
- 完善 Agent 契约、事件流和可撤销的任务权限。

## 0.1.0-alpha.2

### English

Native component polish, provider logos, bilingual project documentation,
embedded notices, bounded history and verified public preview packages.

### 简体中文

打磨原生组件和服务标志，完善双语项目文档、内嵌许可、历史保留与公开预览包校验。
