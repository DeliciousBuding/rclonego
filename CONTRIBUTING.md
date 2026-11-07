# Contributing / 参与贡献

[English](#english) · [简体中文](#简体中文)

## English

Start with a reproducible problem or a concrete storage workflow. Small fixes
can be submitted directly; use an issue to discuss changes to task semantics,
account authorization or persisted state before a large implementation.

Read [architecture](docs/architecture.md) and [API](docs/api.md). GUI, CLI and API
must use one application service and scheduler. Use the existing components,
native MyGo APIs and bounded state instead of adding a second execution path.

Run `gofmt`, `go test ./...`, `go vet ./...` and
`python scripts/check_public.py --tracked`. `python scripts/test.py` supplies a
verified temporary rclone engine for integration tests. If the API changes,
regenerate `python scripts/update_contract.py`.

For native UI changes, inspect a real window with synthetic data, narrow and
regular widths, light and dark appearance, long labels and display scaling.
Headless layout tests complement this inspection. A cross-build does not
establish platform runtime support.

Use temporary data and an isolated configuration. Do not test on personal
accounts, running transfers or existing mounts. Source, examples, screenshots,
logs and release artifacts must contain no credentials or private environment
information. Security reports belong in the private
[advisory form](https://github.com/DeliciousBuding/rclonego/security/advisories/new).

Keep a PR focused on one result. Describe the before/after behavior and relevant
verification. Note AI or automation assistance when used. Contributions are
licensed under Apache-2.0; preserve imported assets' original notices and
trademarks. The [release guide](docs/releasing.md) describes packaging.

## 简体中文

先说明可复现的问题或具体的存储工作流。小修复可以直接提交；涉及任务语义、
账户授权或持久状态的大改动，先通过 Issue 说明方案与兼容影响。

阅读[架构](docs/architecture.md)与 [API](docs/api.md)。GUI、CLI 和 API
共用应用服务与调度器。沿用已有组件、MyGo 原生能力和有界状态，避免新增第二条执行链。

提交前运行 `gofmt`、`go test ./...`、`go vet ./...` 与
`python scripts/check_public.py --tracked`。`python scripts/test.py`
会提供经过校验的临时 rclone 引擎。修改 API 后运行
`python scripts/update_contract.py` 同步契约。

原生 UI 修改需要用合成数据检查真实窗口：窄窗口与常规窗口、浅深主题、
长标签和显示缩放。无窗口布局测试作为补充。跨平台构建成功不等于实际运行通过。

测试使用临时目录与隔离配置，不操作私人账户、活动传输和已有挂载。
源码、示例、截图、日志和发行产物不得包含凭据或私人环境信息。安全问题通过
[私密报告入口](https://github.com/DeliciousBuding/rclonego/security/advisories/new)反馈。

每个 PR 围绕一个结果，写明修改前后行为和相关验证。使用 AI 或自动化时说明参与部分。
贡献采用 Apache-2.0，保留引入资产的许可与商标说明。
打包方式见[发行指南](docs/releasing.md)。
