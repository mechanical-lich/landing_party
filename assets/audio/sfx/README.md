# World SFX assets

Positional in-world sound effects. See
[docs/developer/audio.md](../../../docs/developer/audio.md) for the architecture
(composed impacts, material/footstep resolution, variation sets, the world
bridge); this is just the drop-in reference. Mapped in
[`data/audio.json`](../../../data/audio.json).

### Variation sets (the `impacts/` pack)
A clip key can map to an array of files; the mixer picks one at random.

| Key pattern | Files | Used by |
|---|---|---|
| `impact_{surface}_{weight}` | `impacts/impact{Surface}_{weight}_00N.ogg` | melee/ranged impacts + chop swings (target `Material` × striker `Weight`) |
| `impact_{material}_medium` | (above) | material-aware mining (ore→metal, crystal→glass) |
| `mine` | `impacts/impactMining_00N.ogg` | generic dig/mine |
| `footstep_{surface}` | `impacts/footstep_{surface}_00N.ogg` | footsteps by Floor `material` (default `wood`) |

Surfaces present: bell, generic, glass, metal, plank, plate, punch, soft, tin,
wood (not every surface has every weight — unmatched combos fall back to
`sfx_impact`).

### Generic fallback clips
| File (expected) | Clip key | Fallback for |
|---|---|---|
| `gunshot.ogg` | `sfx_gunshot` | ranged shot |
| `impact.ogg`  | `sfx_impact`  | any impact whose composed set is missing |
| `break.ogg`   | `sfx_break`   | dig/mine when the `mine` set is missing |
| `death.ogg`   | `sfx_death`   | death when the entity has no `death` sound |

### Entity-specific clips (from `SoundComponent`)
| File (expected) | Clip key | Used by |
|---|---|---|
| `worm_death.ogg` | `sfx_worm_death` | worms dying |
| `worm_alert.ogg` | `sfx_worm_alert` | worm tremorsense screech |

`.ogg`/`.mp3`/`.wav` (inferred from the extension); missing files play silently.
