package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/mechanical-lich/scifi_settlements/internal/config"
	"github.com/mechanical-lich/scifi_settlements/internal/factory"
	"github.com/mechanical-lich/scifi_settlements/internal/game"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

// If a data/ directory sits next to the executable (release archive layout),
// chdir there so relative "data/..." opens work regardless of launch cwd.
// In dev (go run), the exe lives in a temp dir with no data/ — we leave cwd alone.
func chdirToExecutableIfBundled() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	if _, err := os.Stat(filepath.Join(dir, "data")); err != nil {
		return
	}
	_ = os.Chdir(dir)
}

func main() {
	chdirToExecutableIfBundled()

	if err := world.LoadTileDefinitions("data/tile_definitions.json"); err != nil {
		log.Fatalf("Failed to load tile definitions: %v", err)
	}
	if err := factory.FactoryLoadDir(config.Global().BlueprintPath); err != nil {
		log.Fatalf("Failed to load blueprints: %v", err)
	}

	g, err := game.NewGame(config.Global().Title)
	if err != nil {
		log.Fatal(err)
	}

	if err := g.Run(); err != nil {
		log.Fatal(err)
	}
}
