# Landing Party

> ## Legacy branch: a time capsule
>
> This branch preserves Landing Party as it stood on **2026-11-19** (`dev` at
> `43cccb1`), just before the project changed direction. It is not developed
> further. Work continues on `dev`.
>
> **Why it exists.** The game was about to shift from an indirect-control RTS
> toward a simulation: a galaxy that is no longer wholly dead, with major
> factions competing across it and minor peoples living in between. This
> branch keeps the game as it was before that shift, to play, to refer back
> to, and to build other ideas from.
>
> **The story in this version**
>
> - You are a damaged ship's AI leading a party of colonist "blanks", as
>   described below.
> - **The galaxy is dead.** Peoples lived here, rose as far as the stars, and
>   are gone. Each galaxy's past is generated: its peoples, their wars and
>   how each world ended. What is left is ruins, graves, wrecks and datapads,
>   and the things that outlived their makers.
> - **Everyone alive here came from outside.** Each expedition, yours and
>   every rival's, arrives at its own point on the galaxy's rim.
> - **There is one gate, and it leads out.** It is switched off. Its controls
>   are locked in a guarded vault on the world beside it. Switch them on, fly
>   to the gate with enough fuel, and leave: that is the only way to win.
>   Otherwise a run goes on until your people are gone.
> - **The rivals:** xenos and troids (which are more than they seem), the
>   autonz machines, the greys with their saucers and mothership, and rival
>   colonies like your own, which look for the gate and leave through it.
>   The hive, the autonz and the greys can also be played.
>
> **What is in it**
>
> - A whole galaxy generated at the start (40, 80 or 150 systems in one of
>   six shapes), hidden until seen, identified by scanning.
> - Four rival factions as data-driven doctrines, each run by its own
>   director; several factions of one race; groups that decide who fights
>   whom; rival choices on New Expedition.
> - Playable hive, autonz and grey runs beside the colony.
> - Generated history, Legends, the Encyclopedia, quests and contracts.
> - Save version 5.
>
> **What was planned and never built here** (the plans are in
> [docs/brainstorm](docs/brainstorm)): campaign objectives per race, new
> biomes, worlds and wildlife from existing art, and revealing the hive's
> hidden identity in play.
>
> Start with the [Player Guide](docs/player/README.md); the
> [Developer Guide](docs/developer/README.md) and
> [docs/developer/factions.md](docs/developer/factions.md) describe how it
> works.

You are a damaged ship's AI.  Lead a group of alien colonist "blanks" aboard a space life-raft.  Blanks are backup bodies for your usual inhabitants, however, due to damage the bioprinter is unable to restore their personalities from backup. 
Every expedition begins adrift at the Wreck Site, an empty point in space with
nothing to land on, so the first act is choosing a destination. From the **Star Map**
you choose where to make planetfall, spend **fuel** to travel between planets,
moons, asteroid fields and stations, and beam a landing party down to gather
resources, build, and refine more fuel to keep exploring. Locations you
establish are paused when you leave and resumed exactly as you left them.

On the ground it is an indirect-control RTS: each colonist has its own needs
that must be met for them to carry out the tasks that build the settlement.
Hostiles and hazards probe your colony; keep your people alive and supplied.

See the [Player Guide](docs/player/README.md) (start with
[The Ship & the Star Map](docs/player/star_map.md)) and the
[Developer Guide](docs/developer/README.md)
([Campaign & Overworld](docs/developer/campaign.md)).


## Setup
This project used ebiten and installation instructions can be found here: https://ebiten.org/documents/install.html

If you are on Ubuntu chances are all you need is:

`sudo apt install libc6-dev libglu1-mesa-dev libgl1-mesa-dev libxcursor-dev libxi-dev libxinerama-dev libxrandr-dev libxxf86vm-dev libasound2-dev pkg-config`


Go modules fetch the Go dependencies automatically the first time you build or run — no separate step is needed. To pre-fetch them: `go mod download`.

## Running/Building
`make run`

`make build`

To experiment with location generation, run the Planet Generator tool: `make run-planet-generator`. See [World Generation](docs/developer/world_generation.md#planet-generator).

To test with a local copy of the mlge library add this to the mod file.  It assumes that game_engine was cloned in the same parent directory as this repo. 

`replace github.com/mechanical-lich/mlge => ../mlge`


## Controls
- **WASD**: Move camera
- **Q/E**: Up/Down Z Level
- **Scroll Wheel/Pinch**: Zoom in/out
- **M**: Minimap hotkey
- **O** / Dashboard button: Open the Dashboard (star map, crew, inventory, quests, encyclopedia)
- **B**: Beam the selected colonist up to the ship
- **Esc**: Open pause menu

See [docs/player/controls.md](docs/player/controls.md) for the full reference.

**Default mode**
- **Left Click**: Select entity / escalate task at tile
- **Right Click**: Cancel task at tile

**With an order selected (Dig, Build, etc.)**
- **Left Click**: Perform the order (drag to paint area)
- **Right Click**: Cancel order, return to Default


## Credits

- [Oryx Design](https://www.oryxdesignlab.com/home)
- [Lotovik's autotiling template](https://lotovik.itch.io/tile47-autotiling)
- [Kenney](https://kenney.nl/)
- [Alexandr Zhelanov](https://soundcloud.com/alexandr-zhelanov) [OpenGameArt](https://opengameart.org/content/undead-cyborg)
- [maxstack](https://opengameart.org/content/nebula)
- bart [creepy scifi synth](https://opengameart.org/content/creepy-sci-fi-synth-loop) [alien ruins](https://opengameart.org/content/alien-ruins)
- [yd](https://opengameart.org/content/another-space-background-track)