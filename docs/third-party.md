# Third-party components and marks

RcloneGo is Apache-2.0. This license does not replace dependency licenses.

| Component | Terms | Role |
| --- | --- | --- |
| MyGo | MIT | Native UI, desktop APIs and SQLite binding |
| purego | Apache-2.0 | Native library bindings |
| fsnotify | BSD-3-Clause | Filesystem notifications with periodic scan fallback |
| go-text/typesetting | BSD-3-Clause or Unlicense, with included notices | Text shaping |
| Go extended libraries | BSD-3-Clause | Text, image, network and OS support |
| SQLite | Public domain, with MyGo shim under its terms | Local state |
| rclone | MIT | Separately selected transfer engine |

Dependency license texts are in `third_party/licenses`.
The standalone executable embeds these notices. Read them in About or with
`rclonego licenses`; copying only the executable preserves the license texts.

RcloneGo's geometric mark and action icons are project assets. System fonts are
provided by the operating system; Apple fonts and SF Symbols are not bundled.

Google and Google Drive marks identify those integrations only. They are
trademarks of Google LLC and are not covered by the project's source license.
The RcloneGo app mark does not incorporate them.
Google, Drive, OneDrive, Dropbox and GitHub SVGs are sourced from
[gilbarbara/logos](https://github.com/gilbarbara/logos), whose collection is CC0
(text in `third_party/licenses/logos-cc0.txt`). Trademark ownership remains with
each provider. The Drive mark uses its path-only revision
`c1a8f4dfd9334aeb28220dbad6175d47e6332823` for native SVG compatibility.
Other action icons and the wordmark are project-owned vectors.

The rclone symbol comes from the official
[rclone SVG](https://github.com/rclone/rclone/blob/5d4d5d2b073ec0a20a30d00438ad891b6681e772/docs/static/img/logo_on_light__horizontal_color.svg).
The asset retains its three symbol paths and colors, omits the wordmark and
converts inline styles to SVG attributes for native rendering. rclone retains
its own identity; the symbol identifies the transfer engine.

WinFsp is a separate Windows mount dependency licensed under GPLv3 with a
FLOSS exception. It is not bundled in this development baseline. Distribution
must follow the [WinFsp license](https://github.com/winfsp/winfsp/blob/master/License.txt),
including applicable UI and documentation attribution.
