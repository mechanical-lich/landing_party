# Audio — Developer Guide

The audio system splits into a **reusable engine** in `mlge/audio` and
**game-specific policy** in `internal/audio`. The engine knows nothing about the
game; it just plays sounds. The game decides *what* plays *when*.

All sounds are configured in [`data/audio.json`](../../data/audio.json) — code
references stable **clip keys**, and the JSON maps keys to files, so adding or
reskinning sounds is a data change.

---

## Architecture

| Layer | Location | Responsibility |
|-------|----------|----------------|
| Mixer | `mlge/audio/mixer.go` | SFX/UI one-shots: buses, per-clip player pool, voice caps, same-frame coalescing, random **variation** sets |
| Clip | `mlge/audio/clip.go` | a short sound decoded fully into memory (overlap-capable) |
| MusicDirector | `mlge/audio/music.go` | streaming music: per-state playlists, crossfade, random rotation |
| Loader | `mlge/audio/audio.go` | `LoadAudioFromFile` (ogg/mp3/wav streaming), `LoadClip` (buffered) |
| System | `internal/audio/system.go` | owns the mixer + music director; the single façade `internal/game` talks to |
| UI director | `internal/audio/director.go` | maps minui interactions → UI clips |
| World bridge | `internal/audio/worldbridge.go` | plays in-world `SoundEvent`s positionally |
| Config | `internal/audio/config.go` | parses the `audio.json` sound map |

`internal/audio` keeps a package-global `System` (set on `New`), mirroring the
`effect.GetEffectManager()` idiom, so gameplay code reaches audio via package
functions (`PlayWorldSounds`, `PlayGlobal`, `NotifyCombat`, `SetMusicState`, …)
without threading a reference. All of them no-op when audio is disabled.

The system is built once in `game.NewGame` (`audio.New("data/audio.json")`) and
ticked once per frame from `Game.Update` via `System.Update` (which advances the
mixer and the music director).

---

## Buses & volume

The mixer has five buses: `Master`, `Music`, `SFX`, `UI`, `Global`. Effective
per-voice gain = `master × bus × per-play`. The settings screen
([`settings_state.go`](../../internal/game/settings_state.go)) exposes three
sliders that map to buses:

| Slider | Bus | Notes |
|--------|-----|-------|
| UI Volume | `BusUI` | button clicks, list/tab/toggle |
| Game Volume | `BusSFX` | all positional world SFX |
| Music Volume | (director) | streaming music isn't on a mixer bus — `SetMusicVolume` drives the `MusicDirector` directly |

The `Global` bus (scan/research chimes) is currently **not** covered by any
slider. Sliders are stored 0..1 but pass through a **perceptual (squared) curve**
(`perceptualGain`) before hitting the bus, so the middle of the slider is audibly
quieter (linear volume feels dead until near zero).

Volumes and the two footstep toggles persist to
[`data/config.local.json`](../../internal/config/config.go) (`SaveAudioSettings`)
and are re-applied on startup and after any change.

---

## 1. UI feedback (synchronous input hook)

UI sounds fire from a synchronous hook, **not** the event bus, so they're instant
even when a handler does slow work (e.g. world-gen on a button click).

- minui exposes `var InteractionSound func(kind event.EventType, elementID string)`.
- Each interactive widget calls it the moment it registers a click — **before**
  its `OnClick`/`OnChange` — and only on real input paths (never programmatic
  setters like `SelectByIndex`).
- `internal/audio` sets the hook to the UI director, which maps the interaction
  `kind` → a clip key (the `ui` map in `audio.json`) → `mixer.Play` on `BusUI`.

Because the hook only fires on genuine input, programmatic setup (a title-screen
`SelectByIndex` during construction) never chirps — that's why there's no startup
sound to suppress.

---

## 2. Positional world SFX (the bridge)

In-world sounds are emitted as `world.SoundEvent`s (`level.EmitSound` /
`EmitSoundClip`) and played by the **world bridge**, which each frame reads the
**live** level's `Sounds` slice (using the monotonic `SoundSeq` as a cursor) and
plays new events that are:

- on the **camera's z**, and
- **inside the view rect** `[CameraX, +ViewW) × [CameraY, +ViewH)`,

with volume rolling off from the view center. Because it reads only the live
on-screen level, background-simulated planets are silent. Driven from
`MainState.Update` via `audio.PlayWorldSounds(s.level)`.

### How a world sound picks its clip
`SoundEvent` carries an optional explicit `Clip` and a `Tag`. The bridge resolves:

1. **Explicit clip** if set *and loaded* (an entity-resolved sound, or a composed
   impact key).
2. Else the **tag's generic clip** (the `world` map in `audio.json`) as a fallback.

So a composed key like `impact_metal_heavy` that isn't loaded degrades to the
plain `sfx_impact`.

Footsteps are additionally played at **half volume**, and gated by the
friendly/enemy toggles (see below) — but the `SoundEvent` still fires regardless,
so AI hearing is unaffected.

---

## 3. Entity & material sound resolution

Two ECS-level mechanisms decide the *specific* clip for a gameplay event, both in
[`internal/components/SoundComponent.go`](../../internal/components/SoundComponent.go):

