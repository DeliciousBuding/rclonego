# Native interface design

RcloneGo uses MyGo native components. Navigation, task information and controls
share a restrained palette and spacing system; service marks retain their own
colors and are separate from the product mark.

## Hierarchy and material

The sidebar and title bar carry the window material. The content canvas remains
opaque for readable paths, progress and error messages. Light, dark and system
appearance use the same hierarchy. MyGo's native active/inactive button treatment
is preserved; icon/text children inherit the button's color.

Windows uses the same solid background for the native caption, sidebar and
exposed frame, keeping their color consistent through focus and wallpaper changes.
Windows 11's caption/text color attributes style the system title bar; its
buttons and behavior remain native. High contrast and older Windows retain
system caption colors.
macOS shares one thin tint over MyGo's system Glass material across the inset
title bar and navigation. Inactive windows, high contrast and reduced transparency
use a consistent opaque theme color. Transient messages use MyGo Glass over the
content. A fine canvas border separates content.

Light appearance uses a white canvas and white control surfaces. Dark
appearance uses a near-black neutral canvas with charcoal control surfaces.
Dark chrome stays charcoal instead of becoming a bright gray frame.
Clear blue identifies interactions and progress; sidebar selection uses a
translucent blue fill with readable text in both focused and unfocused states.
Provider marks keep their official colors. Primary controls use white text in
light appearance and dark text on the brighter blue in dark appearance.
The base is MyGo's matching light/dark theme, preserving its native scrollbar
and inactive-control behavior. Text-heavy content stays opaque; transparency
is concentrated in navigation and window chrome.

## Window frame and navigation

Windows and Linux keep system decorations, including minimize, maximize/restore,
close, resizing and the window menu. Application content does not draw replacement
caption buttons or cover the system title bar. Search and refresh share the top
of the sidebar; the content canvas starts close to the frame without a second,
empty title row. Linux leaves button placement and decoration to GTK and the
desktop instead of imposing the Windows arrangement.

The initial window is 1280 × 760 DIPs with a 208-DIP sidebar. This gives
transfer metrics and paired lists room without reducing text size. Smaller
windows remain supported down to 900 × 560 DIPs.

macOS uses a native inset title bar. One full-width draggable toolbar reserves
the control area reported by MyGo before placing the product mark and commands.
Sidebar navigation and the content canvas both start below this row. The toolbar
height also accommodates a larger reported control area. Full screen and native
title bars report no inset area, so no extra caption row is reserved.

## Page structure and density

Each management page starts with a 22px title, one short description or count,
and its primary action aligned at the trailing side. Repeated rows align names,
secondary paths, status and actions. Workflow rows show Run and Edit; clone,
export and deletion use a More menu. Mount rows show their remote-to-drive route,
state and mount/unmount action; editing, cache and offline-copy actions use More.
Running mount configuration cannot be removed. Resource counters have labelled columns,
and live bandwidth controls appear when a transfer can use them.

Related preferences share a row when space permits and wrap at smaller widths.
Advanced settings remain collapsed. Keep default text and control sizes; increase
useful density through grouping and disclosure before reducing click targets or
font sizes. Current task details replace the list at narrow widths.

The transfer header identifies its storage provider and keeps the ownership
hint beside the path; the full handoff explanation is available on hover.
Observed mappings show their remote when their recorded name repeats the drive.
Provider marks recur beside mappings, recent transfers and the current file
location. Their meaning follows the configured remote, including a download's
source; local or unknown locations use the existing neutral line icons.
Action buttons remain monochrome so provider identity and actions stay distinct.

Account rows show the provider mark, name and provider, with Browse as the
labelled primary action. Details groups capacity, shared hash algorithms and
backend capabilities; reconnect sits in More. Unknown space stays unknown.
Account quota and directory size are separate measurements. Directory statistics
are requested inside the file browser and can be canceled. The browser shows
file sizes and prepares a transfer through the existing task form.

Resources starts with the official rclone symbol, selected version and engine
shortcut. The client's process totals and other rclone processes are separate.
Diagnostic counters expose listing, checks, elapsed time and engine errors.
Provider colors identify services; controls keep the existing single-color
action icon system.

Activity identifies who manages a transfer. Imported active work shows the old
client as owner until it records completion; pending steps and whole-run progress
remain visible. Passive external observations do not imply ownership of their
controls or final result. Global commands stay with navigation while the page
heading carries its title and primary action, avoiding repeated headings.

File browsing offers current-page search, name/size sorting and native list
selection. Enter opens a selected directory, Backspace returns to its parent,
and Ctrl/Cmd+C copies the selected path. Selection details and bottom actions
remain visible at the minimum window size. Search never silently scans remote
trees or other pages. A [current native browser](../assets/screenshots/files-dark.png)
uses a synthetic account and sample documents.

The Files workspace extends browsing into a full page. It uses MyGo breadcrumbs,
native list multi-selection and context menus, with a fixed selection/action
footer. Name, size and modification columns align across rows. Enter/double-click
opens, Space previews and F2 renames. Preview and file-action dialogs use short
140 ms transitions; button colors settle over 120 ms. MyGo respects reduced
motion. Nothing continuously animates while idle.

Files uses a compact heading, a single navigation row and 28-DIP file rows.
The path editor replaces the breadcrumbs on demand. Selection actions and
usable pagination share one footer. Latest-operation feedback occupies one row;
failed item details open separately so the result does not displace the list.
The queue can focus that operation's tasks with aligned destination, status and
progress columns, using the same stored jobs and inspector.

