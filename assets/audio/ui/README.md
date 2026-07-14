# UI sound assets

Drop UI sound files here. The game maps them via [`data/audio.json`](../../../data/audio.json)
(`clips` = key → path, `ui` = minui event → key). Filenames below match the
default config — drop a file in and it plays with no code change.

| File (expected) | Clip key | Plays on |
|---|---|---|
| `click.ogg`  | `ui_click`  | button + icon-button clicks |
| `select.ogg` | `ui_select` | list-box / select-box / popup-menu selection |
| `toggle.ogg` | `ui_toggle` | toggle switches |
| `tab.ogg`    | `ui_tab`    | tab changes |

Notes:
- **Format**: `.ogg` or `.mp3` — inferred from the extension. If you use `.mp3`,
  update the path in `data/audio.json` to match (e.g. `"ui_click": ".../click.mp3"`).
- Missing files are harmless: they play silently, and startup logs a one-line
  `audio: loaded N/M clips` summary.
- Keep clips short; they're decoded fully into memory (unlike streaming music).
