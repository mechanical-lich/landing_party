package campaign

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/lore"
	"github.com/mechanical-lich/landing_party/internal/objective"
)

// Default template paths (data-driven; overridable in tests).
var (
	LocationTemplatePath = "data/location_templates.json"
	QuestTemplatePath    = "data/quest_templates.json"
)

const (
	// separationAttempts bounds how many times placement re-rolls to satisfy
	// MinSeparation before giving up and using the last roll (so gen never stalls).
	separationAttempts = 16
	// nameAttempts bounds how many name rolls we try for uniqueness before
	// falling back to a numeric suffix.
	nameAttempts = 12
)

type locArchetype struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Weight  int      `json:"weight"`
	StartOK bool     `json:"start_ok"`
	Maps    []string `json:"maps"`
	Tags    []string `json:"tags"`
	Summary string   `json:"summary"`
	// SpawnEvent, if set, is the chapter every site of this archetype ends in
	// (a hive world is overrun by xenos); a contract's own spawn_event wins.
	SpawnEvent string `json:"spawn_event,omitempty"`
}

type locTemplates struct {
	Archetypes []locArchetype `json:"archetypes"`
}

type questTemplate struct {
	ID          string `json:"id"`
	Weight      int    `json:"weight"`
	RequiresTag string `json:"requires_tag"`
	// RequiresModule keeps the quest to station sites whose plan has a
	// module of this kind (station_archive needs an archive).
	RequiresModule string `json:"requires_module,omitempty"`
	// Flag is the level flag a flag_set quest waits on ("power:main").
	Flag           string         `json:"flag,omitempty"`
	Name           string         `json:"name"`
	Desc           string         `json:"desc"`
	Trigger        string         `json:"trigger"`
	Resource       string         `json:"resource"`
	Blueprints     []string       `json:"blueprints"`
	Structure      string         `json:"structure"`
	TechPool       []string       `json:"tech_pool"`
	Min            int            `json:"min"`
	Max            int            `json:"max"`
	RewardFuelPer  int            `json:"reward_fuel_per"`
	RewardFuelFlat int            `json:"reward_fuel_flat"`
	RewardRes      map[string]int `json:"reward_resources"`
	SpawnSystems   int            `json:"spawn_systems"`
	// TargetBlueprints / TargetNames drive a target_killed "bounty" quest: a
	// named entity (random blueprint + name from these pools) is spawned as a
	// location fixture and must be killed.
	TargetBlueprints []string `json:"target_blueprints"`
	TargetNames      []string `json:"target_names"`
	// FixtureStructure, when set, names a gen_structure script that builds a
	// lair around the quest's fixture entity on arrival (see QuestFixture).
	FixtureStructure string `json:"fixture_structure"`
	FixtureStructW   int    `json:"fixture_struct_w"`
	FixtureStructH   int    `json:"fixture_struct_h"`
	// SpawnArchetype, when set, makes this a "contract": instead of attaching
	// to a system by its tags, it points at an existing system made from
	// that archetype (e.g. a distress call from a xeno-infested world).
	// Accepting the quest reveals it.
	SpawnArchetype string `json:"spawn_archetype"`
	// SpawnEvent, on a contract, names the history chapter the system should
	// end in (a distress call wants a colony overrun by xenos): a system
	// whose site plays it is preferred.
	SpawnEvent string `json:"spawn_event,omitempty"`
	// History, when set, makes this a history-backed template bound at
	// charting from the site's history (see addHistoryQuest): "figure_log",
	// "civ_relic" or "chapter_origin". Never tag-rolled.
	History string `json:"history,omitempty"`
}

type questTemplates struct {
	Templates []questTemplate `json:"templates"`
}

func loadJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func loadGenTemplates() (*locTemplates, *questTemplates, error) {
	var lt locTemplates
	var qt questTemplates
	if err := loadJSON(LocationTemplatePath, &lt); err != nil {
		return nil, nil, fmt.Errorf("location templates: %w", err)
	}
	if err := loadJSON(QuestTemplatePath, &qt); err != nil {
		return nil, nil, fmt.Errorf("quest templates: %w", err)
	}
	if len(lt.Archetypes) == 0 || len(qt.Templates) == 0 {
		return nil, nil, fmt.Errorf("empty generation templates")
	}
	return &lt, &qt, nil
}

