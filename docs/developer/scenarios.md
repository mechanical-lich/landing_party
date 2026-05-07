# Scenarios — Developer Guide

Scenarios are defined as JSON files in `data/scenarios/`. Each file configures spawn rules, win/loss conditions, and world parameters for one game mode. The scenario is selected at game start.

---

## Scenario File Structure

```json
{
    "id": "survival",
    "name": "Survival",
    "description": "Survive 30 days on a hostile alien world.",
    "enabled": true,
    "setup_scripts": ["data/scripts/scenarios/survival_setup.basic"],
    "hostile_max": 20,
    "lighting": { "mode": "day_night" },
    "spawn_rules": { ... },
    "win_conditions": { "rules": [ ... ] }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique scenario identifier |
| `name` | string | Display name |
| `description` | string | Short description shown at scenario selection |
| `enabled` | bool | Whether the scenario appears in the picker |
| `setup_scripts` | []string | Paths to `.basic` scripts run once at scenario start |
| `hostile_max` | int | Maximum concurrent hostile entities before spawning pauses |
| `lighting.mode` | string | Lighting mode: `"day_night"`, `"fixed"`, or `"pitch_dark"` |
| `spawn_rules` | object | Spawn rules keyed by blueprint ID (see below) |
| `win_conditions.rules` | []Rule | Ordered list of win/loss rules evaluated each turn |

---

## Spawn Rules

`spawn_rules` is a JSON object keyed by blueprint ID. Each entry configures how that entity type spawns.

```json
"spawn_rules": {
    "alien_grunt": {
        "spawn_rate": 40,
        "light_min": 0,
        "tiles": ["regolith", "dirt"],
        "min_z": 5,
        "max_z": 5
    }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `spawn_rate` | int | Inverse probability — lower = more frequent. Checked as `rnd(spawn_rate) == 0` each turn. |
| `light_min` | int | Minimum ambient light level required for this entity to spawn |
| `tiles` | []string | Tile types the entity may spawn on |
| `min_z` | int | Minimum Z-level to spawn on |
| `max_z` | int | Maximum Z-level to spawn on |

Spawning stops for all rules when the total hostile count reaches `hostile_max`.

---

## Win and Loss Conditions

Rules are evaluated each turn in order. The first rule whose condition is satisfied determines the outcome.

```json
{
    "id": "survive_30_days",
    "trigger": "days_survived",
    "threshold": 30,
    "op": "gte",
    "result": "win",
    "outcome": "victory",
    "message": "Your colony survived 30 days on this hostile world. Victory!"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique rule identifier |
| `trigger` | string | Event or counter to evaluate (`"days_survived"`, `"colonist_eliminated"`) |
| `threshold` | int | Value to compare against |
| `op` | string | Comparison operator: `"gte"`, `"lte"`, `"eq"` |
| `result` | string | `"win"` or `"lose"` |
| `outcome` | string | Outcome label (`"victory"`, `"defeat"`) |
| `message` | string | Message shown to the player on scenario end |

---

## Lighting Modes

| Mode | Description |
|------|-------------|
| `day_night` | Sun intensity cycles over time; darkness at night reduces visibility |
| `fixed` | Constant lighting level, no cycle |
| `pitch_dark` | No ambient light; only placed `work_light` entities provide visibility |

---

## Adding a New Scenario

1. Create a new file in `data/scenarios/` (e.g., `siege.json`).
2. Set a unique `id` and `enabled: true`.
3. Add entries to `spawn_rules` for each entity type you want to appear.
4. Define at least one win rule and one loss rule in `win_conditions.rules`.
5. Optionally add a setup script in `setup_scripts` for one-time initialization logic.
6. The scenario picker will include it on next load.
