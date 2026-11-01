package campaign

import (
	"math"
	"math/rand"
)

// TutorialLocationID is the Training Ground's location ID. The tutorial's step
// data refers to it as "tutorial" and to the second destination as "next".
const TutorialLocationID = "tutorial"

// tutorialSeed is the Training Ground's fixed level seed. With a flat uniform
// biome and no features it only shapes the caverns below the surface; it is
// fixed so the map is the same for everyone.
const tutorialSeed = 1701

// tutorialRadiusMin/Max bound the Training Ground's star-map distance from the
// Wreck Site where the ship starts adrift: a real but small fuel cost (5-7 out
// of 400), and always nearer than the generated start system (start_radius 8)
// and every neighbour (nearby_radius_min 9).
const (
	tutorialRadiusMin = 5.2
	tutorialRadiusMax = 7.0
)

// TutorialProgress is the guided first landing's state. Nil on Campaign means
// no tutorial, which covers every older save and any expedition that turned it
// off.
type TutorialProgress struct {
	// StepID is the current step, by ID, so editing the step list keeps saves
	// valid. "" means the first step.
	StepID  string `json:"step_id"`
	Done    bool   `json:"done,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
	// NextLocationID is the second destination: the generated start system.
	NextLocationID string `json:"next_location_id"`
	// Unlocked is the Travel lock's allow list.
	Unlocked []string `json:"unlocked,omitempty"`
	// Baselines holds counts captured when the current step started, for the
	// "above baseline" flags, so a reload mid-step does not move the goalposts.
	Baselines map[string]int `json:"baselines,omitempty"`
	// PadPlaced records that the second planet was given a supply pad because
	// it had no container of its own.
	PadPlaced bool `json:"pad_placed,omitempty"`
}

// Active reports whether the tutorial is still guiding this campaign.
func (t *TutorialProgress) Active() bool {
	return t != nil && !t.Done && !t.Skipped
}

// IsUnlocked reports whether the Travel lock allows a jump to locID.
func (t *TutorialProgress) IsUnlocked(locID string) bool {
	if t == nil {
		return true
	}
	for _, id := range t.Unlocked {
		if id == locID {
			return true
		}
	}
	return false
}

// Unlock adds locID to the Travel lock's allow list once.
func (t *TutorialProgress) Unlock(locID string) {
	if t == nil || locID == "" || t.IsUnlocked(locID) {
		return
	}
	t.Unlocked = append(t.Unlocked, locID)
}

// TutorialActive reports whether the campaign's tutorial is running.
func (c *Campaign) TutorialActive() bool {
	return c != nil && c.Tutorial.Active()
}

// Travel lock reasons, shown on the Star Map where the Travel button is. Each
// names the one place the ship may go instead.
const (
	lockFirstLanding = "Travel locked: go to the Training Ground first"
	lockFirstBuild   = "Travel locked: finish the first build at "
)

// TutorialTravelLock returns why the ship may not jump to toID while the
// tutorial runs, or "" if it may. It reads saved state only, so a loaded game
// is locked correctly from the first frame. The check is an allow list, so
// worlds charted later (scans, contracts) are locked without extra code. The
// current location never reports a lock; TravelToIncident bypasses it on
// purpose.
func (c *Campaign) TutorialTravelLock(toID string) string {
	if c == nil {
		return ""
	}
	t := c.Tutorial
	if !t.Active() || toID == c.CurrentLocationID || t.IsUnlocked(toID) {
		return ""
	}
	if len(t.Unlocked) < 2 {
		return lockFirstLanding
	}
	next := t.NextLocationID
	if loc := c.Locations[next]; loc != nil && loc.Name != "" {
		next = loc.Name
	}
	return lockFirstBuild + next
}

// AddTutorialLocation adds the Training Ground to a freshly generated campaign
// and starts its tutorial progress. The ship stays adrift at the Wreck Site;
// the Training Ground becomes the nearest destination from there, nearer than
// the generated start system, which is the tutorial's second destination. The
// placement draws from its own RNG so the campaign's seeded generation and
// GenSeq are untouched: the same seed gives the same galaxy with the tutorial
// on or off, apart from this one location.
func AddTutorialLocation(c *Campaign) *Location {
	if c == nil || c.Locations == nil {
		return nil
	}
	if loc := c.Locations[TutorialLocationID]; loc != nil {
		return loc
	}
	from := c.CurrentLocation()
	sx, sy := 0.0, 0.0
	if from != nil {
		sx, sy = from.X, from.Y
	}
	next := StartLocationID
	if c.Locations[next] == nil {
		next = c.CurrentLocationID
	}
	// Away from Home: the opposite bearing, within a 120 degree arc, so the
	// Training Ground stays clear of the dashed Home line.
	awayAng := 0.0
	if home := c.HomeLocation(); home != nil {
		awayAng = math.Atan2(home.Y-sy, home.X-sx) + math.Pi
	}
	rng := rand.New(rand.NewSource(c.Seed*7919 + 17))
	x, y := c.placeSeparated(func() (float64, float64) {
		ang := awayAng + (rng.Float64()-0.5)*(2*math.Pi/3)
		r := tutorialRadiusMin + rng.Float64()*(tutorialRadiusMax-tutorialRadiusMin)
		return sx + math.Cos(ang)*r, sy + math.Sin(ang)*r
	})
	loc := &Location{
		ID:         TutorialLocationID,
		Name:       "Training Ground",
		Kind:       "planet",
		MapID:      "tutorial",
		ScenarioID: "tutorial",
		Seed:       tutorialSeed,
		Discovered: true,
		X:          x,
		Y:          y,
		Summary:    "A small, quiet world with a supply pad, a rock outcrop and food on the ground. The guided first landing.",
		Tutorial:   true,
	}
	c.Locations[loc.ID] = loc
	c.Tutorial = &TutorialProgress{
		NextLocationID: next,
		Unlocked:       []string{TutorialLocationID},
	}
	return loc
}