// GenerationConfig exposes the data-driven generation tuning (defaults if the
// file is absent) so callers can read start fuel/colonists/roster cap.
func GenerationConfig() GenConfig { return genConfig() }

// GenerateCampaign builds a campaign on a fresh galaxy of the default size
// (GenerateGalaxy) with the expedition arrived at a rolled landfall.
// Everything is derived from seed and persisted, so loading a save
// reproduces the run exactly. It seeds no rival factions (see
// GenerateCampaignOpts).
func GenerateCampaign(name string, seed int64) (*Campaign, error) {
	return GenerateCampaignOpts(name, seed, GenOptions{})
}

// GenerateCampaignOpts is GenerateCampaign with the New Expedition options:
// the galaxy is made first (GenerateGalaxy, from seed and opts.Size alone),
// then the expedition arrives: a landfall rolled from its own stream, the
// Wreck Site at that landfall's point outside the rim, the landfall system
// its first jump, one opening contract, and opts.Factions rival presences,
// each from a stream of its own, so the galaxy is the same whoever arrives.
func GenerateCampaignOpts(name string, seed int64, opts GenOptions) (*Campaign, error) {
	c, err := generateGalaxy(seed, opts.Size, opts.Shape)
	if err != nil {
		return nil, err
	}
	_, qt, err := loadGenTemplates()
	if err != nil {
		return nil, err
	}
	cfg := genConfig()
	c.Name = name
	c.Ship = NewShipState(cfg.RosterCap)
	c.Player = NewPlayerPresence(opts.Doctrine)
	if c.Player.Faction == PlayerFaction {
		c.Diplomacy = map[string]string{PlayerFaction: StanceNeutral} // a stray Stance lookup
	}

	// The expedition's landfall, rolled from the free ones.
	if len(c.Galaxy.Landfalls) == 0 {
		return nil, fmt.Errorf("galaxy: seed %d has no landfall", seed)
	}
	lrng := rand.New(rand.NewSource(seed*6364136223846793 + 1442695040888963))
	c.Landfall = lrng.Intn(len(c.Galaxy.Landfalls))
	lf := c.Galaxy.Landfalls[c.Landfall]
	c.StartID = lf.System
	// The crew can see the nearest star: the landfall system is identified.
	c.Identify(c.StartLocation())

	// The ship begins adrift at the landfall's point just outside the rim:
	// the Wreck Site, an empty point with nothing to land on, so the first
	// act is choosing a destination. The landfall system is the nearest.
	adrift := &Location{
		ID: AdriftLocationID, Name: "Wreck Site", Kind: AdriftKind,
		Identified: true, Visited: true, X: lf.X, Y: lf.Y,
		Summary: "Where the damaged life-raft came to rest, at the edge of the galaxy. Empty space: nothing to orbit, nothing to land on. Choose a destination.", // DRAFT
	}
	c.Locations[adrift.ID] = adrift
	c.CurrentLocationID = adrift.ID
	// From the landfall the crew can see what lies within its sight range.
	c.SeeAround(adrift)

	// One opening "contract" so a quest-defines-its-destination mission (e.g.
	// a distress call) is offered from the very start.
	erng := rand.New(rand.NewSource(seed*2862933555777941757 + 3037000493))
	if qd := c.makeContract(erng, qt); qd != nil {
		c.QuestDefs = append(c.QuestDefs, *qd)
		c.offerQuest()
	}

	if opts.Rivals != nil {
		// The rival boxes (rival_selection_plan.md step 3): the player's row
		// and group first, so the instances key round it.
		g := opts.PlayerGroup
		if g <= 0 {
			g = PlayerGroup
		}
		c.addFactionRow(c.Player.Faction, c.Player.Doctrine, 0, g)
		c.seedRivals(opts.Rivals)
	} else {
		c.seedFactions(opts.Factions)
	}
	c.Observer = opts.Observer
	c.BindQuests()
	return c, nil
}

// StartLocation is the expedition's landfall system: its first jump.
func (c *Campaign) StartLocation() *Location {
	if c == nil {
		return nil
	}
	return c.Locations[c.StartID]
}

