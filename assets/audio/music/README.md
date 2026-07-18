# Music

Streaming background tracks. See
[docs/developer/audio.md](../../../docs/developer/audio.md) for the `MusicDirector`
(states, crossfade, rotation, combat detection); this is just the drop-in
reference. Configured in [`data/audio.json`](../../../data/audio.json) under
`music`. Supports `.ogg`, `.mp3`, `.wav`.

### States → tracks (current config)
| State | Tracks | When |
|---|---|---|
| `menu`   | ObservingTheStar, nebula | title + settings screens |
| `field`  | alien_ruins2, nebula, ObservingTheStar, creepy1 | in a game (base state) |
| `combat` | Undead Cyborg | overrides `field` while colonists are fighting |

Missing files are skipped with a one-line log; the game runs music-less.
