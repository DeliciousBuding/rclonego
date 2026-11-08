# Native interface design

RcloneGo uses MyGo native components. Navigation, task information and controls
share a restrained palette and spacing system; service marks retain their own
colors and are separate from the product mark.

## Hierarchy and material

The sidebar and title bar carry the window material. The content canvas remains
opaque for readable paths, progress and error messages. Light, dark and system
appearance use the same hierarchy. MyGo's native active/inactive button treatment
is preserved; icon/text children inherit the button's color.

Active navigation and title-bar chrome use one thin tint over the system material.
Inactive windows, high contrast and reduced transparency use a consistent opaque
theme color. Windows uses Mica for the persistent main window; macOS uses MyGo's
system Glass material. Toolbar buttons sit directly on that material; transient messages
use MyGo Glass over the content. A fine canvas border separates content without
adding another full-window blur.

## Window frame and navigation

Windows and Linux keep system decorations, including minimize, maximize/restore,
close, resizing and the window menu. Application content does not draw replacement
caption buttons or cover the system title bar. Search and refresh share the top
of the sidebar; the content canvas starts close to the frame without a second,
empty title row. Linux leaves button placement and decoration to GTK and the
desktop instead of imposing the Windows arrangement.

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

The [account screenshot](../assets/screenshots/accounts-light.png) uses an empty
synthetic profile with no account credentials.

The [active queue preview](../assets/screenshots/queue-light.png) uses synthetic remote aliases and files.
