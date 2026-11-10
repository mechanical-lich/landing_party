//go:debug randseednop=0

package game

import (
	"math/rand"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/workerai"
	"github.com/mechanical-lich/mlge/task"
)

// seedGlobalRand seeds the global math/rand source that the AI, needs,
// combat and dice code draw from (here and in ml-rogue-lib and mlge), so a
// long headless run plays the same way every time. Go 1.24 made rand.Seed a
// no-op; the go:debug line above turns it back on for this test binary.
func seedGlobalRand(t *testing.T, seed int64) {
	t.Helper()
	rand.Seed(seed) //nolint:staticcheck // deliberate: the game's global source
}

// deterministicTaskStops makes a worker's stopped task go straight back to
// the queue (task.ReQueue) instead of mlge's five-real-second restart delay
// (task.Stop), which made a headless run depend on CPU load: thousands of
// ticks pass in five seconds at soak speed. A long run then plays the same
// way every time for the same seed (with seedSimRand, from NewWorldManager).
func deterministicTaskStops(t *testing.T) {
	t.Helper()
	prev := workerai.StopTask
	workerai.StopTask = (*task.Task).ReQueue
	t.Cleanup(func() { workerai.StopTask = prev })
}
