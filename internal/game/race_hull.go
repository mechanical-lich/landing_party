package game

import (
	"fmt"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/ship"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/message"
)

// Growing a saucer (phase F step 6d; greyish_plan.md, "The saucer"): a grey
// player's hull grows in rings (Expand Hull on a deck) and decks (Add Deck
// above the top one), each step its doctrine's ship block lists, some gated
// by research. An upgrade is physical work, queued as ordinary orders in
// phases, each queued once the last is done:
//
//	expand: breach (one old rim tile dug, so the crew can reach the ring),
//	        floor (the new ring, spreading out from the breach), open (the
//	        rest of the old rim dug out: the ring is a tile wide in places,
//	        so a new rim raised first would cut parts of it off from the
//	        breach), rim and wall (the new outline: its stepped corners
//	        first, while the tiles beside them are still open). The ship
//	        has no vacuum, so nothing is exposed meanwhile.
//	deck:   stairs (up from the deck below), floor, rim, wall.
//
// The hull's shape and the job in progress live in the ship level's flags,
// so they are saved with it: hull_r:<z> (each deck's radius), hull_decks,
// hull_job ("expand <z> <from> <to> <phase>" or "deck <z> <r> <phase>").

const (
	flagHullDecks = "hull_decks"
	flagHullJob   = "hull_job"
)

func flagHullRadius(z int) string { return fmt.Sprintf("hull_r:%d", z) }

// hullJob is an upgrade in progress.
type hullJob struct {
	Kind     string // "expand" or "deck"
	Z        int
	From, To int // radii (a deck: From 0)
	Phase    string
}

func (j hullJob) String() string {
	return fmt.Sprintf("%s %d %d %d %s", j.Kind, j.Z, j.From, j.To, j.Phase)
}

func readHullJob(level *world.Level) (hullJob, bool) {
	s, _ := level.Flags[flagHullJob].(string)
	var j hullJob
	if s == "" {
		return j, false
	}
	if _, err := fmt.Sscan(s, &j.Kind, &j.Z, &j.From, &j.To, &j.Phase); err != nil {
		return j, false
	}
	return j, true
}

// saucerSpec is the player's saucer when this state is its ship (nil else).
func (s *MainState) saucerSpec() *campaign.SaucerSpec {
	if !s.aboardShip() || s.level == nil {
		return nil
	}
	return playerSettlementOf(s.campaign).SaucerShip()
}

func flagInt(level *world.Level, key string) (int, bool) {
	switch v := level.Flags[key].(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	}
	return 0, false
}

// hullDecks is how many decks the saucer has.
func hullDecks(level *world.Level, sp *campaign.SaucerSpec) int {
	if n, ok := flagInt(level, flagHullDecks); ok {
		return n
	}
	return max(sp.Start.Decks, 1)
}

// hullRadius is deck z's radius (0: no deck there).
func hullRadius(level *world.Level, sp *campaign.SaucerSpec, z int) int {
	if r, ok := flagInt(level, flagHullRadius(z)); ok {
		return r
	}
	if z >= 0 && z < hullDecks(level, sp) {
		return sp.Start.Radius
	}
	return 0
}

// nextRing is the ring deck z grows to next (ok false: at its widest).
func nextRing(sp *campaign.SaucerSpec, r int) (campaign.SaucerStep, bool) {
	for _, st := range sp.Rings {
		if st.Radius > r {
			return st, true
		}
	}
	return campaign.SaucerStep{}, false
}

// nextDeck is the deck the saucer adds next (ok false: all of them).
func nextDeck(sp *campaign.SaucerSpec, decks int) (campaign.SaucerStep, bool) {
	for _, st := range sp.Decks {
		if st.Deck == decks && decks < ship.LevelDepth {
			return st, true
		}
	}
	return campaign.SaucerStep{}, false
}

// hullGate is "" when step may be built, else why not (DRAFT text).
func (s *MainState) hullGate(step campaign.SaucerStep) string {
	if step.RequiresTech == "" || s.campaign.HasTech(step.RequiresTech) {
		return ""
	}
	name := step.RequiresTech
	if t, ok := researchTech(step.RequiresTech); ok {
		name = t
	}
	return "needs " + name // DRAFT
}

