package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// MainState must satisfy the narrow host interface the tools depend on; this
// compile-time assertion catches drift if an adapter method is removed.
var _ cursorToolHost = (*MainState)(nil)

// fakeToolHost is a recording stand-in for MainState, so a cursor tool's logic
// can be exercised without the full game state.
type fakeToolHost struct {
	level      *world.Level
	settlement *settlement.Settlement
	walkable   bool

	contexts    [][3]string          // (title, body, blueprint) passed to SetContext
	modes       []gui.CursorModeType // modes requested via RequestMode
	openedModal *ecs.Entity
}

func (f *fakeToolHost) Level() *world.Level                { return f.level }
func (f *fakeToolHost) Settlement() *settlement.Settlement { return f.settlement }
func (f *fakeToolHost) SetContext(t, b, bp string) {
	f.contexts = append(f.contexts, [3]string{t, b, bp})
}
func (f *fakeToolHost) OpenColonistModal(c *ecs.Entity)  { f.openedModal = c }
func (f *fakeToolHost) TileWalkable(x, y, z int) bool    { return f.walkable }
func (f *fakeToolHost) RequestMode(m gui.CursorModeType) { f.modes = append(f.modes, m) }

func (f *fakeToolHost) lastContext() [3]string {
	if len(f.contexts) == 0 {
		return [3]string{}
	}
	return f.contexts[len(f.contexts)-1]
}

// colonist builds a minimal worker entity carrying the components the Drop/
// Pickup tools require.
func colonistEntity() *ecs.Entity {
	e := &ecs.Entity{}
	e.AddComponent(&components.WorkerComponent{})
	e.AddComponent(&rlcomponents.InventoryComponent{})
	e.AddComponent(&rlcomponents.AIMemoryComponent{})
	return e
}

func TestRelocateBeginSetsPendingAndRequestsMode(t *testing.T) {
	h := &fakeToolHost{}
	tool := &relocateTool{host: h}
	src := &ecs.Entity{}

	tool.Begin(src, "iron_ore", 5)

	if tool.pending == nil {
		t.Fatal("Begin did not set pending relocate")
	}
	if tool.pending.qty != 5 || tool.pending.blueprint != "iron_ore" || tool.pending.source != src {
		t.Fatalf("pending = %+v, want {src, iron_ore, 5}", tool.pending)
	}
	if len(h.modes) != 1 || h.modes[0] != gui.CursorModeRelocate {
		t.Fatalf("modes = %v, want [Relocate]", h.modes)
	}
}

func TestRelocateBeginRejectsInvalid(t *testing.T) {
	h := &fakeToolHost{}
	tool := &relocateTool{host: h}

	tool.Begin(nil, "iron_ore", 5)           // nil source
	tool.Begin(&ecs.Entity{}, "iron_ore", 0) // zero qty

	if tool.pending != nil {
		t.Fatalf("pending should stay nil for invalid Begin, got %+v", tool.pending)
	}
	if len(h.modes) != 0 {
		t.Fatalf("invalid Begin should not request a mode, got %v", h.modes)
	}
}

func TestPickupRequestGuards(t *testing.T) {
	cases := []struct {
		name    string
		colon   *ecs.Entity
		wantSet bool
	}{
		{"nil", nil, false},
		{"missing components", &ecs.Entity{}, false},
		{"valid colonist", colonistEntity(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &fakeToolHost{}
			tool := &pickupTool{host: h}
			tool.Request(tc.colon)

			if tc.wantSet {
				if tool.colonist != tc.colon {
					t.Fatal("valid colonist was not captured")
				}
				if len(h.modes) != 1 || h.modes[0] != gui.CursorModePickup {
					t.Fatalf("modes = %v, want [Pickup]", h.modes)
				}
			} else {
				if tool.colonist != nil {
					t.Fatal("invalid Request should not capture a colonist")
				}
				if len(h.modes) != 0 {
					t.Fatalf("invalid Request should not request a mode, got %v", h.modes)
				}
			}
		})
	}
}

func TestDropRequestGuards(t *testing.T) {
	h := &fakeToolHost{}
	tool := &dropTool{host: h}

	tool.Request(colonistEntity(), nil) // nil item
	if tool.item != nil || len(h.modes) != 0 {
		t.Fatal("Request with nil item should be a no-op")
	}

	colon, item := colonistEntity(), &ecs.Entity{}
	tool.Request(colon, item)
	if tool.colonist != colon || tool.item != item {
		t.Fatal("valid Request did not capture (colonist, item)")
	}
	if len(h.modes) != 1 || h.modes[0] != gui.CursorModeDrop {
		t.Fatalf("modes = %v, want [Drop]", h.modes)
	}
}

func TestStoreToolLifecycleAndHighlight(t *testing.T) {
	h := &fakeToolHost{}
	tool := &storeTool{host: h}

	// Enter with no item picked → "pick an item" banner.
	tool.Enter()
	if got := h.lastContext(); got[0] != "Store: Pick an Item" {
		t.Fatalf("Enter banner = %q, want 'Store: Pick an Item'", got[0])
	}

	// setItem highlights the picked item via the Selected component...
	item := &ecs.Entity{Blueprint: "iron_ore"}
	tool.setItem(item)
	if !item.HasComponent(components.Selected) {
		t.Fatal("setItem should add Selected to the picked item")
	}
	// ...and clears it when the selection is dropped.
	tool.setItem(nil)
	if item.HasComponent(components.Selected) {
		t.Fatal("setItem(nil) should remove Selected from the previous item")
	}

	// Exit clears the context tooltip.
	tool.setItem(&ecs.Entity{Blueprint: "iron_ore"})
	tool.Exit()
	if got := h.lastContext(); got != [3]string{"", "", ""} {
		t.Fatalf("Exit should clear context, got %v", got)
	}
	if tool.item != nil {
		t.Fatal("Exit should drop the pending item")
	}
}
