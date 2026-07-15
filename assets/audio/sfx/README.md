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

### Impact + mining variation sets (from the impacts/ pack)
| Key pattern | Files | Used by |
|---|---|---|
| `impact_{surface}_{weight}` | `impacts/impact{Surface}_{weight}_00N.ogg` | melee + ranged impacts and chop swings, by target `Material` × striker `Weight` |
| `mine` | `impacts/impactMining_00N.ogg` | digging + mining |

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
- Emitting today: combat (`hitting`/`hit`/`shoot`, falling back to
  `gunshot`/`impact`), death (`death` → `scream` fallback), digging/mining
  (`break`), and AI-scripted sounds via the `play_sound(event)` ml-basic
  primitive (e.g. the worm's `alert` on tremorsense).