// expandHull starts growing deck z to its next ring.
func (s *MainState) expandHull(z int) error {
	sp := s.saucerSpec()
	if sp == nil {
		return fmt.Errorf("only a saucer grows") // DRAFT
	}
	if j, ok := readHullJob(s.level); ok {
		return fmt.Errorf("the hull is already being worked on (%s)", j.Kind) // DRAFT
	}
	r := hullRadius(s.level, sp, z)
	if r == 0 {
		return fmt.Errorf("there is no deck %d", z+1) // DRAFT
	}
	step, ok := nextRing(sp, r)
	if !ok {
		return fmt.Errorf("deck %d is as wide as it grows", z+1) // DRAFT
	}
	if g := s.hullGate(step); g != "" {
		return fmt.Errorf("the next ring %s", g) // DRAFT
	}
	if z > 0 && step.Radius > hullRadius(s.level, sp, z-1) {
		return fmt.Errorf("a deck is never wider than the deck below it") // DRAFT
	}
	s.startHullJob(hullJob{Kind: "expand", Z: z, From: r, To: step.Radius, Phase: "breach"})
	return nil
}

// addDeck starts a new deck above the top one, at the start radius (no
// wider than the deck below).
func (s *MainState) addDeck() error {
	sp := s.saucerSpec()
	if sp == nil {
		return fmt.Errorf("only a saucer grows") // DRAFT
	}
	if j, ok := readHullJob(s.level); ok {
		return fmt.Errorf("the hull is already being worked on (%s)", j.Kind) // DRAFT
	}
	decks := hullDecks(s.level, sp)
	step, ok := nextDeck(sp, decks)
	if !ok {
		return fmt.Errorf("the saucer has all its decks") // DRAFT
	}
	if g := s.hullGate(step); g != "" {
		return fmt.Errorf("the next deck %s", g) // DRAFT
	}
	r := min(sp.Start.Radius, hullRadius(s.level, sp, decks-1))
	s.startHullJob(hullJob{Kind: "deck", Z: decks, To: r, Phase: "stairs"})
	return nil
}

func (s *MainState) startHullJob(j hullJob) {
	s.level.Flags[flagHullJob] = j.String()
	s.queueHullPhase(j)
}

// hullStairs is where a deck's stairs up from the deck below stand.
func hullStairs() (int, int) {
	cx, cy, _ := ship.HullCenter()
	return cx + 2, cy
}

// hullBreach is the old rim tile an expansion digs first: the rim's east
// point.
func hullBreach(r int) [2]int {
	rr := world.NewRoundRoom(r)
	best := [2]int{r, 0}
	if !rr.Wall[best] {
		for c := range rr.Wall {
			if c[1] == 0 && c[0] > 0 {
				best = c
			}
		}
	}
	return best
}

// hullPhaseCells is the tiles a phase works on, by offset from the hull
// centre, in a fixed order.
func hullPhaseCells(j hullJob) [][2]int {
	var out [][2]int
	switch {
	case j.Kind == "expand" && j.Phase == "breach":
		out = append(out, hullBreach(j.From))
	case j.Kind == "expand" && j.Phase == "floor":
		old := world.NewRoundRoom(j.From)
		for _, c := range world.NewRoundRoom(j.To).Cells() {
			if !old.In(c[0], c[1]) {
				out = append(out, c)
			}
		}
	case j.Phase == "rim":
		hard, _ := rimCells(j.To)
		out = hard
	case j.Phase == "wall":
		_, soft := rimCells(j.To)
		out = soft
	case j.Kind == "expand" && j.Phase == "open":
		out = sortedCells(world.NewRoundRoom(j.From).Wall)
	case j.Kind == "deck" && j.Phase == "stairs":
		x, y := hullStairs()
		cx, cy, _ := ship.HullCenter()
		out = append(out, [2]int{x - cx, y - cy})
	case j.Kind == "deck" && j.Phase == "floor":
		sx, sy := hullStairs()
		cx, cy, _ := ship.HullCenter()
		for _, c := range world.NewRoundRoom(j.To).Cells() {
			if c != [2]int{sx - cx, sy - cy} {
				out = append(out, c)
			}
		}
	}
	return out
}

