# UI sound assets

Drop UI sound files here. See [docs/developer/audio.md](../../../docs/developer/audio.md)
for how they're wired; this is just the drop-in reference. Mapped in
[`data/audio.json`](../../../data/audio.json) — filenames below match the default
config, so dropping a file in plays it with no code change.

| File (expected) | Clip key | Plays on |
|---|---|---|
| `click.ogg`  | `ui_click`  | button + icon-button clicks |
| `select.ogg` | `ui_select` | list-box / select-box / popup-menu selection |
| `toggle.ogg` | `ui_toggle` | toggle switches |
| `tab.ogg`    | `ui_tab`    | tab changes |
| `notify.ogg` | `notify`    | scan success (global chime) |
| `research_done.ogg` | `research_done` | research complete (global chime) |

`.ogg`/`.mp3`/`.wav` (inferred from the extension); missing files play silently.
