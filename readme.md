# Landing Party
Play as a group of intergalatic travelers trying to form a settlement in a harsh new land.   This is an indirect control style RTS.   Each colonist has its own needs that need to be met in order to get them to perform their tasks to build the settlement.   

Sanity must be maintained, meaning in addition to basic survival, expansion, and resource gathering; you will need to ensure your colonists interests are met and explored.   They could also see things that impact their sanity leading to them potentially going crazy and hurting others and go rogue. 


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
- **Esc**: Open pause menu

**Default mode**
- **Left Click**: Select entity / escalate task at tile
- **Right Click**: Cancel task at tile

**With an order selected (Dig, Build, etc.)**
- **Left Click**: Perform the order (drag to paint area)
- **Right Click**: Cancel order, return to Default