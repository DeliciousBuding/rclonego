# Third-party components and marks

RcloneGo is Apache-2.0. This license does not replace dependency licenses.

| Component | Terms | Role |
| --- | --- | --- |
| MyGo | MIT | Native UI, desktop APIs and SQLite binding |
| purego | Apache-2.0 | Native library bindings |
| go-text/typesetting | BSD-3-Clause or Unlicense, with included notices | Text shaping |
| Go extended libraries | BSD-3-Clause | Text, image and network support |
| SQLite | Public domain, with MyGo shim under its terms | Local state |
| rclone | MIT | Separately selected transfer engine |

Dependency license texts are in `third_party/licenses`.

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

WinFsp is a separate Windows mount dependency licensed under GPLv3 with a
FLOSS exception. It is not bundled in this development baseline. Distribution
must follow the [WinFsp license](https://github.com/winfsp/winfsp/blob/master/License.txt),
including applicable UI and documentation attribution.
