# World SFX assets

Positional in-world sound effects, played through the SFX bus and attenuated by
distance from the camera view. Mapped in [`data/audio.json`](../../../data/audio.json)
under `world` (world.SoundTag → clip key) and `clips` (key → path).

Ways a world sound picks its clip:
1. **Composed impact** — a strike plays `impact_{surface}_{weight}`: the target's
   `Material` (gear can override — metal armor rings) picks the surface, the
   striker's `Weight` (weapon overrides wielder) picks the force. One impact per
   melee hit; ranged adds a shot at the shooter. Falls back to `sfx_impact` when
   the composed set isn't loaded.
2. **Entity-specific, by `SoundComponent`** — an entity/gear names a clip per
   event (`death`, `alert`, `shoot`); played directly, overriding the generic
   tag. Gear wins over the entity's own default.
3. **Generic, by tag** — `data/audio.json` `world` maps a `world.SoundTag` to a
   clip; the fallback when nothing more specific applies.

A clip key can map to **one file or an array of variations** — the mixer picks
one at random per play (impacts and mining use this).

### Impact / mining / footstep variation sets (from the impacts/ pack)
| Key pattern | Files | Used by |
|---|---|---|
| `impact_{surface}_{weight}` | `impacts/impact{Surface}_{weight}_00N.ogg` | melee + ranged impacts and chop swings, by target `Material` × striker `Weight` |
| `mine` | `impacts/impactMining_00N.ogg` | digging + mining generic rock |
| `impact_{material}_medium` | (impact sets above) | material-aware mining — a tile's Middle `material` (ore→metal, crystal→glass) rings like that surface; else `mine` |
| `footstep_{surface}` | `impacts/footstep_{surface}_00N.ogg` | footsteps, by the Floor tile's `material` (grass/snow/wood/carpet/concrete; default wood) |

Tile `material` lives in `data/tiledefinitions/*` (`material` field): Middle-slot
material drives mining, Floor-slot material drives footsteps. Tagged so far:
ore/radioactive→metal, crystal→glass, hull_wall→metal, grass→grass, ice_floor→snow.

Surfaces present: bell, generic, glass, metal, plank, plate, punch, soft, tin,
wood (not every surface has every weight — unmatched combos fall back to
`sfx_impact`). Declared materials so far: colonists = `soft`, robots = `metal`,
worms = `soft`.

### Generic tag fallbacks
| File (expected) | Clip key | Fallback for (`world.SoundTag`) |
|---|---|---|
| `gunshot.ogg` | `sfx_gunshot` | ranged shot (`gunshot`) |
| `impact.ogg`  | `sfx_impact`  | any impact whose composed set is missing (`impact`) |
| `break.ogg`   | `sfx_break`   | digging/mining when the `mine` set is missing (`break`) |
| `death.ogg`   | `sfx_death`   | death (`scream`) when the entity has no `death` sound |

### Entity-specific clips (from `SoundComponent`)
| File (expected) | Clip key | Used by |
|---|---|---|
| `worm_death.ogg` | `sfx_worm_death` | worms dying (`ancient_worm`, `worm`) |
| `worm_alert.ogg` | `sfx_worm_alert` | a worm screeching when it senses prey (tremorsense) |

Notes:
- Only sounds on the **live** level, at the **camera's z**, **inside the view
  rect** play; everything else (off-screen, background planets) is silent.
- `.ogg` or `.mp3` (inferred from extension); missing files play silently.
- Emitting today: combat (composed impacts, `gunshot`/`impact` fallback), death
  (`death` → `scream` fallback), digging/mining (material-aware, `break`
  fallback), chop swings, **footsteps** on movement, AI-scripted sounds via
  `play_sound(event)` (e.g. worm `alert`), and **global one-shots** via
  `audio.PlayGlobal(key)` on the Global bus — heard anywhere, not camera-gated
  (wired to scan success as `notify`; drop `assets/audio/ui/notify.ogg`).
