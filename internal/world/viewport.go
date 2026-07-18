package world

// Viewport is the on-screen world rect the game is currently displaying: its
// top-left tile (X,Y), the displayed z-level (Z), and its width/height in tiles.
// It is view/render state owned by the game layer, not part of the world data
// model — it is passed to the few world-level consumers that need it (FOV clear,
// positional-audio frustum gate) rather than stored on Level.
type Viewport struct {
	X, Y, Z int
	W, H    int
}