// rimCells splits radius r's outline: hard cells (no four-neighbour inside
// the hull that is not itself outline: a stepped corner, reachable only
// from the outline tiles beside it) and the rest. The hard ones are walled
// first, while the outline beside them is still open floor.
func rimCells(r int) (hard, soft [][2]int) {
	rr := world.NewRoundRoom(r)
	for _, c := range sortedCells(rr.Wall) {
		open := false
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := [2]int{c[0] + d[0], c[1] + d[1]}
			if rr.In(n[0], n[1]) && !rr.Wall[n] {
				open = true
			}
		}
		if open {
			soft = append(soft, c)
		} else {
			hard = append(hard, c)
		}
	}
	return hard, soft
}

func sortedCells(m map[[2]int]bool) [][2]int {
	out := make([][2]int, 0, len(m))
	for c := range m {
		out = append(out, c)
	}
	for i := 1; i < len(out); i++ {
		for k := i; k > 0 && (out[k][1] < out[k-1][1] || out[k][1] == out[k-1][1] && out[k][0] < out[k-1][0]); k-- {
			out[k], out[k-1] = out[k-1], out[k]
		}
	}
	return out
}

// hullPhaseZ is the deck a phase's tiles are on (a deck's stairs stand on
// the deck below).
func hullPhaseZ(j hullJob) int {
	if j.Kind == "deck" && j.Phase == "stairs" {
		return j.Z - 1
	}
	return j.Z
}

// hullCellDone reports whether one tile of a phase is done.
func hullCellDone(level *world.Level, sp *campaign.SaucerSpec, j hullJob, x, y, z int) bool {
	t := level.GetTilePtr(x, y, z)
	if t == nil {
		return true
	}
	switch j.Phase {
	case "breach", "open":
		return t.Middle.IsEmpty()
	case "floor":
		return !t.Floor.IsEmpty()
	case "rim", "wall":
		return !t.Middle.IsEmpty()
	case "stairs":
		return !t.Middle.IsEmpty() && world.TileIndexToName[t.Middle.Type] == "stairs_up"
	}
	return true
}

// hullStandable reports a tile the crew can stand on to work beside it:
// floor with nothing in the middle, or a stair (a new deck's way up has no
// floor of its own).
func hullStandable(level *world.Level, x, y, z int) bool {
	t := level.GetTilePtr(x, y, z)
	if t == nil {
		return false
	}
	if !t.Middle.IsEmpty() {
		n := world.TileIndexToName[t.Middle.Type]
		return n == "stairs_down" || n == "stairs_up"
	}
	return !t.Floor.IsEmpty()
}

// hullFrontier reports a floor tile the crew can reach now: one beside (of
// four) a tile they can stand on. A floor phase orders only these, so the
// floor spreads out from the breach (or the stairs) and no order is posted
// that no one can reach (each would fail its path and wait).
func hullFrontier(level *world.Level, x, y, z int) bool {
	for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		if hullStandable(level, x+d[0], y+d[1], z) {
			return true
		}
	}
	return false
}

// queueHullPhase orders the phase's open tiles: digs for a breach and the
// old rim, builds of the doctrine's floor (the reachable frontier), wall or
// the stairs.
func (s *MainState) queueHullPhase(j hullJob) {
	sp := s.saucerSpec()
	if sp == nil || s.MainSettlement == nil {
		return
	}
	cx, cy, _ := ship.HullCenter()
	z := hullPhaseZ(j)
	for _, c := range hullPhaseCells(j) {
		x, y := cx+c[0], cy+c[1]
		if hullCellDone(s.level, sp, j, x, y, z) {
			continue
		}
		switch j.Phase {
		case "breach", "open":
			orderDig(s.MainSettlement, s.level, x, y, z)
		case "floor":
			if hullFrontier(s.level, x, y, z) {
				orderBuild(s.MainSettlement, s.level, sp.Floor, x, y, z)
			}
		case "rim", "wall":
			orderBuild(s.MainSettlement, s.level, sp.Wall, x, y, z)
		case "stairs":
			orderBuild(s.MainSettlement, s.level, "stairs_up", x, y, z)
		}
	}
}

