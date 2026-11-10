package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
)

// The bio-printer is in the Structures build menu, and printing a colonist
// through its crafting modal queues print_colonist (the recipe's key, not
// its output "colonist", which no recipe is called) and deploys the colonist
// at the printer (deploy_on_craft).
func TestBioPrinterPrintsAColonist(t *testing.T) {
	useContentTables(t)
	useRealCosts(t)
	if b := construction.GetBuildable("bio_printer"); b.Category != "structures" || b.Hidden {
		t.Fatalf("bio_printer is not a Structures menu entry: %+v", b)
	}
	found := false
	for _, c := range buildMenuOf(colonySettlement(t)) {
		for _, k := range c.Types {
			found = found || (c.ID == "structures" && k == "bio_printer")
		}
	}
	if !found {
		t.Fatal("bio_printer is not in the colony's build menu")
	}

	c := newE2EColony(t)
	c.addChest(t, 3, 2, map[string]int{"biomass": 25})
	printer, err := factory.Create("bio_printer", 8, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	c.ms.level.AddEntity(printer)

	// The station click opens the modal; its one row queues what selecting
	// it sends.
	if err := c.ms.HandleEvent(gui.StationClickedEvent{Station: printer}); err != nil {
		t.Fatal(err)
	}
	ids := c.ms.guiManager.CraftingRecipeIDs()
	if len(ids) != 1 || ids[0] != "print_colonist" {
		t.Fatalf("bio-printer rows %v, want [print_colonist]", ids)
	}
	if err := c.ms.HandleEvent(gui.CraftRequestedEvent{RecipeID: ids[0], Station: printer}); err != nil {
		t.Fatal(err)
	}
	var queued *task_requests.CraftRequest
	for _, tk := range c.ms.MainSettlement.Tasks.GetTasks() {
		if cr, ok := tk.Data.(task_requests.CraftRequest); ok && tk.Action == task_requests.CraftAction {
			queued = &cr
		}
	}
	if queued == nil || queued.RecipeID != "print_colonist" {
		t.Fatalf("queued %+v, want a print_colonist craft", queued)
	}

	before := len(colonyWorkers(c.ms))
	c.tickUntil(t, "colonist printed", func() bool { return len(colonyWorkers(c.ms)) > before })
	var printed int
	for _, e := range colonyWorkers(c.ms) {
		if e != c.colonist && e.Blueprint == "colonist" && components.IsColonyWorker(e, c.name) {
			printed++
		}
	}
	if printed != 1 || c.bagUnits("colonist") != 0 || c.stock()["biomass"] != 0 {
		t.Errorf("printed %d, in bag %d, biomass left %d", printed, c.bagUnits("colonist"), c.stock()["biomass"])
	}
}
