# Native interface design

RcloneGo uses MyGo native components. Navigation, task information and controls
share a restrained palette and spacing system; service marks retain their own
colors and are separate from the product mark.

## Hierarchy and material

The sidebar and title bar carry the window material. The content canvas remains
opaque for readable paths, progress and error messages. Light, dark and system
appearance use the same hierarchy. MyGo's native active/inactive button treatment
is preserved; icon/text children inherit the button's color.

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

Brand assets include the [wordmark](../assets/wordmark.svg),
[standalone mark](../assets/mark.svg) and [motion study](../assets/motion.gif).
The app uses the static mark to avoid continuous animation at idle.
The [account screenshot](../assets/screenshots/accounts-light.png) uses an empty
synthetic profile with no account credentials.
