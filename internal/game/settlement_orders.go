package game

import (
	"log"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/workerai"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

// The player's order paths on any settlement (phase C2): what the build,
// dig and mine cursors, a click's haul and the research and crafting modals
// post, as functions of a settlement and its level. The cursor posts onto
// the player's settlement through MainState's queue* wrappers; a faction
// settlement's director posts onto its own through settlementOrders. One
// path, so a rival colony builds exactly as the player does.

// orderBuild posts a build of kind at (x, y, z), the BuildRequest position
// (a sized entity's centre), unless one is already ordered there.
func orderBuild(st *settlement.Settlement, level *world.Level, kind string, x, y, z int) bool {
	if st == nil {
		return false
	}
	if construction.GetBuildable(kind).DeployOnly {
		return false // a relay pylon is deployed, never built
	}
	if !buildFloorOK(level, kind, x, y, z) {
		return false // a hive structure off resin, an autonz one off wire
	}
	for _, t := range st.Tasks.GetTasks() {
		if t.X == x && t.Y == y && t.Z == z && t.Action == task_requests.BuildAction {
			return false
		}
	}
	buildable := construction.GetBuildable(kind)
	st.Tasks.AddTask(&task.Task{
		Action: task_requests.BuildAction,
		Data:   task_requests.BuildRequest{X: x, Y: y, Z: z, Type: kind, Required: buildable.BuildTime},
		X:      x, Y: y, Z: z,
	})
	return true
}

// buildFloorOK reports whether kind's required floor lies under its whole
// footprint at (x, y, z) (a sized entity centred there). No level: unchecked.
func buildFloorOK(level *world.Level, kind string, x, y, z int) bool {
	b := construction.GetBuildable(kind)
	if len(b.RequiredFloor) == 0 || level == nil {
		return true
	}
	return b.FloorOK(buildFootprint(b, kind, x, y), func(tx, ty int) string { return floorNameAt(level, tx, ty, z) })
}

// buildFootprint is the tiles a build of kind at (x, y) covers (a sized
// entity is centred, start = pos - size/2).
func buildFootprint(b construction.Buildable, kind string, x, y int) [][2]int {
	w, h := 1, 1
	if b.IsEntity {
		if sc := factory.GetSize(kind); sc != nil && sc.Width > 0 && sc.Height > 0 {
			w, h = sc.Width, sc.Height
		}
	}
	var out [][2]int
	for dx := 0; dx < w; dx++ {
		for dy := 0; dy < h; dy++ {
			out = append(out, [2]int{x - w/2 + dx, y - h/2 + dy})
		}
	}
	return out
}

// floorNameAt is the floor tile's name at (x, y, z), "" for none.
func floorNameAt(level *world.Level, x, y, z int) string {
	t := level.GetTilePtr(x, y, z)
	if t == nil || t.Floor.IsEmpty() {
		return ""
	}
	return world.TileDefinitions[t.Floor.Type].Name
}

// orderDig posts a Dig on a solid tile, unless one is already ordered.
func orderDig(st *settlement.Settlement, level *world.Level, x, y, z int) bool {
	if st == nil || level == nil {
		return false
	}
	for _, t := range st.Tasks.GetTasks() {
		if t.X == x && t.Y == y && t.Z == z && t.Action == task_requests.DigAction {
			return false
		}
	}
	tile := level.GetTileAt(x, y, z)
	if tile == nil || !tile.IsSolid() {
		return false
	}
	st.Tasks.AddTask(&task.Task{
		Action: task_requests.DigAction,
		Data:   task_requests.DigRequest{X: x, Y: y, Z: z, Required: 5},
		X:      x, Y: y, Z: z,
	})
	return true
}

// orderMine posts a Mine/Chop at (x, y, z) when it holds something worth
// collecting: a Choppable entity (its Required from the entity's health) or
// a deposit tile. It reports whether such an order is (now) open; plain rock
// yields nothing, so it is false there.
func orderMine(st *settlement.Settlement, level *world.Level, x, y, z int) bool {
	if st == nil || level == nil {
		return false
	}
	// Dedup: an area sweep can revisit a tile; don't stack duplicate mine orders.
	for _, t := range st.Tasks.GetTasks() {
		if t.X == x && t.Y == y && t.Z == z && t.Action == task_requests.MineAction {
			return true
		}
	}
	var entBuf []*ecs.Entity
	level.GetEntitiesAt(x, y, z, &entBuf)
	for _, e := range entBuf {
		if !e.HasComponent(components.Choppable) {
			continue
		}
		ch := e.GetComponent(components.Choppable).(*components.ChoppableComponent)
		st.Tasks.AddTask(&task.Task{
			Action: task_requests.MineAction,
			Data:   task_requests.MineRequest{X: x, Y: y, Z: z, Required: ch.Health * 10, Target: e},
			X:      x, Y: y, Z: z,
		})
		return true
	}
	tile := level.GetTileAt(x, y, z)
	if tile == nil {
		return false
	}
	t := tile.(*world.Tile)
	if t.Middle.IsEmpty() || !world.IsDepositTileName(world.TileDefinitions[t.Middle.Type].Name) {
		return false
	}
	st.Tasks.AddTask(&task.Task{
		Action: task_requests.MineAction,
		Data:   task_requests.MineRequest{X: x, Y: y, Z: z, Required: 50},
		X:      x, Y: y, Z: z,
	})
	return true
}

// orderRetrieve posts a haul of a loose item into storage.
func orderRetrieve(st *settlement.Settlement, item *ecs.Entity, x, y, z int) {
	st.Tasks.AddTask(&task.Task{
		Action: task_requests.RetrieveAction,
		Data:   task_requests.RetrieveRequest{Item: item},
		X:      x, Y: y, Z: z, Escalated: true,
	})
}

// orderResearch posts research of techKey at station, when it is in the
// doctrine's tree, not known and not already being researched. why says
// what refused it ("" when it queued).
func orderResearch(st *settlement.Settlement, ds *campaign.DoctrineSettlement, known func(string) bool, techKey string, station *ecs.Entity) (ok bool, why string) {
	if st == nil || station == nil {
		return false, "no settlement"
	}
	tech, found := research.GetTech(techKey)
	if !found || !ds.CanResearch(techKey) {
		log.Printf("[RESEARCH] tech not found in the doctrine's tree: %s", techKey)
		return false, "unknown"
	}
	if known(techKey) {
		return false, tech.Name + " already researched."
	}
	for _, t := range st.Tasks.GetTasks() {
		if t.Completed || t.Action != task_requests.ResearchAction {
			continue
		}
		if rr, ok := t.Data.(*task_requests.ResearchRequest); ok && rr.TechKey == techKey {
			return false, tech.Name + " is already being researched."
		}
	}
	pc := station.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	st.Tasks.AddTask(&task.Task{
		Action: task_requests.ResearchAction,
		Data: &task_requests.ResearchRequest{
			TechKey:  techKey,
			Building: tech.RequiredBuilding,
			Required: tech.Duration,
		},
		X: pc.GetX(), Y: pc.GetY(), Z: pc.GetZ(),
	})
	return true, ""
}

// orderCraft posts a craft of recipeID at station when the doctrine offers
// it and known reports its tech (a campaign player's techs live on the
// campaign record, a faction settlement's on the settlement).
func orderCraft(st *settlement.Settlement, ds *campaign.DoctrineSettlement, known func(string) bool, recipeID string, station *ecs.Entity) bool {
	if st == nil || station == nil {
		return false
	}
	recipe, found := crafting.GetRecipe(recipeID)
	if !found || !ds.CanCraft(recipe.Key) {
		log.Printf("[CRAFT] recipe not found in the doctrine: %s", recipeID)
		return false
	}
	if recipe.RequiresTech != "" && (known == nil || !known(recipe.RequiresTech)) {
		return false // refused until the settlement knows it
	}
	wbPC := station.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if recipe.ConsumeStation {
		// One egg per grey egg: a station the craft uses up takes one order.
		for _, t := range st.Tasks.GetTasks() {
			if cr, ok := t.Data.(task_requests.CraftRequest); ok && !t.Completed &&
				cr.WorkbenchX == wbPC.GetX() && cr.WorkbenchY == wbPC.GetY() && cr.WorkbenchZ == wbPC.GetZ() {
				return false
			}
		}
	}
	st.Tasks.AddTask(&task.Task{
		Action: task_requests.CraftAction,
		Data: task_requests.CraftRequest{
			RecipeID:   recipeID,
			WorkbenchX: wbPC.GetX(),
			WorkbenchY: wbPC.GetY(),
			WorkbenchZ: wbPC.GetZ(),
			Required:   printTime(st, recipe),
		},
		X: wbPC.GetX(), Y: wbPC.GetY(), Z: wbPC.GetZ(),
	})
	return true
}

// printTime is a craft's work: a print's (a recipe making a typed unit)
// shortened by every develop print_time factor the settlement knows (the
// greys' print refinement), at least 1.
func printTime(st *settlement.Settlement, recipe crafting.Recipe) int {
	t := float64(recipe.BuildTime)
	if recipe.KinType == "" || st == nil {
		return recipe.BuildTime
	}
	for _, k := range st.KnownTechs {
		if r, ok := research.GetTech(k); ok && r.Develop != nil && r.Develop.PrintTime > 0 {
			t *= r.Develop.PrintTime
		}
	}
	return max(1, int(t+0.5))
}

// cancelOrder is the Cancel Task order on one of a settlement's tasks.
func cancelOrder(st *settlement.Settlement, t *task.Task) {
	t.Complete()
	if st != nil {
		st.Tasks.RemoveTask(t)
	}
}

// settlementOrders are those order paths on one settlement and level, for
// a faction settlement's director (director.Orders).
type settlementOrders struct {
	st    *settlement.Settlement
	level *world.Level
	ds    *campaign.DoctrineSettlement
}

func (o settlementOrders) Build(kind string, x, y, z int) bool {
	if construction.GetBuildable(kind).ModuleOnly {
		return false // a ship's module: carried, never built by a director
	}
	return orderBuild(o.st, o.level, kind, x, y, z)
}
func (o settlementOrders) Mine(x, y, z int) bool { return orderMine(o.st, o.level, x, y, z) }
func (o settlementOrders) Dig(x, y, z int) bool  { return orderDig(o.st, o.level, x, y, z) }
func (o settlementOrders) Cancel(t *task.Task)   { cancelOrder(o.st, t) }

func (o settlementOrders) Retrieve(item *ecs.Entity) bool {
	if o.st == nil || !item.HasComponent(rlcomponents.Position) {
		return false
	}
	pc := item.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	orderRetrieve(o.st, item, pc.GetX(), pc.GetY(), pc.GetZ())
	return true
}

func (o settlementOrders) Research(tech string, station *ecs.Entity) bool {
	ok, _ := orderResearch(o.st, o.ds, o.st.HasTech, tech, station)
	return ok
}

func (o settlementOrders) Craft(recipe string, station *ecs.Entity) bool {
	return orderCraft(o.st, o.ds, o.st.HasTech, recipe, station)
}

// ConvertTech is the research that gives the player ConvertAction (the
// plan's open question: the design wants it, gated behind research). No
// research entry or cursor offers it yet: the order path exists for the
// directors, and the player's waits for content.
const ConvertTech = "terraforming"

// orderConvert posts a ConvertAction turning (x, y, z) into tile, unless one
// is already ordered there or the tile cannot be converted.
func orderConvert(st *settlement.Settlement, level *world.Level, x, y, z int, tile string) bool {
	if st == nil || level == nil || tile == "" || !workerai.Convertible(level, x, y, z, tile) {
		return false
	}
	for _, t := range st.Tasks.GetTasks() {
		if t.X == x && t.Y == y && t.Z == z && t.Action == task_requests.ConvertAction {
			return false
		}
	}
	st.Tasks.AddTask(&task.Task{
		Action: task_requests.ConvertAction,
		Data:   task_requests.ConvertRequest{X: x, Y: y, Z: z, Tile: tile, Required: convertTicks, Rate: convertRate(st)},
		X:      x, Y: y, Z: z,
	})
	return true
}

// convertTicks is a ConvertAction's work.
const convertTicks = 8

// convertRate is st's live conversion rate: its doctrine's ConversionRate
// over the settlement's known techs, the multiplier the abstract step's
// convertWork reads (1 for a doctrine without rate techs, or the colony).
func convertRate(st *settlement.Settlement) float64 {
	d := campaign.LoadFactionData().Doctrine(doctrineOfSettlement(st.Name))
	if d == nil {
		return 1
	}
	return d.ConversionRate(st.KnownTechs)
}

func (o settlementOrders) Convert(x, y, z int, tile string) bool {
	return orderConvert(o.st, o.level, x, y, z, tile)
}

// orderAttack posts the attack order on target (a raid, a hunt), as the
// player's click on a target does.
func orderAttack(st *settlement.Settlement, target *ecs.Entity) bool {
	if st == nil || target == nil || !target.HasComponent(rlcomponents.Position) || target.HasComponent(rlcomponents.Dead) {
		return false
	}
	pc := target.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	st.Tasks.AddTask(&task.Task{
		Action: task_requests.AttackAction, Data: target,
		X: pc.GetX(), Y: pc.GetY(), Z: pc.GetZ(), Escalated: true,
	})
	return true
}

// orderEquip has an idle, empty-handed worker of st take blueprint out of
// storage and hold it, as the player's equip button does for one colonist.
func orderEquip(st *settlement.Settlement, level *world.Level, blueprint string) bool {
	if st == nil || level == nil {
		return false
	}
	for _, e := range settlementWorkers(level, st.Name) {
		wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
		if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
			continue
		}
		if e.HasComponent(rlcomponents.Inventory) {
			inv := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
			if inv.LeftHand != nil || inv.RightHand != nil {
				continue
			}
		}
		wc.CurrentTask = &task.Task{
			Action: task_requests.EquipAction, Escalated: true,
			Data: task_requests.EquipRequest{ItemBlueprint: blueprint},
		}
		return true
	}
	return false
}

