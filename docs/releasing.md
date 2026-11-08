# Releases / 发行

## English

The application version lives in `internal/version/version.go`, with the same
value in `mygo.json` and `release-notes.md`. Run
`python scripts/check_repository.py --tag v<version>` before creating a tag.
Keep release notes bilingual and explicit about validated platforms and signing.
Keep the same release section in `CHANGELOG.md` when signing MyGo updates.

Pushes, pull requests and version tags do not trigger Actions. Routine
development uses focused local checks. To build a release, create and push the
version tag, then manually run **Release packages** in Actions with that existing
`tag`. Leave `require_os_signing` off for an explicitly unsigned alpha, or turn
it on after configuring trusted signing credentials.

A cheap prepare job resolves the tag to an exact commit, rejects already
published releases and checks public sources, version and documentation once.
Native Windows amd64/arm64, Linux amd64 and macOS amd64/arm64 jobs all check out
that commit. Each runs its full suite once with a temporary verified rclone
engine; Linux uses the race detector. The macOS jobs explicitly select Intel
and Apple Silicon runners. Every target has its own artifact name and update
manifest. No automatic full CI/CD or release publication is configured.

Build locally with `python scripts/build.py --platform windows/amd64 --installer`
or choose `windows/arm64`, `linux/amd64`, `darwin/amd64` or `darwin/arm64`.
macOS disk images and signing require a macOS host.
Signed release archives require the matching operating-system host so executable
permissions are preserved. Unsigned cross-builds are compilation checks; their
successful compilation does not qualify them as installable update archives.
`scripts/package.py --platform <target>` prepares no-install artifacts,
SHA256SUMS and the Go/native dependency inventory.
`scripts/check_native_package.py --platform <target>` runs the built CLI on the
matching native architecture and verifies its version and embedded licenses.
The release workflow requires each platform's recommended installer/package,
checks executable architecture and archive permissions, and then verifies signed
update archives. Native CLI validation does not replace manual GUI installation.

The Release workflow checks version, tests and archive contents, then assembles
a GitHub **draft release** (a prerelease for alpha/beta tags). Only the draft job
has `contents: write`; prepare and package jobs are read-only. Signing inputs
are confined to their prepare/build steps. Check installation, actual
runtime, hashes and release notes before publishing the draft. Signing and
notarization are separate from hash checks and remain optional, explicitly
reported capabilities in the alpha.

Do not overwrite published tags or silently replace public artifacts. Cut a new
version for corrections. GitHub artifacts expire after fourteen days; the release
holds accepted deliverables. rclone versions and client versions remain independent.

### Installation and platform signing

The recommended packages are per-user NSIS installers on Windows, DMGs on macOS
and DEBs on Debian/Ubuntu. Windows ZIP/standalone executables and macOS application
ZIPs are secondary no-install formats. User data lives outside the installed
application. Use the [installation guide](install.md) for the user-facing steps.

Before publishing, install a package on a clean profile, launch it, upgrade it,
uninstall it and reinstall it. Verify retained preferences/accounts/history,
shortcuts and startup-item cleanup. An active transfer or mount must not be
silently killed by an installer or update restart. Perform these checks using
synthetic data and an independent rclone configuration.

`scripts/build.py` accepts these optional private signing inputs and writes the
corresponding MyGo configuration only into a disposable source snapshot:

| Environment input | Use |
| --- | --- |
| `RCLONEGO_WINDOWS_CERTIFICATE` | Path to the code-signing PFX |
| `MYGO_WINDOWS_CERTIFICATE_PASSWORD` | Password for that PFX |
| `RCLONEGO_WINDOWS_SIGN_COMMAND` | Alternative official MyGo signing command, with `%1` for the file |
| `RCLONEGO_MACOS_SIGNING_IDENTITY` | Developer ID Application identity available in Keychain |
| `RCLONEGO_MACOS_NOTARY_PROFILE` | Existing notarytool Keychain credentials profile |
| `RCLONEGO_MACOS_NOTARY_KEYCHAIN` | Optional Keychain path for that profile |