// hullPhaseLeft is how many of the phase's tiles are not done.
func (s *MainState) hullPhaseLeft(j hullJob) int {
	sp := s.saucerSpec()
	if sp == nil {
		return 0
	}
	cx, cy, _ := ship.HullCenter()
	z := hullPhaseZ(j)
	n := 0
	for _, c := range hullPhaseCells(j) {
		if !hullCellDone(s.level, sp, j, cx+c[0], cy+c[1], z) {
			n++
		}
	}
	return n
}

// hullPhases is each kind's phases in order.
var hullPhases = map[string][]string{
	"expand": {"breach", "floor", "open", "rim", "wall"},
	"deck":   {"stairs", "floor", "rim", "wall"},
}

// stepHull advances an upgrade in progress once a second: a phase done
// queues the next; the last done records the deck's new radius (or the new
// deck) and ends the job. An order lost on the way (cancelled, refused) is
// queued again.
func (s *MainState) stepHull() {
	if s.tick%30 != 0 || s.level == nil {
		return
	}
	j, ok := readHullJob(s.level)
	if !ok || s.saucerSpec() == nil {
		return
	}
	if s.hullPhaseLeft(j) > 0 {
		if j.Phase == "floor" || s.tick%300 == 0 {
			s.queueHullPhase(j) // the floor's new frontier; anything dropped
		}
		return
	}
	phases := hullPhases[j.Kind]
	for i, p := range phases {
		if p != j.Phase {
			continue
		}
		if i+1 < len(phases) {
			j.Phase = phases[i+1]
			s.startHullJob(j)
			return
		}
	}
	s.level.Flags[flagHullRadius(j.Z)] = float64(j.To)
	if j.Kind == "deck" {
		s.level.Flags[flagHullDecks] = float64(j.Z + 1)
	}
	delete(s.level.Flags, flagHullJob)
	postHullMessage(fmt.Sprintf("Deck %d's hull is complete: radius %d.", j.Z+1, j.To)) // DRAFT
}

// hullLines is the Hull section's lines (the research modal; DRAFT text):
// each deck's radius, the next ring and deck with their gate, the job in
// progress.
func (s *MainState) hullLines() []string {
	sp := s.saucerSpec()
	if sp == nil {
		return nil
	}
	var out []string
	decks := hullDecks(s.level, sp)
	for z := 0; z < decks; z++ {
		r := hullRadius(s.level, sp, z)
		line := fmt.Sprintf("Deck %d: radius %d", z+1, r)
		if st, ok := nextRing(sp, r); ok {
			line += fmt.Sprintf("; next ring %d", st.Radius)
			if g := s.hullGate(st); g != "" {
				line += " (" + g + ")"
			}
		} else {
			line += "; at its widest"
		}
		out = append(out, line)
	}
	if st, ok := nextDeck(sp, decks); ok {
		line := fmt.Sprintf("Next deck: deck %d", st.Deck+1)
		if g := s.hullGate(st); g != "" {
			line += " (" + g + ")"
		}
		out = append(out, line)
	} else {
		out = append(out, "All decks built")
	}
	if j, ok := readHullJob(s.level); ok {
		out = append(out, fmt.Sprintf("Working: %s deck %d, %s (%d tiles left)", strings.Title(j.Kind), j.Z+1, j.Phase, s.hullPhaseLeft(j))) //nolint:staticcheck // DRAFT text
	}
	return out
}

// researchTech is a research entry's name.
func researchTech(key string) (string, bool) {
	if t, ok := research.GetTech(key); ok {
		return t.Name, true
	}
	return "", false
}

func postHullMessage(text string) { message.PostMessage("Hull", text) }

// hullOrder reports an Expand Hull or Add Deck click: its refusal, else ok.
func (s *MainState) hullOrder(err error, ok string, args ...any) {
	if err != nil {
		postHullMessage(err.Error())
		return
	}
	postHullMessage(fmt.Sprintf(ok, args...))
}
