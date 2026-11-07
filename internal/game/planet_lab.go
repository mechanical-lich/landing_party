package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/effect"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/view"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/event"
)

// planetLabColony is the settlement dummy colonists and setup scripts belong to.
const planetLabColony = "Lab Colony"

// labSettleTicks are run after generation so the first frame is lit and the first
// spawn wave, whose rules read tile light, has happened.
const labSettleTicks = 2

// LoadPlanetLabData loads the generation data the title screen loads in the game.
func LoadPlanetLabData() error {
	generation.SetStructureRunner(func(level *world.Level, name string, x, y, w, h int, params map[string]any, seed int64) error {
		return RunStructureScriptWithParams(level, name, x, y, w, h, params, seed)
	})
	if err := scenario.Load("data/scenarios"); err != nil {
		return err
	}
	if err := mapdef.Load("data/maps"); err != nil {
		return err
	}
	return generation.LoadBiomes("data/biomes")
}

// PlanetLabSpec is everything the Planet Generator tool can vary about a location.
type PlanetLabSpec struct {
	Map                  mapdef.MapDef
	Scenario             *scenario.Scenario // nil for none
	Width, Height, Depth int
	Seed                 int64
	// LightingMode overrides the scenario's lighting; "" keeps it.
	LightingMode    string
	LightingAmbient int
	SuppressSpawns  bool
	// History and Site, when set, build the level as the game builds a
	// charted site: the history's evidence anchored at resolved origins, lore
	// flags for the setup scripts, and lore datapads at the origins.
	History *history.History
	Site    *history.SiteHistory
}

// PlanetLab is a generated location running the game's real simulation, without
// the HUD, campaign or fog of war. See cmd/planet-generator.
type PlanetLab struct {
	s *MainState
}

// NewPlanetLab generates a location. There is one lab at a time: this clears the
// global settlement table, so Close any previous lab first.
func NewPlanetLab(spec PlanetLabSpec) (*PlanetLab, error) {
	if spec.Width <= 0 || spec.Height <= 0 || spec.Depth <= 0 {
		return nil, fmt.Errorf("invalid size %dx%dx%d", spec.Width, spec.Height, spec.Depth)
	}
	for name := range settlement.Settlements {
		delete(settlement.Settlements, name)
	}
	cfg := SettlementConfig{
		Name:            planetLabColony,
		ColonyName:      planetLabColony,
		MapID:           spec.Map.ID,
		Seed:            spec.Seed,
		LightingMode:    spec.LightingMode,
		LightingAmbient: spec.LightingAmbient,
		CampaignMode:    true,
	}
	lore, hasLore := siteLoreFor(spec.History, spec.Site)
	if hasLore {
		lore.apply(&cfg)
	}
	s, err := newMainStateBase(cfg)
	if err != nil {
		return nil, err
	}
	s.teardown() // the tool reads input itself
	if spec.Scenario != nil {
		s.settlementCfg.ScenarioID = spec.Scenario.ID
		_ = scenario.SelectByID(spec.Scenario.ID)
	}
	md := spec.Map
	s.generate(&md, spec.Scenario, spec.Width, spec.Height, spec.Depth, spec.Seed)
	s.applyScenarioLighting(s.level) // generate only does this when there's a scenario
	if hasLore {
		// The game spawns these as location fixtures on arrival; same code here.
		loc := &campaign.Location{Name: "Planet Lab", Seed: spec.Seed, Fixtures: campaign.LoreDatapadFixtures(lore.evidence.Datapads)}
		spawnLocationFixtures(nil, loc, s.level)
	}
	s.gm.suppressSpawns = spec.SuppressSpawns
	s.level.RevealAll = true

	p := &PlanetLab{s: s}
	for i := 0; i < labSettleTicks; i++ {
		p.Step()
	}
	return p, nil
}

// Close detaches the lab from global state so it can be collected.
func (p *PlanetLab) Close() {
	p.s.teardown()
	p.s.guiManager.Close()
}

// Step advances the simulation one tick, as MainState.Update does in the game.
func (p *PlanetLab) Step() {
	s := p.s
	fspath.ResetFrameCounter()
	s.fovSystem.Viewport = s.viewport
	s.stepWorld()
	if s.tick%30 == 0 {
		s.purgeCompletedTasks()
	}
	s.advanceClock()
}

// Draw renders the world view. It matches the world half of MainState.Draw.
func (p *PlanetLab) Draw(dst *ebiten.Image) {
	s := p.s
	if s.worldImage == nil {
		s.worldImage = ebiten.NewImage(s.camera.CanvasW, s.camera.CanvasH)
	}
	s.worldImage.Fill(color.Black)
	s.viewport = s.camera.Viewport()
	world.DrawLevel(s.level, s.worldImage, s.camera)
	world.DrawRadiationOverlay(s.level, s.worldImage, s.camera)
	s.drawTasks(s.worldImage)
	effect.GetEffectManager().Draw(s.worldImage, s.camera)
	dst.DrawImage(s.worldImage, nil)
}

// Level returns the generated level.
func (p *PlanetLab) Level() *world.Level { return p.s.level }

// Camera returns the world camera for the tool to drive.
func (p *PlanetLab) Camera() *view.Camera { return &p.s.camera }

// Tick returns how many simulation ticks have run.
func (p *PlanetLab) Tick() int { return p.s.tick }

// Speed returns the initiative speed; SetSpeed changes it (the game's 1/2/3 keys).
func (p *PlanetLab) Speed() int { return p.s.initiativeSystem.Speed }

// SetSpeed sets the initiative speed.
func (p *PlanetLab) SetSpeed(n int) { p.s.initiativeSystem.Speed = n }

// Colonists returns the living dummy colonists.
func (p *PlanetLab) Colonists() []*ecs.Entity {
	var out []*ecs.Entity
	for _, e := range p.s.level.Entities {
		if components.IsColonyWorker(e, planetLabColony) {
			out = append(out, e)
		}
	}
	return out
}

// PlaceColonist puts a dummy colonist on a walkable, unoccupied tile. It works for
// the settlement like any colonist, creating the settlement if the map had no
// plaza to found one on.
func (p *PlanetLab) PlaceColonist(x, y, z int) (*ecs.Entity, error) {
	s := p.s
	if !s.tileWalkable(x, y, z) || !isStandable(s.level, x, y, z) {
		return nil, fmt.Errorf("can't stand at %d,%d,%d", x, y, z)
	}
	if s.MainSettlement == nil {
		st := settlement.NewSettlement(x, y, z)
		delete(settlement.Settlements, st.Name)
		st.Name = planetLabColony
		settlement.Settlements[planetLabColony] = st
		s.MainSettlement = st
	}
	return spawnColonist(s.level, planetLabColony, x, y, z)
}

// RemoveEntity takes an entity out of the level, stopping any task it holds.
func (p *PlanetLab) RemoveEntity(e *ecs.Entity) {
	if e.HasComponent(components.Worker) {
		wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
		if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
			wc.CurrentTask.Stop()
		}
		wc.CurrentTask = nil
	}
	p.s.level.RemoveEntity(e)
}

// DrainEvents runs the global event queue, which feeds the message log.
func DrainEvents() {
	event.GetQueuedInstance().HandleQueue()
}