// orderEquipWorker sets worker to take blueprint out of storage and wear or
// hold it now (the arm rung), whatever it was doing back in the queue.
func orderEquipWorker(worker *ecs.Entity, blueprint string) *task.Task {
	if worker == nil || !worker.HasComponent(components.Worker) || blueprint == "" {
		return nil
	}
	wc := worker.GetComponent(components.Worker).(*components.WorkerComponent)
	if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
		if wc.CurrentTask.Data == task_requests.HoldHere {
			wc.CurrentTask.Complete() // a post: the guard rung posts it again
		} else {
			wc.CurrentTask.ReQueue()
		}
	}
	wc.CurrentTask = &task.Task{
		Action: task_requests.EquipAction, Escalated: true,
		Data: task_requests.EquipRequest{ItemBlueprint: blueprint},
	}
	return wc.CurrentTask
}

func (o settlementOrders) EquipWorker(worker *ecs.Entity, blueprint string) *task.Task {
	return orderEquipWorker(worker, blueprint)
}

func (o settlementOrders) Attack(target *ecs.Entity) bool { return orderAttack(o.st, target) }
func (o settlementOrders) Equip(blueprint string, _ *ecs.Entity) bool {
	return orderEquip(o.st, o.level, blueprint)
}

// orderSend sets worker on target at once (a raid party member): whatever
// it was doing goes back to the queue, as a death or Cancel would leave it.
func orderSend(worker, target *ecs.Entity) *task.Task {
	if worker == nil || target == nil || !worker.HasComponent(components.Worker) ||
		!target.HasComponent(rlcomponents.Position) || target.HasComponent(rlcomponents.Dead) {
		return nil
	}
	wc := worker.GetComponent(components.Worker).(*components.WorkerComponent)
	if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
		wc.CurrentTask.ReQueue() // no wall-clock restart gate (mlge Stop)
	}
	pc := target.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	t := &task.Task{Action: task_requests.AttackAction, Data: target,
		X: pc.GetX(), Y: pc.GetY(), Z: pc.GetZ(), Escalated: true}
	t.Start()
	wc.CurrentTask = t
	wc.LastTaskAction = t.Action
	return t
}

