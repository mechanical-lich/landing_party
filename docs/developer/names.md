# Names — Developer Guide

All randomly-generated names (settlements, colonists, mutants, locations, boss epithets) come from a single data file: `data/names.json`. The runtime exposes one set of functions in `internal/lore` that any subsystem can call. Adding a new pool — yeti names, raider names, whatever — is a JSON edit.

---

## `names.json` Schema

A top-level JSON object keyed by **name type**. Each type declares a `kind` of either `"flat"` (a single list) or `"combined"` (prefix + suffix). One reserved key, `"epithets"`, holds boss-name decoration pools.

```json
{
  "settlement": {
    "kind": "combined",
    "prefixes": ["New", "Fort", "Colony", ...],
    "suffixes": ["Alpha", "Prime", "Hope", ...]
  },
  "colonist": {
    "kind": "flat",
    "names": ["Vera", "Dax", "Kira", ...]
  },
  "mutant": {
    "kind": "flat",
    "names": ["Grosk", "Vrall", "Sludge", ...]
  },
  "location": {
    "kind": "combined",
    "prefixes": ["Veil", "Ashen", "Cinder", ...],
    "suffixes": ["Reach", "Expanse", "Hollow", ...]
  },
  "epithets": {
    "title":  ["the Destroyer", "the Merciless", ...],
    "suffix": ["mauler of cities", "eater of worlds", ...]
  }
}
```

Schema rules:

| Field | Required for | Notes |
|-------|--------------|-------|
| `kind` | every type | `"flat"` or `"combined"`. |
| `names` | `flat` types | Non-empty array of strings. |
| `prefixes`, `suffixes` | `combined` types | Both non-empty. Output is `"<prefix> <suffix>"`. |
| `epithets.title` | optional | Pool for the "the X" decoration. |
| `epithets.suffix` | optional | Pool for the ", X of Y" decoration. |

Type names are arbitrary lowercase strings. There's no registry of valid types in code — `RandomName("yeti")` simply works if `yeti` is in the JSON.

The file is loaded lazily on first call via `sync.Once` and held in memory; restart the game to pick up edits. `lore.NamesPath` can be overridden in tests to point at a fixture.

---

## Runtime API

All in `github.com/mechanical-lich/landing_party/internal/lore`.

| Function | Use |
|----------|-----|
| `RandomName(type)` | Roll a name using the global RNG. Use for one-off, non-deterministic generation (UI, gameplay events). |
| `RandomNameSeeded(rng, type)` | Roll a name using a supplied `*rand.Rand`. Use whenever the caller must be reproducible from a seed — campaign generation, saved-world regeneration. |
| `ResolveName(s)` | If `s` is a `<type>` placeholder, return a freshly-rolled name. Otherwise return `s` unchanged. Used by the entity factory. |
| `HasNameType(type)` | Reports whether the type exists in `names.json`. |
| `NameWithEpithet(base)` | Decorates `base` with a title epithet and, ~50% of the time, a trailing suffix epithet. Used by the `add_epithet` script function. |
| `RandomTitleEpithet()` / `RandomSuffixEpithet()` | Pick one epithet from the named pool. Useful if you want full control over decoration. |
| `RandomSettlementName()` | Thin convenience wrapper kept for legacy callers (`settlement.go`, title screen). Equivalent to `RandomName("settlement")`. |

### Unknown types

`RandomName("typo")` returns the literal string `"Unknown"` so a missing type stays visible instead of producing a stealthy bad name. `ResolveName` is stricter — when a `<type>` placeholder references an unknown name type, the original placeholder text is returned unchanged so the broken reference is obvious in the UI.

---

## `<type>` Placeholders in Blueprints

The entity factory resolves any `DescriptionComponent.Name` wrapped in angle brackets via `lore.ResolveName`. So a blueprint can roll a random name on every spawn just by declaring the placeholder:

```json
"mutant_brute": {
    "Description": { "Name": "<mutant>", "Faction": "mutant" },
    ...
}
```

Each `mutant_brute` spawn now gets a fresh name from the `mutant` pool. The placeholder token is **normalized** — lowercased, with a trailing `name` stripped — so all of these resolve to the `colonist` type:

- `<colonist>`
- `<Colonist>`
- `<ColonistName>` (legacy form, still works)

Any blueprint with a `<…>` name that doesn't match a known type leaves the literal placeholder visible. That's deliberate, to surface typos.

To make existing static-named entities roll instead, set their `Description.Name` to `<your_type>` in the blueprint JSON. No code change.

---

## Determinism

`RandomName` reaches into `math/rand`'s global generator and is therefore **non-deterministic** across runs. Anywhere a system depends on seed reproducibility — campaign generation, save-time world regeneration, anything covered by `TestGenerateDeterministic` — must use `RandomNameSeeded` instead.

Concrete current use: `campaign.genName` calls `lore.RandomNameSeeded(rng, "location")` so a campaign seed always produces the same set of location names. Falling back to `RandomName` here would break the determinism test and any save-reload that re-derives locations from seed.

`ResolveName` and `NameWithEpithet` use the global RNG (entities and bosses are rolled at spawn time, not at campaign-gen time). If you ever need a seeded boss epithet, factor out a seeded helper the same way `RandomNameSeeded` mirrors `RandomName`.

---

## Epithets and Boss Names

Epithets exist to promote a generic NPC into a quest boss without hardcoding text. The pattern is:

1. The NPC blueprint rolls a base name via a placeholder (`<mutant>`, `<yeti>`, …).
2. A structure script calls `add_epithet(x, y, z)` on the spawned entity. That wraps `lore.NameWithEpithet`.
3. `mark_quest_target(x, y, z)` runs afterward; the rewritten quest title picks up the full decorated name.

Example output: `"Grosk"` → `"Grosk the Destroyer, mauler of cities"`. See [structure_scripts.md](structure_scripts.md) for the script-side API and the `bunker.basic` example.

To tune what epithets feel like, edit `names.json` → `"epithets"`. Both pools may be extended independently — title-only and suffix-only entries are fine.

---

## Adding a New Name Type

1. Add a new top-level entry to `data/names.json`:

   ```json
   "yeti": {
     "kind": "flat",
     "names": ["Brokk", "Skarn", "Vrun"]
   }
   ```

2. (If desired) update the relevant blueprint to use the placeholder: `"Name": "<yeti>"`.
3. Restart the game. `lore.RandomName("yeti")` and `<yeti>` placeholders work everywhere.

Adding a `combined` type is the same shape — declare `kind: "combined"`, `prefixes`, `suffixes`. No code change is ever required to introduce a new pool.

---

## Wiring Reference

| Layer | Where to look |
|-------|---------------|
| Data file | `data/names.json` |
| Loader, public API | `internal/lore/names.go` |
| Blueprint placeholder resolution | `internal/factory/entityFactory.go` (`lore.ResolveName` in the description-component callback) |
| Seeded campaign use | `internal/campaign/generate.go` (`genName` → `lore.RandomNameSeeded(rng, "location")`) |
| Boss epithet script function | `internal/game/setup_script.go` (`add_epithet` builtin → `lore.NameWithEpithet`) |
