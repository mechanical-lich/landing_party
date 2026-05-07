# Colonists

Colonists are the workforce of your settlement. They act autonomously, pulling tasks from the settlement's task queue and executing them based on proximity and availability.

---

## Stats

Each colonist has three stats that affect how quickly they complete tasks:

| Stat | Abbreviation | Effect |
|------|-------------|--------|
| Strength | Str | Speeds up physical tasks: hauling, construction, mining |
| Intelligence | Int | Speeds up research and crafting tasks |
| Dexterity | Dex | Speeds up gathering, fine manipulation tasks |

Stats act as **multipliers on task duration** — a colonist with high Str will finish a construction job faster than one with low Str.

---

## Needs

### Hunger

Colonists consume energy over time. When energy drops below the **hunger threshold** a colonist will stop whatever they are doing and seek food. If no food is available and energy drops to the **starvation threshold**, they begin taking damage each turn.

Keep a stockpile of food items near your colonists to prevent starvation. A `storage_locker` is the standard container for food and supplies.

---

## Task System

Colonists do not take orders directly. They poll the settlement's **task queue** and claim the nearest unclaimed task they are able to perform. Multiple colonists can contribute to the same task — construction and research tasks track progress on the task object itself, so any colonist working on it advances it.

### Task States

| State | What the colonist is doing |
|-------|--------------------------|
| Idle | Looking for a task to claim |
| Task | Performing a claimed task at its location |
| Haul | Carrying materials to a build site or storage |
| Dropoff | Returning gathered resources to a storage container |
| Find Food | Seeking a food item due to hunger |
| Gather Materials | Collecting resources needed for a task |

---

## Combat

Colonists are not soldiers. They will retaliate when attacked but deal minimal damage and have limited health. Do not rely on colonists to defend the colony — use barriers, airlocks, and blast doors to keep hostiles out.
