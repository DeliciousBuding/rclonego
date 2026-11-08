# Install, update and uninstall / 安装、更新与卸载

For Data Ops Console users, **Settings → Legacy client handoff** previews an
import of `state.json` and the explicitly selected rclone configuration.
Use a profile with no task history. Beginning the handoff pauses old dispatch
without stopping its current transfer, copies private account configuration,
bounded logs and exclusion evidence, and preserves task IDs and step order.
Mounts retain their original owner. The old client must remain open until its
active step finishes and records an exit result. The new queue then continues
unfinished steps; a failed copy still blocks final verification.

“Return to previous client” is available before this client starts imported
work. After return, use a fresh empty profile for another import. A crash or
lost pause response may leave the old queue paused: read the handoff status and
resume the old client explicitly if no import was committed. Neither process
disappearance nor a progress bar reaching 100% proves completion.

Data Ops Console 用户可在**设置 → 旧客户端接管**中选择 `state.json` 和原 rclone
配置，先预览，再导入到没有任务历史的配置目录。开始交接只暂停旧队列的后续派发，
当前上传继续，任务顺序、私有配置、有界日志和排除证明会保留。
原挂载仍由原程序管理。旧客户端需保持打开，直到当前步骤记录最终退出结果；
新队列随后接续未完成步骤，复制失败仍会阻止最终校验。
新客户端启动过导入任务后不再允许直接交还。交还后若需重新导入，使用新的空白配置。
如果交接期间崩溃或暂停响应丢失，旧队列可能保持暂停，应先查询交接状态；
没有提交导入时可在旧客户端手动恢复队列。

## English

