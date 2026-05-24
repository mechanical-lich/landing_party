# Colonists

Colonists are the workforce of your settlement. They act autonomously, pulling tasks from the settlement's task queue and executing them based on proximity and availability.

---

## Stats

Each colonist has four stats:

| Stat | Abbreviation | Effect |
|------|-------------|--------|
| Strength | Str | Increases melee damage |
| Dexterity | Dex | Increases melee and ranged attack accuracy |
| Intelligence | Int | Required minimum to perform high-tier research tasks |
| Constitution | Con | Reserved — no effect yet |

---

## Stat Progression

Colonists improve their stats through experience. Each time a colonist completes a task, they earn **25 XP** toward the relevant stat:

| Task | Stat gained |
|------|------------|
| Dig, Mine, Build | Strength |
| Research, Craft | Intelligence |
| Fetch, Retrieve items | Dexterity |

When a colonist accumulates enough XP, their stat **levels up** — permanently increasing that stat by 1. The XP required to reach each level grows: early levels are quick, later levels take sustained effort. Each stat caps at **10 levels** above its starting value.

Level-ups appear as messages in the event log and are visible in the colonist detail panel (Str Lv.N / Dex Lv.N / Int Lv.N). Constitution is tracked but has no tasks that improve it yet.

---

## Needs

Colonists have two needs that must be managed: **Hunger** and **Exhaustion**. Both are visible in the colonist detail panel and the hover tooltip. A colonist will interrupt their current task when a need becomes urgent, then return to work once it is met.

### Hunger

Hunger builds up over time. When it crosses the **hunger threshold** a colonist stops what they are doing and looks for something to eat — first checking their own inventory, then searching nearby settlement storage. If hunger reaches the **starvation threshold** with no food available, the colonist takes damage each turn.

Keep a stockpile of food items (ration packs, etc.) in a `storage_locker` near your workers. Colonists start with a ration pack in their inventory, which buys time before storage is established.

### Exhaustion

Exhaustion accumulates while colonists work. Physical tasks (digging, mining, building) are more tiring than mental ones (research, crafting). When exhaustion is high enough, the colonist will stop working and look for a bed to sleep in. A proper bed restores exhaustion quickly; with no bed available, the colonist eventually passes out and recovers much more slowly.

Build beds early — one per colonist is ideal. Colonists remember the last bed they slept in and will return to it.

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

---

## Rogue Mode (Direct Control)

You can take direct control of any colonist from its detail panel
(**[ Take Control (Rogue) ]**) or the Population sidebar tab
(**→ Take Control**). The controlled colonist stops taking tasks and responds
to your keyboard and mouse instead, the camera follows them, and the game
switches to turn-based until you exit. See **Controls → Rogue Mode** for the
full key list. Press **X** (or the on-screen button) to hand the colonist back
to the autonomous task system.
