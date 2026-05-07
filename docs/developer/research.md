# Research — Developer Guide

Technologies are defined in `data/research.json`. The research system (`internal/research/`) loads these at startup and checks prerequisite chains at runtime.

---

## research.json Structure

`research.json` is a JSON object keyed by tech ID.

```json
"basic_metallurgy": {
    "name": "Basic Metallurgy",
    "description": "Foundational metal processing techniques.",
    "duration": 100,
    "required_building": "research_lab",
    "required_int": 8,
    "unlocks": ["metal_plating", "reinforced_door"]
}
```

| Field | Type | Description |
|-------|------|-------------|
| key | string | Unique tech identifier |
| `name` | string | Display name in the research menu |
| `description` | string | Short description |
| `duration` | int | Base number of worker-ticks to complete |
| `required_building` | string | Build ID that must be present in the settlement for research to proceed |
| `required_int` | int | Minimum colonist `Int` stat required to work this research |
| `requires_tech` | string | ID of a prerequisite tech. Omit if none. |
| `unlocks` | []string | Build or tech IDs unlocked on completion |

---

## How It Works

- `internal/research/research.go` loads all tech definitions at startup.
- `settlement.KnownTechs` is a set of tech IDs the settlement has completed.
- A tech is available to research when: its `required_building` is built, `requires_tech` (if set) is in `KnownTechs`, and an available colonist meets `required_int`.
- Multiple colonists can work the same research task simultaneously — progress accumulates from all contributors.
- On completion the tech ID is added to `KnownTechs` and any entries in `unlocks` become available.

---

## Current Tech Tree

```
basic_metallurgy ──▶ advanced_metallurgy
xenobiology
power_systems
```

| Tech | Duration | Required Int | Unlocks |
|------|----------|-------------|---------|
| `basic_metallurgy` | 100 | 8 | `metal_plating`, `reinforced_door` |
| `advanced_metallurgy` | 200 | 12 | `plasma_cutter`, `armored_wall` |
| `xenobiology` | 150 | 10 | `bio_farm`, `alien_medicine` |
| `power_systems` | 180 | 11 | `generator`, `power_conduit` |

All techs require a `research_lab` to be built.

---

## Adding a New Technology

1. Add an entry to `data/research.json` with a unique key.
2. Set `required_building` (typically `"research_lab"`), `required_int`, and optional `requires_tech`.
3. List the build or tech IDs this unlocks in `unlocks`.
4. No Go code changes needed for a standard research entry.