// RevealCharts is a quest's star-chart reward: it makes known the n systems
// nearest the ship that are not known yet, and returns them for the caller
// to announce. Nothing is created.
func (c *Campaign) RevealCharts(n int) []*Location {
	if n <= 0 {
		return nil
	}
	cur := c.CurrentLocation()
	var cx, cy float64
	if cur != nil {
		cx, cy = cur.X, cur.Y
	}
	var cands []*Location
	for _, l := range c.Systems() {
		if !l.Identified {
			cands = append(cands, l)
		}
	}
	if g := c.GateLocation(); g != nil && !g.Identified {
		cands = append(cands, g) // a chart may show the gate
	}
	sort.SliceStable(cands, func(i, j int) bool {
		return math.Hypot(cands[i].X-cx, cands[i].Y-cy) < math.Hypot(cands[j].X-cx, cands[j].Y-cy)
	})
	if len(cands) > n {
		cands = cands[:n]
	}
	for _, l := range cands {
		l.Identified, l.Seen = true, true
	}
	return cands
}

// MaybeTravelQuests is rolled on every jump to a system: a 1-in-N chance
// (config: travel_quest_one_in) to post 1..travel_quest_max new quests, each
// on an existing system (a contract on one of its archetype, the rest on one
// near the ship). Keeps the run from stalling as long as the player keeps
// moving. Returns the number of quests added.
func (c *Campaign) MaybeTravelQuests() int {
	_, qt, err := loadGenTemplates()
	if err != nil {
		return 0
	}
	cfg := genConfig()
	c.TravelSeq++
	rng := rand.New(rand.NewSource(c.Seed + int64(c.TravelSeq)*2654435761 + int64(c.GenSeq)*131))
	if rng.Intn(cfg.TravelQuestOneIn) != 0 {
		return 0
	}
	count := 1 + rng.Intn(cfg.TravelQuestMax)
	hasContracts := len(contractTemplates(qt)) > 0
	added := 0
	for i := 0; i < count; i++ {
		// ~1/3 of rolled quests are "contracts" pointing at a system of their
		// own archetype (e.g. a distress call).
		if hasContracts && rng.Intn(cfg.ContractOneIn) == 0 {
			if qd := c.makeContract(rng, qt); qd != nil {
				c.QuestDefs = append(c.QuestDefs, *qd)
				c.offerQuest()
				added++
				continue
			}
		}
		target := c.questTarget(rng, nil)
		if target == nil {
			continue
		}
		// Roll a template against the target's own tag, so the quest fits
		// where it's posted.
		a := locArchetype{Tags: tagSlice(target.QuestTag)}
		if qd := c.makeQuest(rng, qt, a, target); qd != nil {
			c.QuestDefs = append(c.QuestDefs, *qd)
			c.offerQuest()
			added++
		}
	}
	if added > 0 {
		c.BindQuests()
	}
	return added
}

// offerQuest marks the last quest added as news: offered whatever the
// player knows of its system, whose point it puts on the map (seen;
// accepting it identifies the system).
func (c *Campaign) offerQuest() {
	if n := len(c.QuestDefs); n > 0 {
		c.QuestDefs[n-1].Offered = true
		c.See(c.Locations[c.QuestDefs[n-1].Location])
	}
}

// questTarget picks the existing system a posted quest or contract points
// at: one ok accepts (nil: any), never visited, never the ship's own, never
// the Training Ground, weighted toward systems near the ship. nil if none.
func (c *Campaign) questTarget(rng *rand.Rand, ok func(*Location) bool) *Location {
	cur := c.CurrentLocation()
	var cx, cy float64
	if cur != nil {
		cx, cy = cur.X, cur.Y
	}
	var cands []*Location
	for _, l := range c.Systems() {
		if l.Visited || l.ID == c.CurrentLocationID || (ok != nil && !ok(l)) {
			continue
		}
		cands = append(cands, l)
	}
	return pickWeighted(rng, cands, func(l *Location) float64 {
		return 1 / (1 + math.Hypot(l.X-cx, l.Y-cy)/questNearScale)
	})
}

// questNearScale is the distance at which a system is half as likely to get
// a posted quest as one beside the ship.
const questNearScale = 40

func tagSlice(t string) []string {
	if t == "" {
		return nil
	}
	return []string{t}
}

