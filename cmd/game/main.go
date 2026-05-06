package main

import (
	"log"

	"github.com/mechanical-lich/scifi_settlements/internal/config"
	"github.com/mechanical-lich/scifi_settlements/internal/factory"
	"github.com/mechanical-lich/scifi_settlements/internal/game"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

func main() {
	if err := world.LoadTileDefinitions("data/tile_definitions.json"); err != nil {
		log.Fatalf("Failed to load tile definitions: %v", err)
	}
	factory.FactoryLoad(config.Global().BlueprintPath)

	g, err := game.NewGame(config.Global().Title)
	if err != nil {
		log.Fatal(err)
	}

	if err := g.Run(); err != nil {
		log.Fatal(err)
	}
}
