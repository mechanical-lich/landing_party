package lore

import (
	"fmt"
	"math/rand"
)

var settlementPrefixes = []string{"New", "Fort", "Colony", "Station", "Camp", "Base", "Outpost"}
var settlementSuffixes = []string{"Alpha", "Prime", "One", "Hope", "Dawn", "Point", "Haven"}

func RandomSettlementName() string {
	prefix := settlementPrefixes[rand.Intn(len(settlementPrefixes))]
	suffix := settlementSuffixes[rand.Intn(len(settlementSuffixes))]
	return fmt.Sprintf("%s %s", prefix, suffix)
}

var colonistNames = []string{
	"Vera", "Dax", "Kira", "Oren", "Sable", "Ryn", "Mace", "Tova",
	"Lev", "Nira", "Cael", "Zara", "Brix", "Asha", "Thorn", "Elsi",
}

func RandomColonistName() string {
	return colonistNames[rand.Intn(len(colonistNames))]
}

func RandomName(nameType string) string {
	switch nameType {
	case "settlement":
		return RandomSettlementName()
	case "colonist":
		return RandomColonistName()
	default:
		return "Unknown"
	}
}