// Scan runs one ship-scanner sweep from the current location: it identifies
// every system within the scanner tier's range (the gate as the gate) and
// shows what holds the worlds there. No chance roll; nothing is created. It
// returns the newly identified, nearest first.
//
// The bool reports whether the scan actually ran: true means it executed (the
// caller should charge fuel); false means a precondition failed (no scanner,
// no current location) and no fuel should be spent.
func (c *Campaign) Scan(level int) ([]*Location, bool) {
	if level < 1 {
		return nil, false
	}
	tiers := genConfig().ScannerTiers
	if len(tiers) == 0 {
		return nil, false
	}
	tier := tiers[min(level, len(tiers))-1]
	cur := c.Locations[c.CurrentLocationID]
	if cur == nil {
		return nil, false
	}
	found := c.IdentifyWithin(cur.X, cur.Y, tier.Range)
	c.RevealPresencesWithin(cur.X, cur.Y, tier.Range)
	return found, true
}

// ScanRange is the scanner's reach at level (0: none).
func ScanRange(level int) float64 {
	tiers := genConfig().ScannerTiers
	if level < 1 || len(tiers) == 0 {
		return 0
	}
	return tiers[min(level, len(tiers))-1].Range
}

// IdentifyWithin identifies every system and the gate within r of (x, y),
// returning the newly identified nearest first.
func (c *Campaign) IdentifyWithin(x, y, r float64) []*Location {
	var out []*Location
	for _, l := range c.sortedLocations() {
		if l.Kind != GateKind && (!l.HasSite() || l.Kind == EncounterKind) {
			continue
		}
		if math.Hypot(l.X-x, l.Y-y) <= r && c.Identify(l) {
			out = append(out, l)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return math.Hypot(out[i].X-x, out[i].Y-y) < math.Hypot(out[j].X-x, out[j].Y-y)
	})
	return out
}

// tooClose reports whether (x,y) lands within minSep of any existing location,
// so systems don't stack into overlapping star-map icons. minSep <= 0 disables.
func (c *Campaign) tooClose(x, y, minSep float64) bool {
	if minSep <= 0 {
		return false
	}
	for _, loc := range c.Locations {
		if math.Hypot(loc.X-x, loc.Y-y) < minSep {
			return true
		}
	}
	return false
}

// placeSeparated re-rolls gen until it yields coordinates at least
// MinSeparation from every existing location, or the attempt budget is spent —
// then it uses the last roll so generation never stalls. gen must draw only
// from the seeded rng so the outcome stays reproducible (the number of retries
// is itself deterministic given the campaign state).
func (c *Campaign) placeSeparated(gen func() (float64, float64)) (float64, float64) {
	minSep := genConfig().MinSeparation
	var x, y float64
	for i := 0; i < separationAttempts; i++ {
		x, y = gen()
		if !c.tooClose(x, y, minSep) {
			break
		}
	}
	return x, y
}

// uniqueName rolls location names until it finds one not already in use, then
// falls back to a numeric suffix, so two systems never share a name.
func (c *Campaign) uniqueName(rng *rand.Rand) string {
	used := make(map[string]bool, len(c.Locations))
	for _, loc := range c.Locations {
		used[loc.Name] = true
	}
	for i := 0; i < nameAttempts; i++ {
		if n := genName(rng); !used[n] {
			return n
		}
	}
	// Pool exhausted (or an unlucky streak) — disambiguate with a suffix.
	base := genName(rng)
	n := base
	for i := 2; used[n]; i++ {
		n = fmt.Sprintf("%s %d", base, i)
	}
	return n
}

// bindHistory gives a charted site its history: the scenario is the history's
// chapter (or the map's quiet base), the summary its one-line story. wantEvent,
// if set, asks for a history ending in that chapter (a contract's threat). On a
// failure the site keeps its archetype summary and the map's base scenario.
func (c *Campaign) bindHistory(loc *Location, wantEvent string) {
	if !loc.HasSite() || loc.Tutorial {
		return
	}
	lib, err := history.DefaultLibrary()
	if err != nil {
		log.Printf("history: %v", err)
		return
	}
	if c.History == nil {
		c.History = history.New(c.Seed)
	}
	var site *history.SiteHistory
	switch {
	case c.safeStart(loc): // a first jump is safe whatever it was asked for
		site, err = c.History.BindSiteSafe(lib, loc.MapID, loc.Seed)
	case wantEvent != "":
		site, err = c.History.BindSiteEnding(lib, loc.MapID, loc.Seed, wantEvent)
	default:
		site, err = c.History.BindSite(lib, loc.MapID, loc.Seed)
	}
	if err != nil {
		log.Printf("history: %s (%s): %v", loc.ID, loc.MapID, err)
		if md := lib.Maps[loc.MapID]; md != nil {
			loc.ScenarioID = md.BaseScenario
		}
		return
	}
	c.History.Place(site.ID, loc.ID)
	loc.History = site.ID
	// Every event with a place on the map leaves a datapad there.
	for _, f := range LoreDatapadFixtures(c.History.Evidence(lib, site).Datapads) {
		loc.AddFixture(f)
	}
	loc.ScenarioID = site.Scenario
	loc.Tier = site.Tier
	if s := c.History.Summary(lib, site); s != "" {
		loc.Summary = s
	}
}

