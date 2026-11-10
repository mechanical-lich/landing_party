package game

import (
	"fmt"
	"testing"
)

// The same campaign seed plays the same way: two short hive soaks on one
// seed in one process give the same row (review finding 9: the global
// math/rand source was unseeded, so every hive body's sleep roll, script
// rnd_int, GameMaster spawn and combat die differed run to run; and mlge's
// stopped-task delay is five real seconds, which deterministicTaskStops
// takes out of a headless run).
func TestHiveSoakDeterministic(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	ticks := envInt("LP_DETERMINISM_TICKS", 6000)
	a := hiveSoakOne(t, 3, ticks)
	b := hiveSoakOne(t, 3, ticks)
	if fa, fb := fmt.Sprintf("%+v", a), fmt.Sprintf("%+v", b); fa != fb {
		t.Errorf("same seed, two runs:\n%s\n%s", fa, fb)
	}
}
