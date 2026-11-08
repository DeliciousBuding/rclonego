# Releases / 发行

## English

The application version lives in `internal/version/version.go`, with the same
value in `mygo.json` and `release-notes.md`. Run
`python scripts/check_repository.py --tag v<version>` before creating a tag.
Keep release notes bilingual and explicit about validated platforms and signing.
Keep the same release section in `CHANGELOG.md` when signing MyGo updates.

Normal CI tests main and PRs on Windows, Linux and macOS. Every job uses a
temporary verified rclone engine; Linux runs the race detector. A `v*` tag starts
platform packaging. Manual workflow dispatch builds artifacts without creating
a release.

Build locally with `python scripts/build.py --platform windows/amd64 --installer`
or choose `linux/amd64` / `darwin/arm64`. macOS disk images and signing require
a macOS host. `scripts/package.py --platform <target>` prepares portable artifacts,
SHA256SUMS and the Go/native dependency inventory.

The Release workflow checks version, tests and archive contents, then assembles
a GitHub **draft release** (a prerelease for alpha/beta tags). Only the draft job has `contents: write`; PR jobs
are read-only and never receive signing credentials. Check installation, actual
runtime, hashes and release notes before publishing the draft. Signing and
notarization are separate from hash checks and remain optional, explicitly
reported capabilities in the alpha.

Do not overwrite published tags or silently replace public artifacts. Cut a new
version for corrections. GitHub artifacts expire after fourteen days; the release
holds accepted deliverables. rclone versions and client versions remain independent.

### MyGo signed updates

RcloneGo uses the official
[MyGo native updater](https://mygo.egoist.dev/docs/updates), including its update
window, signature verification, archive installation and delta fallback.
The application offers stable/preview channels and keeps automatic checks off
until the user enables them. Preview uses `Updater.SetPrereleases(true)`; switching
back to stable waits for a newer stable version and does not downgrade the app.

Signing setup is per project:

1. Run the pinned official `go run github.com/egoist/mygo/cmd/mygo keygen -o <private-directory>`.
   Keep the private directory outside the repository and retain the signing key.
2. Copy only the public key into `mygo.json`:

   ```json
   {
     "updates": {
       "publicKey": "<contents of mygo-update.pub>",
       "github": "DeliciousBuding/rclonego",
       "changelog": "CHANGELOG.md",
       "deltas": 3
     }
   }
   ```

3. Store the private key as the GitHub Actions secret `MYGO_UPDATER_PRIVATE_KEY`.
   The configured release workflow requires signed update artifacts.
   Only release build steps receive the secret; ordinary CI and PRs do not.
4. Build with `python scripts/build.py --platform <target> --require-signed-updates`.
   MyGo writes signed archives and manifests. The script verifies every archive
   and delta with the configured public key and copies the original named outputs
   into `dist/`; assembly retains these names.

Release checks fail on a missing required manifest, wrong version, missing file,
size mismatch or signature mismatch. `SHA256SUMS` covers the signed manifests too.
Unconfigured builds remain manual-update builds; hashes do not enable signed
updates. Published prereleases serve the preview channel, while published stable
versions must be marked as non-prereleases in GitHub.
On Windows, the installed/extracted executable keeps the official `RcloneGo.exe`
name. The separately downloaded version-named `.exe` uses manual release-page
updates: MyGo's archive installs `RcloneGo.exe` and cannot replace/relaunch an
arbitrarily renamed running executable.

Downloads do not stop an active process. Update-triggered relaunch waits for
active transfers and this instance's mounts to become idle; explicit application
quit remains available. MyGo update signing does not provide Windows Authenticode
or macOS Developer ID signing/notarization.

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

发布工作流核验版本、测试和归档内容，再组装 GitHub **draft release**，
alpha/beta tag 标记为 prerelease。
只有草稿 job 有 `contents: write` 权限，PR job 只读，不接入签名凭据。
安装、真实运行、校验和与说明复核后再公开草稿。签名、公证与哈希是不同的检查，
alpha 按实际能力标注。

已发布 tag 和产物不覆盖，修正问题通过新版本交付。
CI artifact 保留十四天，Release 保存验收过的发行包；rclone 引擎与客户端分别版本化。

### MyGo 签名更新

更新窗口、归档校验、安装和差量失败回退使用
[MyGo 官方实现](https://mygo.egoist.dev/docs/updates)。
客户端提供稳定版/预览版通道，自动检查由用户开启。
预览通道使用 `Updater.SetPrereleases(true)`；切回稳定版等待更新的稳定版本，
不会降级当前应用。
Windows 安装版/解压版保留官方 `RcloneGo.exe` 文件名；单独下载、文件名带版本的
exe 使用发行页手动更新，避免更新归档写入 `RcloneGo.exe` 后仍重启旧文件。

使用固定版本的官方 `mygo keygen`，把密钥写到仓库外的私人目录。
仅将公钥写入 `mygo.json` 的 `updates.publicKey`，配置
`github: DeliciousBuding/rclonego`、`changelog: CHANGELOG.md` 和所需差量版本数。
私钥放 GitHub Actions secret `MYGO_UPDATER_PRIVATE_KEY`。
当前发行流程要求签名更新产物；普通 CI 与 PR 不接入签名私钥。

`scripts/build.py --require-signed-updates` 会要求 MyGo 生成签名更新归档，
核验公钥签名并保留官方 manifest 引用的原始文件名。
缺失 manifest、版本不符、文件缺失、大小不符和签名错误均阻止发行。
`SHA256SUMS` 同时覆盖更新 manifest。尚未配置签名的构建使用手动更新，
不会把哈希校验当成自动更新签名。

下载更新不会停止运行中的应用。活动任务与本实例拥有的挂载会推迟更新重启；
用户仍可显式退出。预览版标记为 prerelease，稳定版须在 GitHub 标记为非预览版。
MyGo 更新签名与 Windows Authenticode、macOS Developer ID/公证分别处理。
