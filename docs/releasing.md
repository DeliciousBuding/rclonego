# Releases / 发行

## English

The application version lives in `internal/version/version.go`, with the same
value in `mygo.json` and `release-notes.md`. Run
`python scripts/check_repository.py --tag v<version>` before creating a tag.
Keep release notes bilingual and explicit about validated platforms and signing.

Normal CI tests main and PRs on Windows, Linux and macOS. Every job uses a
temporary verified rclone engine; Linux runs the race detector. A `v*` tag starts
platform packaging. Manual workflow dispatch builds artifacts without creating
a release.

Build locally with `python scripts/build.py --platform windows/amd64 --installer`
or choose `linux/amd64` / `darwin/arm64`. macOS disk images and signing require
a macOS host. `scripts/package.py --platform <target>` prepares portable artifacts,
SHA256SUMS and the Go/native dependency inventory.

The Release workflow checks version, tests and archive contents, then assembles
a GitHub **draft prerelease**. Only the draft job has `contents: write`; PR jobs
are read-only and never receive signing credentials. Check installation, actual
runtime, hashes and release notes before publishing the draft. Signing and
notarization are separate from hash checks and remain optional, explicitly
reported capabilities in the alpha.

Do not overwrite published tags or silently replace public artifacts. Cut a new
version for corrections. GitHub artifacts expire after fourteen days; the release
holds accepted deliverables. rclone versions and client versions remain independent.

## 简体中文

应用版本位于 `internal/version/version.go`，同时更新 `mygo.json` 与
`release-notes.md`。创建 tag 前运行
`python scripts/check_repository.py --tag v<version>`。
发行说明保留中英文，并写明实际验证的平台与签名状态。

常规 CI 对 main 和 PR 做 Windows、Linux、macOS 测试。每个 job
使用临时且经过校验的 rclone，Linux 运行 race。`v*` tag 触发平台打包；
手动运行仅生成产物，不创建 Release。

本机使用 `python scripts/build.py --platform windows/amd64 --installer`，
或选择 `linux/amd64` / `darwin/arm64`。macOS 磁盘镜像和签名需要 macOS 主机。
`scripts/package.py --platform <target>` 整理便携产物、SHA256SUMS 和 Go/原生依赖清单。

发布工作流核验版本、测试和归档内容，再组装 GitHub **draft prerelease**。
只有草稿 job 有 `contents: write` 权限，PR job 只读，不接入签名凭据。
安装、真实运行、校验和与说明复核后再公开草稿。签名、公证与哈希是不同的检查，
alpha 按实际能力标注。

已发布 tag 和产物不覆盖，修正问题通过新版本交付。
CI artifact 保留十四天，Release 保存验收过的发行包；rclone 引擎与客户端分别版本化。
