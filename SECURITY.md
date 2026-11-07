# Security / 安全

The latest alpha is the maintained preview. Stable releases are not available
yet. Upgrade to the current preview before reporting an already fixed issue.

Report credential exposure, authorization bypass, unsafe paths or process
ownership issues privately through
[GitHub security advisories](https://github.com/DeliciousBuding/rclonego/security/advisories/new).
Include a minimal synthetic reproduction, affected version and platform.
Do not post real tokens, OAuth responses, configuration files or private
filenames in a public issue. Response times depend on maintainer availability.

The loopback owner API token grants the local user's task and configuration
access. Use an expiring read-only token for observation. Mount drivers, storage
accounts and OAuth clients have independent update and permission boundaries.
See [privacy](docs/privacy.md) for diagnostics and reporting.

目前维护最新 alpha 预览版，尚无稳定版。报告问题前先确认最新版本是否已经修复。

凭据暴露、授权绕过、路径越界和进程归属问题，请通过
[GitHub 私密安全报告](https://github.com/DeliciousBuding/rclonego/security/advisories/new)反馈。
提供最小合成复现、受影响版本和平台；不要在公开 Issue 中粘贴真实 token、
OAuth 响应、配置或私人文件名。响应时间取决于维护者的可用时间。

本地 owner API token 具有当前用户的任务与配置权限，观察程序请使用有期限的只读 token。
挂载驱动、存储账户和 OAuth 客户端分别管理更新与权限。
诊断和隐私边界见[隐私说明](docs/privacy.md)。