// LocationTier is loc's danger class for the Star Map: its Tier, else (a save
// from before tiers) derived from its site history; "" without a history.
func (c *Campaign) LocationTier(loc *Location) string {
	if loc == nil {
		return ""
	}
	if loc.Tier != "" {
		return loc.Tier
	}
	site := c.SiteHistory(loc)
	if site == nil {
		return ""
	}
	lib, err := history.DefaultLibrary()
	if err != nil {
		return ""
	}
	return c.History.TierOf(lib, site)
}

// firstJump reports whether loc is within a first jump of the Wreck Site
// (safe_start_radius, default start_radius), where a landing must be safe.
func (c *Campaign) firstJump(loc *Location) bool {
	cfg := genConfig()
	r := cfg.SafeStartRadius
	if r <= 0 {
		r = cfg.StartRadius
	}
	var wx, wy float64
	if w := c.Locations[AdriftLocationID]; w != nil {
		wx, wy = w.X, w.Y
	}
	return math.Hypot(loc.X-wx, loc.Y-wy) <= r+1e-6
}

// LoreDatapadKind is the fixture kind of a datapad that records a history event.
const LoreDatapadKind = "lore_datapad"

// LoreDatapadFixtures are the fixtures for a site's lore datapads, one per
// event that leaves one (history.SiteEvidence.Datapads).
func LoreDatapadFixtures(events []string) []QuestFixture {
	out := make([]QuestFixture, 0, len(events))
	for _, ev := range events {
		out = append(out, QuestFixture{Kind: LoreDatapadKind, Blueprint: "datapad", EventID: ev})
	}
	return out
}

// storyEvent is the event a site's quest datapads tell: its chapter, else its
// latest event; "" for a site with no history.
func (c *Campaign) storyEvent(loc *Location) string {
	site := c.SiteHistory(loc)
	switch {
	case site == nil:
		return ""
	case site.Chapter != "":
		return site.Chapter
	case len(site.Events) > 0:
		return site.Events[len(site.Events)-1]
	}
	return ""
}

// RevealEvent marks a history event known (a datapad was read) and returns its
// chronicle text, or "" if it is unknown. newly reports whether it was news.
func (c *Campaign) RevealEvent(id string) (text string, newly bool) {
	return c.reveal(id, false)
}

// RevealDatapad is RevealEvent for a datapad picked up on the site: a
// per-side event reads as its own people's account (text_side).
func (c *Campaign) RevealDatapad(id string) (text string, newly bool) {
	return c.reveal(id, true)
}

func (c *Campaign) reveal(id string, side bool) (text string, newly bool) {
	text = c.eventText(id, side)
	if text == "" {
		return "", false
	}
	if c.History.Known == nil {
		c.History.Known = map[string]bool{}
	}
	newly = !c.History.Known[id]
	c.History.Known[id] = true
	return text, newly
}

// DatapadText is what a datapad holding event id reads (RevealDatapad's
// text) without making the event known: the observer reads a datapad where
// it lies. "" for an unknown event.
func (c *Campaign) DatapadText(id string) string { return c.eventText(id, true) }

// eventText renders event id (its datapad account when side), "" if there
// is no such event.
func (c *Campaign) eventText(id string, side bool) string {
	if c == nil || c.History == nil || id == "" {
		return ""
	}
	ev := c.History.Event(id)
	if ev == nil {
		return ""
	}
	lib, err := history.DefaultLibrary()
	if err != nil {
		return ""
	}
	site := ""
	if s := c.History.Sites[ev.Site]; s != nil {
		if loc := c.Locations[s.Location]; loc != nil {
			site = loc.Name
		}
	}
	if side {
		return c.History.RenderSide(lib, ev, site)
	}
	return c.History.Render(lib, ev, site)
}

