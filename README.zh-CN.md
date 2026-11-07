<p align="center"><img src="assets/wordmark.svg" width="400" alt="RcloneGo"></p>
<p align="center">可靠传输，过程可见。</p>
<p align="center">
  <a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> ·
  <a href="https://github.com/DeliciousBuding/rclonego/releases">下载</a> ·
  <a href="docs/agents.md">Agent API</a>
</p>
<p align="center">
  <a href="https://github.com/DeliciousBuding/rclonego/actions/workflows/check.yml"><img src="https://github.com/DeliciousBuding/rclonego/actions/workflows/check.yml/badge.svg" alt="CI"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache--2.0-318b9d" alt="Apache-2.0"></a>
  <a href="https://mygo.egoist.dev/"><img src="https://img.shields.io/badge/native_UI-MyGo-318b9d" alt="Built with MyGo"></a>
</p>

RcloneGo 是管理传输、挂载与校验的原生桌面客户端，由
[rclone](https://rclone.org/) 驱动，使用 [MyGo](https://mygo.egoist.dev/)
构建。图形界面、CLI 和经过认证的本地 API 共用一个持久化调度器。

项目处于 alpha 开发阶段，是独立社区项目，不代表 rclone 或 MyGo 官方。

## 主要能力

- **可核验的传输。** 复制、单文件复制、单向镜像与校验。保存工作流，
  每次执行生成独立 Run、顺序步骤和尝试记录。
- **可观察与控制的过程。** 当前目录和文件、资源计数、并发与限速、计划时间、
  暂停队列，以及暂停和恢复当前任务。
- **完整的存储接入流程。** 校验官方引擎、离线导入、按服务元数据逐步配置、
  浏览器 OAuth 和读取验证。挂载由记录的进程归属管理。
- **人类与 Agent 共用。** 原生浅色、深色、跟随系统主题，托盘后台运行，
  CLI JSON、离线 OpenAPI、事件游标和只读观察 token。

镜像同步要求先 dry-run 预览，再明确授权删除。报告区分大小、可用哈希与下载内容对比；
复制完成不等于内容校验完成。

## 安装

从 [Releases](https://github.com/DeliciousBuding/rclonego/releases) 选择发行包。
预览版标为 prerelease，目前尚无稳定版。

| 平台 | 包形式与验证范围 |
| --- | --- |
| Windows amd64 | 单文件便携 exe 或安装器；已验证原生界面与打包后的 CLI |
| Linux amd64 | `.deb` 或 `.tar.gz`；已验证 Ubuntu WSLg、传输及 FUSE 挂载读取与卸载 |
| macOS arm64 | `.dmg` 或 app ZIP；构建与合成 CI 测试通过，人工原生验收待完成 |

其他 amd64/arm64 组合可交叉构建，实际运行支持仍待验证。
Windows 包尚无 Authenticode 签名，macOS 包尚无 Developer ID 公证。
SHA256SUMS 用于校验产物完整性，不能替代系统签名。

Windows 便携模式只需一个 exe。SQLite 已嵌入程序，首次使用时释放到经过校验的私人缓存。
Linux 使用随包的安装脚本或发行版包。

在“运行与资源”中下载推荐的官方 rclone、选择已有引擎，或离线导入经过校验的 ZIP。
客户端不修改 PATH，也不覆盖包管理器安装。挂载组件单独安装：
Windows 使用 WinFsp，Linux 使用 FUSE，macOS 需要兼容的 FUSE 与引擎构建。

## 开始第一个工作流

1. 启动 RcloneGo，安装或选择引擎。
2. 添加云端账户，或直接使用本机目录。Google Drive 使用自己的 OAuth Client ID，
   见[账户接入](docs/accounts.md)。
3. 创建复制任务，或保存带最终校验的目录工作流。
4. 在队列观察过程，完成后查看执行报告。

桌面支持托盘时，关闭窗口会继续后台运行，选择“退出”才停止服务。
没有托盘时，关闭窗口会退出。重启恢复会重新比较源与目标；
文件内部能否从断点续传取决于后端。

## CLI 与 Agent

```sh
rclonego --help
rclonego schema
rclonego status --json
rclonego engine install
rclonego workflows save --file workflow.json --json
rclonego workflows run <workflow-id> --json
rclonego runs wait <run-id> --timeout 1h --json
rclonego runs report <run-id> --json
```

CLI 自动发现与界面相同的本地实例。隔离开发配置使用 `RCLONEGO_DATA`，
私人 token 文件通过 `RCLONEGO_TOKEN_FILE` 指定。
Ctrl+C 只停止等待，不取消后台任务。

详见 [Agent 指南](docs/agents.md)、[API](docs/api.md)、
[架构](docs/architecture.md)与[隐私说明](docs/privacy.md)。
自动同步、连接独立 daemon 的 GUI、MCP 和带签名的自动客户端更新仍是后续工作。

## 开发与贡献

需要 Go 1.27.3+ 与 Python 3.12+。原生 UI 不需要 JavaScript 工具链。

```sh
go run ./cmd/rclonego
go test ./...
go vet ./...
python scripts/test.py
python scripts/build.py --platform windows/amd64
```

`scripts/test.py` 提供经过校验的临时集成测试引擎。
Windows 构建添加 `--installer` 生成安装器。
`-data`、`-rclone` 和 `-config` 分别指定私人状态、引擎与配置；
默认使用空白应用配置，不自动接管已有上传。

贡献和界面验收见 [CONTRIBUTING](CONTRIBUTING.md)，
打包见[发行指南](docs/releasing.md)，私密安全报告见 [SECURITY](SECURITY.md)。
源码采用 [Apache-2.0](LICENSE)，依赖与服务标志保留[各自的许可](docs/third-party.md)。