Choose the Windows certificate or signing command, not both. Keep certificates
and credentials outside the source tree. Sign the application, installer and
embedded uninstaller. On macOS provision the Developer ID certificate and notary
credentials into the build host's Keychain before building.
Add `--require-os-signing` to require real inputs and native signature validation
instead of producing an unsigned alpha. Windows checks Authenticode; macOS checks
codesign, Gatekeeper assessment and the DMG's stapled ticket.
MyGo's default macOS ad-hoc signing is not Developer ID signing or notarization.
Report missing OS signing explicitly rather than treating update signatures as
equivalent.

For the manually dispatched hosted release, configure these repository secrets:

| Secret | Use |
| --- | --- |
| `MYGO_UPDATER_PRIVATE_KEY` | Required update archive signing key |
| `WINDOWS_CERTIFICATE_PFX_BASE64`, `WINDOWS_CERTIFICATE_PASSWORD` | Windows certificate and password; alternatively use `WINDOWS_SIGN_COMMAND` |
| `MACOS_CERTIFICATE_P12_BASE64`, `MACOS_CERTIFICATE_PASSWORD`, `MACOS_SIGNING_IDENTITY` | Developer ID certificate and identity |
| `APPLE_API_PRIVATE_KEY`, `APPLE_API_KEY_ID`, `APPLE_API_ISSUER_ID` | App Store Connect API credentials for notarization |

With `require_os_signing` selected, `scripts/release_signing.py prepare` imports
only the current platform's inputs into the isolated runner's temporary storage
or Keychain, then the build requires and verifies OS signing. Its cleanup runs
even after a failure. Missing required credentials fail the job; leaving the
option off does not load OS-signing secrets. Update signing remains required.

### MyGo signed updates

