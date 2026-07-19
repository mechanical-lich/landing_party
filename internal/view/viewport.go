// Package view owns the on-screen world camera: the mapping between world tile
// coordinates and screen pixels, the view rect (Viewport) used to gate FOV and
// positional audio, and the HUD-sidebar layout the camera centers around. It is
// pure view/render state and depends on no other game package, so world, audio,
// and the game loop can all import it without cycles.
package view

// Viewport is the on-screen world rect currently displayed: its top-left tile
// (X,Y), the displayed z-level (Z), and its width/height in tiles. It is the
// minimal value the world-level consumers need — FOV's visible-clear and the
// positional-audio frustum gate — so they take a Viewport rather than the whole
// Camera. A Camera produces one via Camera.Viewport.
type Viewport struct {
	X, Y, Z int
	W, H    int
}