// RecordLive appends a live history event at loc (dated in the star date,
// known at once) and returns its ID; "" when the campaign has no history.
func (c *Campaign) RecordLive(typ string, loc *Location, vars map[string]string, figs ...history.Figure) string {
	if c == nil || c.History == nil {
		return ""
	}
	v := map[string]string{}
	for k, val := range vars {
		v[k] = val
	}
	site := ""
	if loc != nil {
		site = loc.History
		v["place"] = loc.Name
	}
	return c.History.Record(typ, c.StarDate, site, v, figs...)
}

// ColonyDisplayName is how lore text names the expedition's colonies: the
// expedition's own name ("the Lore Legends colony"), never the internal
// per-location settlement key ("Lore Legends Colony @ Drift Tract").
func (c *Campaign) ColonyDisplayName() string {
	if c.Name != "" {
		return c.Name
	}
	return "expedition"
}

// QuestTitle is a quest's name for lore text: without the " — <site>" suffix
// quest lists add, since the chronicle names the site itself.
func (c *Campaign) QuestTitle(q *Quest) string {
	if loc := c.Locations[q.Location]; loc != nil {
		return strings.TrimSuffix(q.Name, " — "+loc.Name)
	}
	return q.Name
}

// recordQuestDone logs a completed quest and reveals the event behind it.
func (c *Campaign) recordQuestDone(q *Quest) {
	c.RecordLive("quest_completed", c.Locations[q.Location], map[string]string{"quest": c.QuestTitle(q)})
	c.RevealEvent(q.EventID)
}

// KnownEventCount is how many history events the player has discovered.
func (c *Campaign) KnownEventCount() int {
	if c == nil || c.History == nil {
		return 0
	}
	return len(c.History.Known)
}

// SiteHistory returns the history bound to a location, or nil.
func (c *Campaign) SiteHistory(loc *Location) *history.SiteHistory {
	if c == nil || c.History == nil || loc == nil || loc.History == "" {
		return nil
	}
	return c.History.Sites[loc.History]
}

// addDatapadQuests gives a location 1..datapad_quests_max hidden quests, each
// recoverable only by finding its datapad clue spawned at the location. They
// stay out of the quest log until the datapad is picked up.
func (c *Campaign) addDatapadQuests(rng *rand.Rand, qt *questTemplates, a locArchetype, loc *Location) {
	max := genConfig().DatapadQuestsMax
	if max <= 0 {
		return
	}
	n := 1 + rng.Intn(max)
	for i := 0; i < n; i++ {
		q := c.makeQuest(rng, qt, a, loc)
		if q == nil {
			continue
		}
		q.Delivery = "datapad"
		c.QuestDefs = append(c.QuestDefs, *q)
		loc.AddFixture(QuestFixture{
			QuestID:   q.ID,
			Blueprint: "datapad",
			Kind:      "datapad",
			EventID:   c.storyEvent(loc),
		})
	}
}

// makeQuest instantiates a concrete Quest from a tag-compatible, non-contract
// template, bound to the given (existing) location.
func (c *Campaign) makeQuest(rng *rand.Rand, qt *questTemplates, a locArchetype, loc *Location) *Quest {
	t := pickQuestTemplate(rng, qt, a.Tags)
	if t == nil {
		return nil
	}
	return c.buildQuestFromTemplate(rng, t, loc)
}

// makeContract rolls a "contract" quest pointing at an existing system made
// from the template's spawn archetype (one whose site ends in the template's
// spawn event preferred), near the ship by preference, not yet visited.
// Accepting the quest reveals the system (see Campaign.AcceptQuest). nil
// when no system fits.
func (c *Campaign) makeContract(rng *rand.Rand, qt *questTemplates) *Quest {
	contracts := contractTemplates(qt)
	if len(contracts) == 0 {
		return nil
	}
	t := contracts[rng.Intn(len(contracts))]
	fits := func(l *Location) bool {
		return l.Archetype == t.SpawnArchetype && !c.safeStart(l) && !c.contracted(l)
	}
	if t.SpawnEvent != "" {
		inner := fits
		fits = func(l *Location) bool {
			site := c.SiteHistory(l)
			return inner(l) && site != nil && c.History.Event(site.Chapter) != nil && c.History.Event(site.Chapter).Type == t.SpawnEvent
		}
		if c.questTarget(rand.New(rand.NewSource(0)), fits) == nil {
			fits = inner // no system plays it: any of the archetype
		}
	}
	loc := c.questTarget(rng, fits)
	if loc == nil {
		return nil
	}
	q := c.buildQuestFromTemplate(rng, t, loc)
	if q != nil {
		q.Contract = true
	}
	return q
}