RcloneGo uses the official
[MyGo native updater](https://mygo.egoist.dev/docs/updates), including its update
window, signature verification, archive installation and delta fallback.
The application offers stable/preview channels and keeps automatic checks off
until the user enables them. New alpha/beta profiles default to preview; stored
choices are kept. Preview uses `Updater.SetPrereleases(true)`; switching
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
   Only manually dispatched release build steps receive the secret.
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

push、PR 和版本 tag 都不触发 Actions，日常开发执行针对改动的本地检查。
需要发行包时，先创建并推送版本 tag，再到 Actions 手动运行“Release packages”，
填写已有 `tag`。当前明确未签名的 alpha 可关闭 `require_os_signing`；
配置可信签名凭据后再开启。
签名发行归档必须在对应操作系统生成，以保留可执行权限。
无签名的交叉构建用于编译检查，不能当成可安装更新包。

轻量 prepare job 先把 tag 解析到准确提交，拒绝已公开的不可变发行，统一检查一次
公开源码、版本与文档。Windows amd64/arm64、Linux amd64、macOS amd64/arm64 原生 job
都检出该提交，各执行一次完整测试，再打包。测试使用临时且经过校验的 rclone，
Linux 运行 race；macOS 明确区分 Intel 与 Apple Silicon。每个目标使用独立 artifact
名称和更新 manifest，不配置自动全量 CI/CD 或自动公开发行。

本机使用 `python scripts/build.py --platform windows/amd64 --installer`，
或选择 `windows/arm64`、`linux/amd64`、`darwin/amd64`、`darwin/arm64`。
macOS 磁盘镜像和签名需要 macOS 主机。
`scripts/package.py --platform <target>` 整理免安装产物、SHA256SUMS 和 Go/原生依赖清单。
`scripts/check_native_package.py --platform <target>` 在对应原生架构运行打包后的 CLI，
核对版本与内嵌许可。发行流程要求各平台推荐的安装器/包，核对二进制架构、归档内容和
执行权限，再验证签名更新归档。原生 CLI 验证不替代人工图形界面安装验收。

发布工作流核验版本、测试和归档内容，再组装 GitHub **draft release**，
alpha/beta tag 标记为 prerelease。
只有草稿 job 有 `contents: write` 权限，prepare 与 package job 只读。
签名输入只进入对应的准备/构建步骤。
安装、真实运行、校验和与说明复核后再公开草稿。签名、公证与哈希是不同的检查，
alpha 按实际能力标注。

已发布 tag 和产物不覆盖，修正问题通过新版本交付。
Actions artifact 保留十四天，Release 保存验收过的发行包；rclone 引擎与客户端分别版本化。

### 安装与系统签名

Windows 推荐当前用户 NSIS 安装器，macOS 推荐 DMG，Debian/Ubuntu 推荐 DEB。
Windows ZIP/单独 exe 和 macOS app ZIP 是次级免安装格式。用户数据与程序安装目录分开，
使用步骤见[安装指南](install.md)。

公开发行前，用空白隔离配置完成安装、启动、升级、卸载和重新安装，核对偏好/账户/历史
保留、快捷方式与启动项清理。安装器或更新重启不能静默强制结束活动传输与挂载。
验收使用合成数据及独立 rclone 配置。

`scripts/build.py` 接受上表的私人环境输入，只在临时源码副本中写入对应的 MyGo 配置。
Windows 使用 `RCLONEGO_WINDOWS_CERTIFICATE` 指定 PFX，密码通过
`MYGO_WINDOWS_CERTIFICATE_PASSWORD` 提供；或改用 `RCLONEGO_WINDOWS_SIGN_COMMAND`，
两者不能同时设置。程序、安装器与内嵌卸载器都需要签名。
macOS 使用 `RCLONEGO_MACOS_SIGNING_IDENTITY`、`RCLONEGO_MACOS_NOTARY_PROFILE` 和可选
`RCLONEGO_MACOS_NOTARY_KEYCHAIN`；构建前先在 Keychain 中配置证书与公证凭据。
添加 `--require-os-signing` 会要求真实签名输入并执行原生验证，缺失时失败。
Windows 检查 Authenticode，macOS 检查 codesign、Gatekeeper 和 DMG 的公证 ticket。
证书与凭据不放源码树。MyGo 默认的 ad-hoc 签名不等于 Developer ID 或公证，
缺失系统签名时明确说明，不用更新包签名代替。

手动托管发行使用上表的 GitHub repository secrets：Windows 为 PFX 与密码，或替代的
签名命令；macOS 为 Developer ID P12、密码、身份及 Apple API 公证凭据。
开启 `require_os_signing` 后，`scripts/release_signing.py prepare` 只把当前平台的
凭据放入隔离 runner 临时目录或 Keychain，再要求构建和验证系统签名。
失败后也执行清理。必要凭据缺失时失败；关闭该选项不加载系统签名 secrets。
更新包签名仍为必需。

### MyGo 签名更新

更新窗口、归档校验、安装和差量失败回退使用
[MyGo 官方实现](https://mygo.egoist.dev/docs/updates)。
客户端提供稳定版/预览版通道，自动检查由用户开启。新的 alpha/beta 配置默认使用预览通道，
已有选择保留。
预览通道使用 `Updater.SetPrereleases(true)`；切回稳定版等待更新的稳定版本，
不会降级当前应用。
Windows 安装版/解压版保留官方 `RcloneGo.exe` 文件名；单独下载、文件名带版本的
exe 使用发行页手动更新，避免更新归档写入 `RcloneGo.exe` 后仍重启旧文件。

使用固定版本的官方 `mygo keygen`，把密钥写到仓库外的私人目录。
仅将公钥写入 `mygo.json` 的 `updates.publicKey`，配置
`github: DeliciousBuding/rclonego`、`changelog: CHANGELOG.md` 和所需差量版本数。
私钥放 GitHub Actions secret `MYGO_UPDATER_PRIVATE_KEY`。
当前手动发行流程要求签名更新产物，私钥只进入手动发行的构建步骤。

`scripts/build.py --require-signed-updates` 会要求 MyGo 生成签名更新归档，
核验公钥签名并保留官方 manifest 引用的原始文件名。
缺失 manifest、版本不符、文件缺失、大小不符和签名错误均阻止发行。
`SHA256SUMS` 同时覆盖更新 manifest。尚未配置签名的构建使用手动更新，
不会把哈希校验当成自动更新签名。

下载更新不会停止运行中的应用。活动任务与本实例拥有的挂载会推迟更新重启；
用户仍可显式退出。预览版标记为 prerelease，稳定版须在 GitHub 标记为非预览版。
MyGo 更新签名与 Windows Authenticode、macOS Developer ID/公证分别处理。
