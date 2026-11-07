package game

import (
	"log"

	"github.com/mechanical-lich/landing_party/internal/generation"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// siteLore is everything a site's history adds to a level build: its evidence
// features, rolled origins, chapter and datapads, and its lore flags.
type siteLore struct {
	evidence history.SiteEvidence
	flags    map[string]any
}

// siteLoreFor collects a site's lore, or ok=false for none. The game's
// buildParked and the planet generator's lab both build through it, so a
// preview is what the game builds.
func siteLoreFor(h *history.History, site *history.SiteHistory) (siteLore, bool) {
	if h == nil || site == nil {
		return siteLore{}, false
	}
	lib, err := history.DefaultLibrary()
	if err != nil {
		log.Printf("history: %v", err)
		return siteLore{}, false
	}
	return siteLore{evidence: h.Evidence(lib, site), flags: h.LoreFlags(lib, site)}, true
}

// apply puts the lore into a build's config.
func (l siteLore) apply(cfg *SettlementConfig) {
	cfg.SiteFeatures = l.evidence.Features
	cfg.SiteOrigins = l.evidence.Origins
	cfg.SiteChapter = l.evidence.Chapter
	cfg.SiteFlags = l.flags
}

// setLoreFlags puts the site's history where setup scripts read it: the lore
// flags from the campaign, and the chapter's resolved origin (origin_x/y/z; -1
// when the site has no chapter origin). A script that ignores them behaves as
// it always did.
func (s *MainState) setLoreFlags() {
	for k, v := range s.settlementCfg.SiteFlags {
		s.level.Flags[k] = v
	}
	x, y, z, ok := generation.Origin(s.level, generation.ChapterOrigin)
	if !ok {
		x, y, z = -1, -1, -1
	} else {
		// A region tag too, so tools (vmtest's region:lore_origin) can find it.
		s.level.TagRegion(LoreOriginRegion, x, y, z)
	}
	s.level.Flags["origin_x"] = float64(x)
	s.level.Flags["origin_y"] = float64(y)
	s.level.Flags["origin_z"] = float64(z)
}

// LoreOriginRegion is the level region tag at the site chapter's resolved
// origin.
const LoreOriginRegion = "lore_origin"