Download from [GitHub Releases](https://github.com/DeliciousBuding/rclonego/releases).
Choose the architecture of the computer that will run the app. Each release lists
its available packages, signing status and platform validation. Alpha releases
are prereleases; a configured build target does not mean a package is published.

### Windows

Use `rclonego-setup-<version>-amd64.exe` for most PCs, or the `arm64` installer for
Windows on ARM. Run it and follow the short installation wizard. The default is
a per-user installation in `%LOCALAPPDATA%\Programs\RcloneGo`; administrator
rights are not required for the app. The installer creates a Start menu entry,
offers a desktop shortcut and registers an uninstaller in Windows Settings.

The ZIP is a no-install alternative: extract it into a writable folder and run
`RcloneGo.exe`. Its default settings still live in the user profile. Moving only
the executable does not move accounts and task history. A separately downloaded
version-named executable uses manual updates; the installed/extracted
`RcloneGo.exe` can use the official updater.

Before upgrading or uninstalling, select **Quit RcloneGo** from the tray menu.
Closing the window can leave transfers running in the background. Uninstall
through **Settings → Apps → Installed apps**, rather than deleting the executable.
The installer and uninstaller refuse to continue while a process from their
installation directory is running; they never force a transfer to stop.
Uninstalling the app keeps its user data so that reinstalling can reuse accounts
and task history.

### macOS

Choose the `arm64` DMG for Apple Silicon or the `amd64` DMG for an Intel Mac:

1. Open the DMG.
2. Drag **RcloneGo** to **Applications**.
3. Eject the disk image.
4. Open RcloneGo from Applications.

Run the installed application rather than the copy on the read-only disk image.
The updater needs permission to replace the installed app; when its location is
not writable, download the next DMG and replace the application after quitting.
The application ZIP is an alternative delivery format; extract the `.app` and
move it to Applications.

To uninstall, quit RcloneGo, disable **Settings → Login at startup** if enabled,
and move `RcloneGo.app` to Trash. The separate user-data directory remains.
For unsigned alpha packages, macOS may require explicit user approval to open
the app. Use Apple's [open an app from an unidentified developer guidance](https://support.apple.com/guide/mac-help/open-a-mac-app-from-an-unidentified-developer-mh40616/mac);
do not disable Gatekeeper.

### Linux

On Debian/Ubuntu, install the DEB using the system software installer or:

```sh
sudo apt install ./rclonego-<version>-linux-amd64.deb
```

Use the package manager for upgrades and removal:

```sh
sudo apt remove rclonego
```

The tar.gz is for manual per-user installation. Extract it to a writable
application folder, keep its bundled files together and run `rclonego`.
Package-manager installations use the package manager for updates; writable
user installations can use the in-app updater.

### First launch and data

The app starts with the system theme and best supported system language.
**Settings** can override either immediately. Login startup is opt-in; a launch
at login stays in the tray/menu bar when supported. Open the app again to show
its existing window. **Ctrl+,** / **Cmd+,** opens Settings; **Ctrl+Q** / **Cmd+Q**
quits the application. Independent `-data` profiles do not register login startup,
because the system startup entry must open the same default profile.
Opening the same profile again activates its existing window through MyGo;
independent profiles keep their own state and window.

Install the recommended rclone engine from **Settings → rclone engine**, select
an existing engine, or import its verified ZIP offline. The engine is kept
separate from the client and does not replace a system/package-manager rclone.
Only mounts need a filesystem driver: WinFsp on Windows, FUSE on Linux, or a
compatible FUSE and rclone combination on macOS. Ordinary transfers do not need
these drivers.

| Platform | Default application data |
| --- | --- |
| Windows | `%LOCALAPPDATA%\RcloneGo` |
| macOS | `~/Library/Application Support/RcloneGo` |
| Linux | `$XDG_CONFIG_HOME/RcloneGo`, or `~/.config/RcloneGo` |

These directories contain account configuration, task state, private engines
and logs. Upgrading the application preserves them. For an isolated profile,
start with `-data <directory>` or set `RCLONEGO_DATA`; do not copy a profile into
a public archive. Deliberate data removal is separate from uninstalling the app.

Automatic update checks start off in the alpha and can be enabled in Settings.
New alpha/beta profiles use the preview channel; a saved channel choice is kept.
The stable channel waits for a newer stable release. Installation
and update-triggered relaunch must wait for transfers and owned mounts to stop.
Client update signatures authenticate update archives; Windows Authenticode and
macOS Developer ID signing/notarization are separate platform checks.

## 简体中文

从 [GitHub Releases](https://github.com/DeliciousBuding/rclonego/releases) 下载。
按运行电脑的处理器选择包。每次发行会写明实际提供的架构、签名和验证范围；
alpha 标为 prerelease，已配置构建目标不表示发行包已经公开。

### Windows

大多数电脑选择 `rclonego-setup-<version>-amd64.exe`，Windows on ARM 选择
`arm64` 安装器。双击后按向导安装。默认只为当前用户安装到
`%LOCALAPPDATA%\Programs\RcloneGo`，客户端本身不要求管理员权限。
安装器提供开始菜单入口、可选桌面快捷方式和系统卸载登记。

ZIP 是免安装选项：完整解压到可写目录后运行 `RcloneGo.exe`。
默认设置仍保存在用户目录，移动 exe 不会同时移动账户与任务历史。
文件名带版本的单独 exe 使用手动更新；安装版与保留 `RcloneGo.exe`
名称的解压版可以使用官方更新器。

覆盖安装或卸载前，从托盘菜单选择“退出 RcloneGo”。关闭窗口可能只是隐藏，
后台传输仍在运行。卸载使用“设置 → 应用 → 安装的应用”中的 RcloneGo 条目。
安装和卸载会拒绝修改仍有程序运行的安装目录，不会强制停止传输。
卸载保留用户数据，重新安装可以继续使用原账户与任务历史。

### macOS

Apple Silicon 选择 `arm64` DMG，Intel Mac 选择 `amd64`：
打开 DMG，将 RcloneGo 拖入 Applications，弹出磁盘映像，再从 Applications 启动。
不要长期运行磁盘映像里的只读副本。安装位置不可写时，退出后下载新的 DMG 替换应用；
应用 ZIP 也可以解压出 `.app` 后移入 Applications。

卸载前退出程序，已开启登录启动时先在设置中关闭，再把 `RcloneGo.app` 移到废纸篓。
单独的用户数据目录会保留。未签名的 alpha 可能需要明确批准打开；
按 [Apple 的说明](https://support.apple.com/guide/mac-help/open-a-mac-app-from-an-unidentified-developer-mh40616/mac)
操作，不关闭 Gatekeeper。

### Linux

Debian/Ubuntu 使用软件安装器打开 DEB，或运行：

```sh
sudo apt install ./rclonego-<version>-linux-amd64.deb
```

升级与卸载交给包管理器，卸载命令为 `sudo apt remove rclonego`。
tar.gz 留给手动安装：解压到可写应用目录，保留随包文件并运行 `rclonego`。
系统包通过包管理器升级，用户可写目录中的安装可使用应用内更新。

### 首次使用与数据

首次启动默认跟随系统主题，并匹配受支持的系统语言；设置页可以立即切换。
登录启动默认关闭，开启后登录时留在托盘或菜单栏，再次打开会显示已有窗口。
Ctrl+,／Cmd+, 打开设置，Ctrl+Q／Cmd+Q 退出应用。
独立 `-data` 配置不登记登录启动，避免系统启动项打开另一份默认配置。

在“设置 → rclone 引擎”下载推荐版本、选择已有引擎或导入经过校验的离线 ZIP。
客户端与引擎分别管理，不覆盖系统 rclone。挂载才需要额外的文件系统驱动：
Windows 为 WinFsp，Linux 为 FUSE，macOS 使用兼容的 FUSE 与引擎组合。
普通传输不需要安装这些驱动。

默认数据目录见上表，包含账户配置、任务状态、私有引擎和日志，升级时保留。
隔离配置使用 `-data <directory>` 或 `RCLONEGO_DATA`。用户数据不随卸载清除，
彻底移除数据是单独的明确操作。

alpha 默认关闭自动检查更新，可在设置中开启。新的 alpha/beta 配置默认使用预览通道，
已有通道选择会保留；稳定通道等待更新的稳定版。
安装与更新重启应等待活动传输及本实例挂载结束。更新包签名、Windows Authenticode、
macOS Developer ID 签名与公证分别验证，不能相互替代。
