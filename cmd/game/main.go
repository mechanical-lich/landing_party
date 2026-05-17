package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"

	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/game"
	"github.com/mechanical-lich/landing_party/internal/world"
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

	cfg := config.Global()

	if cfg.ProfileCPU {
		f, err := os.Create("cpu.pprof")
		if err != nil {
			log.Fatal("could not create CPU profile:", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile:", err)
		}
		defer pprof.StopCPUProfile()
	}

	if err := world.LoadTileDefinitions("data/tile_definitions.json"); err != nil {
		log.Fatalf("Failed to load tile definitions: %v", err)
	}
	if err := factory.FactoryLoadDir(cfg.BlueprintPath); err != nil {
		log.Fatalf("Failed to load blueprints: %v", err)
	}

	g, err := game.NewGame(cfg.Title)
	if err != nil {
		log.Fatal(err)
	}

	if err := g.Run(); err != nil {
		log.Fatal(err)
	}

	if cfg.ProfileMemory {
		f, err := os.Create("mem.pprof")
		if err != nil {
			log.Fatal("could not create memory profile:", err)
		}
		defer f.Close()
		pprof.WriteHeapProfile(f)
	}
}
