# Landing Party
Lead a group of alien colonists aboard a space life-raft. From the **Star Map**
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


After installing ebiten dependencies you'll need to run the following command to get all the go deps.

`make vendor`

## Running/Building
`make run`

`make build`

To test with a local copy of the mlge library add this to the mod file.  It assumes that game_engine was cloned in the same parent directory as this repo. 

`replace github.com/mechanical-lich/mlge => ../mlge`


## Controls
- **WASD**: Move camera
- **Q/E**: Up/Down Z Level
- **Scroll Wheel/Pinch**: Zoom in/out
- **M**: Minimap hotkey
- **O** / Star Map button: Open the Star Map (ship / overworld)
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