Mount configuration keeps Save outside its scrolling fields. Available Windows
drive letters and a profile cache location reduce manual setup. Browse connects
mapping rows to Files; existing external drives expose their ownership clearly.

Active tasks distinguish preparation, comparison, transfer and verification.
Unknown totals stay indeterminate; verification progress uses file counts rather
than transfer bytes. The sidebar count includes queued and running work. Empty
history omits filters, and pagination appears only when another page exists.

Task details use one reading flow: name and state, source and destination,
collapsible execution metadata, errors and logs. Paths and errors expose their
complete text through selection and tooltips. Error banners wrap, stay readable
and can be dismissed without leaving the page.

Task forms scroll above a fixed action row. Preview, cancel and submission stay
visible when advanced options or comparison results are expanded. Required-input
issues appear beside their field after submission is attempted. Sync begins with
a preview and enables confirmation only with explicit deletion permission and
complete preview evidence.

Run results open in a dialog with status, completed steps, verification evidence
and scope before the attempt list. A failed step includes the engine's cause,
and recovery stays in the fixed action row. Successful copying does not imply
separate content verification.

These choices follow [Apple's layout guidance](https://developer.apple.com/design/human-interface-guidelines/layout),
[Apple's material guidance](https://developer.apple.com/design/human-interface-guidelines/materials),
[Fluent productivity guidance](https://learn.microsoft.com/en-us/windows/apps/get-started/line-of-business/design-for-lob),
and [Mica layering](https://learn.microsoft.com/en-us/windows/apps/design/style/mica).
Operation forms also follow [Rclone UI's path-first and collapsible-options pattern](https://rcloneui.com/docs/ui/operations)
using the supported [MyGo components](https://mygo.egoist.dev/docs/ui).

The default scale uses a 13px body, 11–12px supporting text, 15px section headings
and 22px page headings. Action icons use a 24-unit grid with 1.65-unit rounded
strokes and appear at 15–18px. Important state uses text and icons or dots together,
so color alone does not communicate outcome.

## Components and objects

- Product mark: application identity, About and overview.
- Storage logo: a recognized provider on an account; unfamiliar providers use the
  generic cloud or storage icon. A logo never implies provider endorsement.
- Action icon: plus for creation, play for execution, trash for deletion, arrow
  for continuation and download for acquisition. Close dismisses a panel.
- Files and folders: icons identify entries, while names and paths carry identity.
- Status bar: engine, attention and queue entries navigate to their relevant page.

Text buttons stay labelled. Icon-only controls have an accessible name and tooltip.
Native browse dialogs select local paths; the text field still accepts remote
locations directly. Destructive mirror execution requires its own verified preview.

The sidebar's **Settings** entry opens one page with General, Updates and Advanced
groups. General uses aligned preference rows for appearance, language and login
startup. New profiles follow the system appearance and language; existing manual
choices are preserved. The language selector shows its automatic/manual mode and
the current effective language separately. Login startup is available only for
the default profile; an independent profile shows a disabled control with a
short explanation. Engine management and new-task defaults are collapsed
until needed. The engine status shortcut opens Settings with its engine section
expanded. Resources keeps process observations and live transfer controls.
The application menu and Ctrl+, / Cmd+, open the same Settings page.
Ctrl+Q / Cmd+Q and the tray Quit action stop the application explicitly.

## Layout boundaries

Overlay scrollbars have a 16px content gutter. Shrinkable labels use `MinWidth(0)`,
single-line truncation or bounded wrapping, with full paths available in tooltips.
Controls retain their click area. Long names must not push actions offscreen.

The window minimum is 900 × 560 logical pixels. At narrow widths, task details
occupy the content pane; closing them returns to the queue. Dialogs fit the viewport
and scroll their body. Resource metrics can wrap while exact process details remain
in their own list.

Keyboard focus, high contrast, reduced transparency and text scale follow MyGo
preferences. Inspect a real window after layout changes, including active/inactive
appearance, long names, narrow dimensions and display scaling. Headless layout and
SVG rendering tests cover regression boundaries alongside native inspection.

SVG service assets use paths supported by MyGo, avoiding embedded images, masks
and CSS-dependent fills. Their [sources and licenses](third-party.md) are recorded.

The [monochrome mark](../assets/mark-mono.svg) is the brand geometry source.
Run `python scripts/generate_brand.py` to regenerate the
[color mark](../assets/mark.svg), [application icon SVG](../assets/icon.svg)
and [wordmark](../assets/wordmark.svg); `--check` verifies their consistency.
The wordmark uses the same symbol geometry and the host's system font. The
application's existing [PNG icon](../assets/icon.png) retains the same geometry
and color plate for native packaging. Raster exports and tray glyphs should be
checked at their actual small display sizes when changed.

Use the monochrome symbol for compact chrome and monochrome trays, the color
symbol beside the product name, and the rounded plate for application launchers.
The [motion study](../assets/motion.gif) is an optional presentation asset.
The application stays static at idle. Service marks identify the provider or
engine and remain separate from the product mark.

The [overview](../assets/screenshots/overview-dark.png),
[Files](../assets/screenshots/files-dark.png),
[accounts](../assets/screenshots/accounts-light.png),
[settings](../assets/screenshots/settings-light.png) and
[resources](../assets/screenshots/resources-light.png) previews render the native
components with sample accounts, tasks and files. They contain no credentials
or live measurements; real window inspection is a separate verification step.

The [active queue preview](../assets/screenshots/queue-light.png) uses synthetic remote aliases and files.
