// Package chatter is the one door through which worker and needs chatter
// ("Ate ration", "Stored items", "Too hungry to continue") reaches the player's
// message log. Several planets can tick at once, but the log belongs to the live
// one, so chatter posted while a background level is stepping is dropped here
// instead of every caller having to know which level it is on.
//
// Important simulation events (kills, structures built, research) do not come
// through here: they are per-level bus events whose listeners route background
// lines to the dashboard notifications.
package chatter

import "github.com/mechanical-lich/mlge/message"

var background bool

// SetBackground marks the start (true) or end (false) of a background level's
// step. MainState.stepBackground sets it alongside level.Background.
func SetBackground(on bool) { background = on }

// Background reports whether a background step is in progress.
func Background() bool { return background }

// Post sends a line to the message log unless a background step is running.
func Post(sender, msg string) {
	if background {
		return
	}
	message.PostMessage(sender, msg)
}
