package game

import (
	"encoding/json"
	"log"
	"math/rand"
	"os"
	"sync"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/world"
)

// LootPath holds the loot tables structure scripts fill containers from.
var LootPath = "data/history/loot.json"

// LootEntry is one roll of a loot table: a blueprint, how many, and the
// percent chance it is there at all.
type LootEntry struct {
	Blueprint string `json:"blueprint"`
	Min       int    `json:"min"`
	Max       int    `json:"max"`
	Chance    int    `json:"chance"`
}

var (
	lootOnce   sync.Once
	lootTables map[string][]LootEntry
)

// LootTables are the tables in LootPath, read once.
func LootTables() map[string][]LootEntry {
	lootOnce.Do(func() {
		var f struct {
			Tables map[string][]LootEntry `json:"tables"`
		}
		b, err := os.ReadFile(LootPath)
		if err == nil {
			err = json.Unmarshal(b, &f)
		}
		if err != nil {
			log.Printf("loot: %v", err)
		}
		lootTables = f.Tables
	})
	return lootTables
}

// fillContainer rolls a loot table into the storage container standing at
// (x, y, z); it returns how many stacks went in (0 without a container or
// table).
func fillContainer(level *world.Level, x, y, z int, table string, rng *rand.Rand) int {
	var st *components.StorageComponent
	for _, e := range level.Entities {
		if e == nil || !e.HasComponent(components.Storage) {
			continue
		}
		if pc, ok := entityPos(e); ok && pc.GetX() == x && pc.GetY() == y && pc.GetZ() == z {
			st = e.GetComponent(components.Storage).(*components.StorageComponent)
			break
		}
	}
	entries, ok := LootTables()[table]
	if st == nil || !ok {
		if !ok {
			log.Printf("fill_container: unknown loot table %q", table)
		}
		return 0
	}
	n := 0
	for _, en := range entries {
		if en.Chance > 0 && rng.Intn(100) >= en.Chance {
			continue
		}
		qty := en.Min
		if en.Max > en.Min {
			qty += rng.Intn(en.Max - en.Min + 1)
		}
		if qty < 1 {
			qty = 1
		}
		item, err := factory.Create(en.Blueprint, x, y, z)
		if err != nil {
			continue
		}
		if item.HasComponent(components.Material) {
			item.GetComponent(components.Material).(*components.MaterialComponent).Quantity = qty
		}
		if st.AddItem(item) {
			n++
		}
	}
	return n
}
