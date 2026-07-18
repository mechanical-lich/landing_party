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
    "cost": { "metal_ore": 5 },
    "required_building": "research_lab",
    "required_int": 8
}
```

| Field | Type | Description |
|-------|------|-------------|
| key | string | Unique tech identifier (the map key) |
| `name` | string | Display name in the research menu |
| `description` | string | Short description |
| `duration` | int | Base number of worker-ticks to complete |
| `cost` | map[string]int | Materials (blueprint ID → quantity) consumed from settlement storage when research begins. Shown in the HUD. Omit if free. |
| `required_building` | string | Build ID that must be present in the settlement for research to proceed |
| `required_int` | int | Minimum colonist `Int` stat required to work this research |
| `requires_tech` | string | ID of a prerequisite tech. Omit if none. |

---

## How It Works

- `internal/research/research.go` loads all tech definitions at startup.
- `settlement.KnownTechs` is a set of tech IDs the settlement has completed.
- A tech is available to research when: its `required_building` is built, `requires_tech` (if set) is in `KnownTechs`, an available colonist meets `required_int`, and the settlement has the `cost` materials.
- When research begins the `cost` materials are deducted from settlement storage.
- Multiple colonists can work the same research task simultaneously — progress accumulates from all contributors.
- On completion the tech ID is added to `KnownTechs`. Downstream techs and content gate on it via `requires_tech` / `KnownTechs` checks (there is no `unlocks` list — availability is derived from prerequisites, not declared here).

---

## Current Tech Tree

The authoritative list is [`data/research.json`](../../data/research.json); it
drifts, so it isn't duplicated here. Chains are formed by `requires_tech` (e.g.
`advanced_metallurgy` requires `basic_metallurgy`), and all current techs require
a `research_lab`.

---

## Adding a New Technology

1. Add an entry to `data/research.json` with a unique key.
2. Set `duration`, optional `cost` (materials), `required_building` (typically `"research_lab"`), `required_int`, and optional `requires_tech`.
3. Gate content on it by checking `KnownTechs` (or via another tech's `requires_tech`); there is no `unlocks` field.
4. No Go code changes needed for a standard research entry.