// contracted reports whether a contract already points at l.
func (c *Campaign) contracted(l *Location) bool {
	for i := range c.QuestDefs {
		if q := &c.QuestDefs[i]; q.Contract && q.Location == l.ID {
			return true
		}
	}
	return false
}

// buildQuestFromTemplate constructs a concrete Quest from a chosen template,
// bound to loc.
func (c *Campaign) buildQuestFromTemplate(rng *rand.Rand, t *questTemplate, loc *Location) *Quest {
	c.GenSeq++
	q := &Quest{
		ID:       fmt.Sprintf("q_%d", c.GenSeq),
		EventID:  c.storyEvent(loc),
		Location: loc.ID,
		Reward:   QuestReward{SpawnSystems: t.SpawnSystems, Resources: t.RewardRes},
	}
	amount := 0
	if t.Max > t.Min {
		amount = t.Min + rng.Intn(t.Max-t.Min+1)
	} else {
		amount = t.Min
	}
	switch t.Trigger {
	case "resource_gathered":
		q.Name = fmt.Sprintf("%s — %s", t.Name, loc.Name)
		q.Description = replaceN(t.Desc, amount)
		q.Objective = objective.Rule{Trigger: objective.TriggerResourceGathered, Resource: t.Resource, Op: "gte", Threshold: amount}
		q.Reward.Fuel = t.RewardFuelFlat + t.RewardFuelPer*amount
	case "entity_killed":
		q.Name = fmt.Sprintf("%s — %s", t.Name, loc.Name)
		q.Description = t.Desc
		q.Objective = objective.Rule{Trigger: objective.TriggerEntityKilled, Blueprints: t.Blueprints, Op: "lte", Threshold: 0}
		q.Reward.Fuel = t.RewardFuelFlat
	case "tech_researched":
		tech := pick(rng, t.TechPool)
		q.Name = fmt.Sprintf("%s — %s", t.Name, loc.Name)
		q.Description = replaceTech(t.Desc, tech)
		q.Objective = objective.Rule{Trigger: objective.TriggerTechResearched, TechKey: tech}
		q.Reward.Fuel = t.RewardFuelFlat
	case "flag_set":
		q.Name = fmt.Sprintf("%s — %s", t.Name, loc.Name)
		q.Description = t.Desc
		q.Objective = objective.Rule{Trigger: objective.TriggerFlagSet, Flag: t.Flag}
		q.Reward.Fuel = t.RewardFuelFlat
	case "structure_built":
		q.Name = fmt.Sprintf("%s — %s", t.Name, loc.Name)
		q.Description = t.Desc
		q.Objective = objective.Rule{Trigger: objective.TriggerStructureBuilt, Structure: t.Structure}
		q.Reward.Fuel = t.RewardFuelFlat
	case "target_killed":
		q.Description = t.Desc
		q.Objective = objective.Rule{Trigger: objective.TriggerTargetKilled, Target: q.ID}
		q.Reward.Fuel = t.RewardFuelFlat
		if t.FixtureStructure != "" {
			// Structure-backed bounty: the generator script decides who the
			// target is and what it's called, then flags it via
			// mark_quest_target. The template carries no target info at all.
			q.TitlePrefix = t.Name
			q.Name = fmt.Sprintf("%s — %s", t.Name, loc.Name)
			loc.AddFixture(QuestFixture{
				QuestID:   q.ID,
				Kind:      "boss",
				Structure: t.FixtureStructure,
				StructW:   t.FixtureStructW,
				StructH:   t.FixtureStructH,
			})
		} else {
			// Legacy bounty: the template names a creature + bounty name and
			// the fixture spawns it in the open.
			blueprint := pick(rng, t.TargetBlueprints)
			if blueprint == "" {
				return nil
			}
			name := pick(rng, t.TargetNames)
			q.Name = fmt.Sprintf("%s: %s — %s", t.Name, name, loc.Name)
			loc.AddFixture(QuestFixture{
				QuestID:   q.ID,
				Blueprint: blueprint,
				Name:      name,
				Kind:      "boss",
			})
		}
	default:
		return nil
	}
	return q
}

// --- helpers ---

