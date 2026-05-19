package lore

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
)

var NamesPath = "data/names.json"

type nameEntry struct {
	Kind     string   `json:"kind"`
	Names    []string `json:"names"`
	Prefixes []string `json:"prefixes"`
	Suffixes []string `json:"suffixes"`
}

type epithets struct {
	Title  []string `json:"title"`
	Suffix []string `json:"suffix"`
}

type namesData struct {
	Epithets epithets
	Types    map[string]nameEntry
}

func (n *namesData) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if ep, ok := raw["epithets"]; ok {
		if err := json.Unmarshal(ep, &n.Epithets); err != nil {
			return err
		}
	}
	n.Types = make(map[string]nameEntry)
	for k, v := range raw {
		if k == "epithets" {
			continue
		}
		var entry nameEntry
		if err := json.Unmarshal(v, &entry); err != nil {
			return err
		}
		n.Types[k] = entry
	}
	return nil
}

var (
	namesOnce sync.Once
	names     namesData
)

func loadNames() {
	namesOnce.Do(func() {
		data, err := os.ReadFile(NamesPath)
		if err != nil {
			panic(fmt.Sprintf("lore: failed to read %s: %v", NamesPath, err))
		}
		if err := json.Unmarshal(data, &names); err != nil {
			panic(fmt.Sprintf("lore: failed to parse %s: %v", NamesPath, err))
		}
	})
}

// RandomName rolls a name of nameType using the global RNG.
func RandomName(nameType string) string {
	return randomName(nameType, rand.Intn)
}

// RandomNameSeeded rolls a name of nameType using the supplied RNG, so callers
// that need reproducible-from-seed output (e.g. campaign generation) stay
// deterministic instead of touching global rand.
func RandomNameSeeded(rng *rand.Rand, nameType string) string {
	return randomName(nameType, rng.Intn)
}

// intn is the [0,n) picker shared by the seeded and global entrypoints.
func randomName(nameType string, intn func(int) int) string {
	loadNames()
	entry, ok := names.Types[nameType]
	if !ok {
		return "Unknown"
	}
	switch entry.Kind {
	case "flat":
		if len(entry.Names) == 0 {
			return "Unknown"
		}
		return entry.Names[intn(len(entry.Names))]
	case "combined":
		if len(entry.Prefixes) == 0 || len(entry.Suffixes) == 0 {
			return "Unknown"
		}
		return fmt.Sprintf("%s %s",
			entry.Prefixes[intn(len(entry.Prefixes))],
			entry.Suffixes[intn(len(entry.Suffixes))],
		)
	}
	return "Unknown"
}

// HasNameType reports whether nameType is defined in names.json.
func HasNameType(nameType string) bool {
	loadNames()
	_, ok := names.Types[nameType]
	return ok
}

// placeholderType extracts the name type from a "<type>" token. It lowercases
// the inner token and strips a trailing "name" so legacy placeholders like
// "<colonist>" resolve to the "colonist" type alongside "<mutant>".
func placeholderType(s string) (string, bool) {
	if len(s) < 3 || !strings.HasPrefix(s, "<") || !strings.HasSuffix(s, ">") {
		return "", false
	}
	t := strings.ToLower(strings.TrimSpace(s[1 : len(s)-1]))
	t = strings.TrimSuffix(t, "name")
	if t == "" {
		return "", false
	}
	return t, true
}

// ResolveName replaces a "<type>" placeholder with a freshly generated name of
// that type. Strings that are not placeholders, or reference an unknown type,
// are returned unchanged so the problem stays visible.
func ResolveName(s string) string {
	t, ok := placeholderType(s)
	if !ok || !HasNameType(t) {
		return s
	}
	return RandomName(t)
}

// NameWithEpithet decorates a base name with a title epithet and, sometimes, a
// trailing suffix epithet — e.g. "Warden" -> "Warden the Destroyer" or
// "Warden the Grim, mauler of cities". Used to build boss names.
func NameWithEpithet(base string) string {
	loadNames()
	out := base
	if t := RandomTitleEpithet(); t != "" {
		if out == "" {
			out = t
		} else {
			out = out + " " + t
		}
	}
	if len(names.Epithets.Suffix) > 0 && rand.Intn(2) == 0 {
		out = out + ", " + RandomSuffixEpithet()
	}
	return out
}

func RandomTitleEpithet() string {
	loadNames()
	if len(names.Epithets.Title) == 0 {
		return ""
	}
	return names.Epithets.Title[rand.Intn(len(names.Epithets.Title))]
}

func RandomSuffixEpithet() string {
	loadNames()
	if len(names.Epithets.Suffix) == 0 {
		return ""
	}
	return names.Epithets.Suffix[rand.Intn(len(names.Epithets.Suffix))]
}

func RandomSettlementName() string { return RandomName("settlement") }