**`SoundComponent`** — an entity (or an equippable item) maps sound events to clip
keys, plus a `Material` and `Weight`:
```json
"Sound": { "Material": "soft", "Sounds": { "death": "sfx_worm_death", "alert": "sfx_worm_alert" } }
```
- `ResolveSound(entity, event)` — clip for `death`/`alert`/`shoot`/etc., **gear
  overrides the entity's own** (armor changes `hit`, a weapon `hitting`, a ranged
  weapon `shoot`). Walks the equipment slots then the entity.
- `ResolveMaterial` / `ResolveWeight` — same gear-then-entity resolution for the
  surface an entity presents when struck, and how hard it strikes.

**Composed impacts** — `ImpactClipKey(attacker, target)` → `impact_{surface}_{weight}`:
the target's `Material` picks the surface (default `generic`), the attacker's
`Weight` picks the force (default `medium`). One impact per melee strike; ranged
adds a separate shot at the shooter. Tiles declare `material` in
`data/tiledefinitions/*` — Middle-slot material drives **mining**
(`MiningClipKey`), Floor-slot material drives **footsteps** (`FootstepClipKey`,
default `wood`).

### Emitters today
| Event | Where | Clip |
|-------|-------|------|
| Melee / ranged impact | `internal/combat/combat.go` | composed `impact_{surface}_{weight}` → `sfx_impact` |
| Ranged shot | `combat.Shoot` | weapon/attacker `shoot` → `sfx_gunshot` |
| Death | `main_state.go` `OnEntityDead` | entity `death` → `scream`/`sfx_death` |
| Dig / mine / chop | `internal/workerai/task_handlers.go` | material-aware mining set → `mine` → `sfx_break` |
| Footsteps | `workerai/movement.go`, ScriptedAI/FactionAI moves, rogue | `footstep_{surface}` |
| AI-scripted | `play_sound(event)` ml-basic primitive | entity's `SoundComponent` clip |

**`play_sound(event[, loudness])`** is an ml-basic AI primitive
([ScriptedAISystem.go](../../internal/systems/ScriptedAISystem.go)) — resolves the
entity's `SoundComponent` and emits positionally. No-op if the entity has no clip
for the event. Used e.g. by `worm.basic` to screech `alert` on tremorsense.

---

## 4. Global one-shots

`audio.PlayGlobal(key)` plays on the `Global` bus, positionless and **not**
camera-gated — for milestones heard anywhere, on any level. Wired to:

- **scan success** (`notify`) — [`starmap_panel.go`](../../internal/game/starmap_panel.go)
- **research complete** (`research_done`) — [`task_handlers.go`](../../internal/workerai/task_handlers.go), so it's heard even when research finishes on a background level (the Ship while planetside).

---

## 5. Music

The `MusicDirector` streams tracks organised into **states**, rotating randomly
among a state's tracks and crossfading (~2s) on rotation and state changes. The
game drives the state:

| State | Set by | Notes |
|-------|--------|-------|
| `menu` | title / settings states | |
| `field` | `MainState` (a site) | base in-game state |
| `combat` | `NotifyCombat()` | overrides `field` while active |

The **dashboard/star-map sets nothing**, so it inherits whatever is currently
playing (no vibe-killing switch as players bounce between a site and the map).

**Combat detection**: `audio.NotifyCombat()` is called whenever a colonist
(`Worker`) is on either side of a strike — the colonist-attack paths
(`combat.MeleeAttack`/`Shoot`) and the mob-attack paths (ScriptedAI `attack_at`,
FactionAI). It sets a **10s cooldown**; each frame the effective state is
`combat` while the cooldown runs, then crossfades back to `field`.

A track shared across states (e.g. `nebula` in `menu` + `field`) loads once and
can carry across the state change. A single-track state (`combat`) just loops.

---

## `data/audio.json` reference

```jsonc
{
  "clips":  { "<key>": "path.ogg"  |  ["a_0.ogg", "a_1.ogg"] },  // single or variation set
  "ui":     { "ui.button.click": "ui_click", ... },              // minui event → clip key
  "world":  { "gunshot": "sfx_gunshot", ... },                   // SoundTag → generic fallback clip
  "music":  { "menu": ["..."], "field": ["..."], "combat": ["..."] }
}
```

- A `clips` value may be a **string** (one file) or an **array** (a random
  variation set the mixer picks from). Formats: `.ogg`, `.mp3`, `.wav`; inferred
  from the extension.
- SFX clips are decoded fully into memory (short, overlap-capable); music streams.
- Missing files are non-fatal: they play silently and startup logs a one-line
  `loaded N/M` summary.

### Asset folders
Filenames the current config expects live alongside the audio in
`assets/audio/{ui,sfx,music}/` — each has a short README with its drop-in table.

---

## Where to add a sound

| Goal | Do |
|------|----|
| New UI sound | add `clips` + a `ui` mapping in `audio.json` |
| New generic world SFX | add `clips` + a `world` tag mapping; emit with `EmitSound(tag)` |
| Entity-specific sound | add a `Sound` component to the blueprint + the `clips` key |
| Material impact/footstep | tag the tile/entity `material`; ensure the `impact_*` / `footstep_*` set exists |
| Global chime | `audio.PlayGlobal("key")` + the `clips` key |
| Music track / state | add files to a `music` state |
| Sound variations | make the `clips` value an array |