func contractTemplates(qt *questTemplates) []*questTemplate {
	var out []*questTemplate
	for i := range qt.Templates {
		if qt.Templates[i].SpawnArchetype != "" {
			out = append(out, &qt.Templates[i])
		}
	}
	return out
}

func archetypeByID(lt *locTemplates, id string) (locArchetype, bool) {
	for _, a := range lt.Archetypes {
		if a.ID == id {
			return a, true
		}
	}
	return locArchetype{}, false
}

func startArchetype(rng *rand.Rand, lt *locTemplates) locArchetype {
	var pool []locArchetype
	for _, a := range lt.Archetypes {
		if a.StartOK {
			pool = append(pool, a)
		}
	}
	if len(pool) == 0 {
		pool = lt.Archetypes
	}
	return pool[rng.Intn(len(pool))]
}

func weightedArchetype(rng *rand.Rand, lt *locTemplates) locArchetype {
	total := 0
	for _, a := range lt.Archetypes {
		w := a.Weight
		if w <= 0 {
			w = 1
		}
		total += w
	}
	r := rng.Intn(total)
	for _, a := range lt.Archetypes {
		w := a.Weight
		if w <= 0 {
			w = 1
		}
		if r < w {
			return a
		}
		r -= w
	}
	return lt.Archetypes[0]
}

// StationQuestTag marks the quest templates every station site gets.
const StationQuestTag = "station"

// addStationQuests gives a station site its goals (station_plan.md, section
// 10): every quest template tagged "station" whose required module the
// site's plan has (power for all; the sealed archive where there is one).
// They are bound here from the site's own data, drawing nothing from the
// campaign's stream, so every other roll stays where it was.
func (c *Campaign) addStationQuests(loc *Location) {
	if loc.Kind != "station" || c.SiteHistory(loc) == nil {
		return
	}
	_, qt, err := loadGenTemplates()
	if err != nil {
		return
	}
	for i := range qt.Templates {
		t := &qt.Templates[i]
		if t.RequiresTag != StationQuestTag || t.Trigger != "flag_set" {
			continue
		}
		if t.RequiresModule != "" && !c.siteHasModule(loc, t.RequiresModule, strings.HasPrefix(t.Flag, "door:")) {
			continue
		}
		c.QuestDefs = append(c.QuestDefs, Quest{
			ID:          "sq_" + t.ID + "_" + loc.ID,
			Name:        t.Name + " — " + loc.Name,
			Description: t.Desc,
			Location:    loc.ID,
			Objective:   objective.Rule{Trigger: objective.TriggerFlagSet, Flag: t.Flag},
			Reward:      QuestReward{Fuel: t.RewardFuelFlat, Resources: t.RewardRes},
		})
	}
}

// siteHasModule reports whether a charted station site's plan has a module
// of kind, a sealed one if sealed (false for a planet or a site without a
// history).
func (c *Campaign) siteHasModule(loc *Location, kind string, sealed bool) bool {
	site := c.SiteHistory(loc)
	if site == nil {
		return false
	}
	lib, err := history.DefaultLibrary()
	if err != nil {
		return false
	}
	plan := c.History.Evidence(lib, site).Station
	if plan == nil {
		return false
	}
	for _, m := range plan.Modules {
		if m.Kind == kind && (!sealed || m.Sealed != "") {
			return true
		}
	}
	return false
}

func pickQuestTemplate(rng *rand.Rand, qt *questTemplates, tags []string) *questTemplate {
	var pool []*questTemplate
	for i := range qt.Templates {
		t := &qt.Templates[i]
		if t.SpawnArchetype != "" || t.History != "" {
			continue // contracts bring their own location, history quests bind at charting; not tag-rolled
		}
		if t.RequiresTag != "" && !hasTag(tags, t.RequiresTag) || t.RequiresTag == StationQuestTag {
			continue // station quests come with the station (addStationQuests)
		}
		w := t.Weight
		if w <= 0 {
			w = 1
		}
		for j := 0; j < w; j++ {
			pool = append(pool, t)
		}
	}
	if len(pool) == 0 {
		return nil
	}
	return pool[rng.Intn(len(pool))]
}

func genName(rng *rand.Rand) string {
	n := lore.RandomNameSeeded(rng, "location")
	if n == "" || n == "Unknown" {
		return fmt.Sprintf("System %d", rng.Intn(900)+100)
	}
	return n
}

func hasTag(tags []string, want string) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}

func firstTag(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return tags[0]
}