func (o settlementOrders) Send(worker, target *ecs.Entity) *task.Task {
	return orderSend(worker, target)
}

// orderMove walks worker to (x, y, z) now, dropping what it was doing.
func orderMove(worker *ecs.Entity, x, y, z int) *task.Task {
	if worker == nil || !worker.HasComponent(components.Worker) {
		return nil
	}
	wc := worker.GetComponent(components.Worker).(*components.WorkerComponent)
	if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
		wc.CurrentTask.ReQueue()
	}
	t := &task.Task{Action: task_requests.MoveAction, X: x, Y: y, Z: z, Escalated: true}
	t.Start()
	wc.CurrentTask = t
	wc.LastTaskAction = t.Action
	return t
}

func (o settlementOrders) Move(worker *ecs.Entity, x, y, z int) *task.Task {
	return orderMove(worker, x, y, z)
}

// deployWork is a deploy's work steps for its doctrine's ticks (one step a
// worker turn, as research durations are: a tenth).
func deployWork(ticks int) int { return max(1, ticks/10) }

// orderDeploy sets worker to deploy into the as structure on (x, y, z) now,
// dropping what it was doing: refused off the floor it needs.
func orderDeploy(level *world.Level, worker *ecs.Entity, x, y, z int, as string, floor []string, ticks int) *task.Task {
	if worker == nil || !worker.HasComponent(components.Worker) || as == "" {
		return nil
	}
	if level != nil && !workerai.DeployFloorOK(level, x, y, z, floor) {
		return nil
	}
	wc := worker.GetComponent(components.Worker).(*components.WorkerComponent)
	if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
		wc.CurrentTask.ReQueue()
	}
	t := &task.Task{Action: task_requests.DeployAction, X: x, Y: y, Z: z, Escalated: true,
		Data: task_requests.DeployRequest{X: x, Y: y, Z: z, As: as, Floor: floor, Required: deployWork(ticks)}}
	t.Start()
	wc.CurrentTask = t
	wc.LastTaskAction = t.Action
	return t
}

func (o settlementOrders) Deploy(worker *ecs.Entity, x, y, z int, as string, floor []string, ticks int) *task.Task {
	return orderDeploy(o.level, worker, x, y, z, as, floor, ticks)
}

func (o settlementOrders) Hold(worker *ecs.Entity, x, y, z int) *task.Task {
	t := orderMove(worker, x, y, z)
	if t != nil {
		t.Data = task_requests.HoldHere
	}
	return t
}
