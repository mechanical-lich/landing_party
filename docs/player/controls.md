# Controls

## Camera

| Key | Action |
|-----|--------|
| W | Pan camera north |
| S | Pan camera south |
| A | Pan camera west |
| D | Pan camera east |

## Z-Levels

| Key | Action |
|-----|--------|
| Q | Move up one Z-level |
| E | Move down one Z-level |

## Map

| Key / Input | Action |
|-------------|--------|
| M | Toggle the full-screen minimap modal |
| ESC (modal open) | Close the minimap modal |
| ← / → (modal open) | Browse to the previous / next Z-level |
| Drag (modal open) | Pan the map viewport |
| Double-click tile (modal open) | Move the camera to that tile and close the modal |

A smaller **minimap widget** is always visible in the top-right corner of the screen. It shows the current Z-level centered on the camera view. Click it to open the full minimap modal.

Entity dots on the minimap use the following color coding:

| Color | Entity type |
|-------|-------------|
| Green | Colonist |
| Red | Hostile creature |
| Yellow | Other entity |

## Expedition / Star Map

| Input | Action |
|-------|--------|
| O | Open the Star Map (parks the current location; nothing is lost) |
| Star Map button | Same as O — top-right of the screen, just left of Follow |
| B | Beam the selected colonist up to the ship roster (must be open to the sky) |

On the Star Map: select a location and **Travel Here** to move the ship there
(spends fuel), **Beam Down >** / **< Beam Up** to move the selected colonist
between the ship and the loaded location, **Resume / Land** (or `Esc`) to enter
the location and play, and **Save Expedition** to save. See
[The Ship & the Star Map](star_map.md).

## Default Mode

When no order is selected the cursor is in Default mode.

| Input | Action |
|-------|--------|
| Left click entity | Select / open detail panel |
| Left click tile with pending task | Escalate task (higher priority) |
| Right click tile with pending task | Cancel and remove the task |

## Orders (Dig, Build, Mine, etc.)

Select an order from the Build menu. The active order is shown in the tooltip near the top of the screen beside the sidebar.

| Input | Action |
|-------|--------|
| Left click | Perform the order at that tile |
| Left click + drag | Paint the order across multiple tiles (Build / Dig) |
| Right click | Cancel the active order and return to Default mode |

## Rogue Mode

You can take direct control of a single colonist. Open a colonist's detail
panel and click **[ Take Control (Rogue) ]**, or click **→ Take Control** next
to a colonist in the Population (Pop) sidebar tab.

While controlling a colonist:

- The colonist stops pulling tasks and obeys your input directly.
- The camera follows the controlled colonist automatically.
- The left sidebar is hidden, and an **Exit Control Mode (X)** button appears.
- The game becomes **turn-based**: the world only advances when you act, and
  each of your actions advances it by one of the colonist's turns (faster
  entities still get proportionally more turns).

| Input | Action |
|-------|--------|
| W / A / S / D | Move one tile. Bump a hostile to attack it; bump rock/ore to dig or mine it |
| Q | Go up stairs (only while standing on a staircase) |
| E | Go down stairs (only while standing on a staircase) |
| P | Pick up an item on the current tile |
| Left click tile | Fire an equipped ranged weapon at that tile |
| X / Exit Control Mode button | Leave Rogue mode; the colonist resumes normal work |

Moving into a friendly colonist swaps places with them, so you are never
blocked in by your own settlers. If the controlled colonist dies, Rogue mode
ends automatically.

## Pause Menu

Press **Escape** when no modal is open to bring up the pause menu.

| Option | What it does |
|--------|-------------|
| Save | Save the whole expedition (freezes the current location, then writes the campaign) |
| Load | Load a saved expedition |
| New Game | Return to the title screen |
| Quit | Exit the application |

You can also save from the Star Map via **Save Expedition**.
