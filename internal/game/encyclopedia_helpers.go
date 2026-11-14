package game

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	mlge_text "github.com/mechanical-lich/mlge/text"
)

// techEncyclopedia gates the Encyclopedia dashboard panel.
const techEncyclopedia = "encyclopedia"

// displayName returns the headline for an entry: the description's archetype
// DisplayName if any, otherwise the blueprint ID.
func displayName(blueprint string, desc *rlcomponents.DescriptionComponent) string {
	if desc == nil {
		return blueprint
	}
	if n := desc.DisplayName(); n != "" {
		return n
	}
	return blueprint
}

// synthDescription builds a short natural-language summary from an entry's
// structured fields, so a card always has a readable description even before any
// LongDescription lore is authored.
func synthDescription(blueprint string, desc *rlcomponents.DescriptionComponent) string {
	if desc == nil {
		return "An unidentified object; no data on record."
	}
	subject := strings.TrimSpace(desc.Classification + " " + desc.Species)
	if subject == "" {
		subject = desc.Name
	}
	if subject == "" {
		subject = blueprint
	}
	subject = strings.ToLower(subject)
	article := "A"
	if len(subject) > 0 && strings.ContainsRune("aeiou", rune(subject[0])) {
		article = "An"
	}
	s := article + " " + subject + "."
	if f := encyclopediaFaction(desc.Faction); f != "" {
		s += " Faction: " + f + "."
	}
	return s
}

// encyclopediaFaction is an entry's faction as the Encyclopedia names it:
// the HUD's and the inspector's hidden-identity rule (hiddenFaction), so a
// hive creature's card never says "hive" outside an observer run.
func encyclopediaFaction(faction string) string {
	if hiddenFaction(faction) {
		return ""
	}
	return faction
}

// encyclopediaAttrs is an entry's structured attribute lines (label,
// value), in card order; an empty value is not drawn.
func encyclopediaAttrs(desc *rlcomponents.DescriptionComponent) [][2]string {
	if desc == nil {
		return nil
	}
	return [][2]string{
		{"Species", desc.Species},
		{"Class", desc.Classification},
		{"Faction", encyclopediaFaction(desc.Faction)},
		{"Tags", strings.Join(desc.Tags, ", ")},
	}
}

// encyclopediaID is the card's "id:" footer: the blueprint, "" for a
// hidden-identity faction's entry (its key names it: "hive_heart"), as the
// inspector leaves out its Blueprint line.
func encyclopediaID(blueprint string, desc *rlcomponents.DescriptionComponent) string {
	if desc != nil && hiddenFaction(desc.Faction) {
		return ""
	}
	return blueprint
}

// drawWrapped renders text wrapping on word boundaries to fit maxW pixels per
// line, estimating characters at fontSize*6/10 wide. Returns the Y below the
// last line drawn so callers can stack further content.
func drawWrapped(screen *ebiten.Image, text string, fontSize, x, y, maxW, lineH int, col color.Color) int {
	return drawWrappedClip(screen, text, fontSize, x, y, maxW, lineH, 0, col)
}

// drawWrappedClip is drawWrapped with a floor: no line is drawn whose bottom
// would pass maxY, and the last line that fits ends in an ellipsis when text
// was cut. maxY <= 0 means no limit.
func drawWrappedClip(screen *ebiten.Image, text string, fontSize, x, y, maxW, lineH, maxY int, col color.Color) int {
	lines := clipLines(wrapLines(text, fontSize, maxW), y, lineH, maxY)
	for _, line := range lines {
		if line != "" {
			mlge_text.Draw(screen, line, float64(fontSize), x, y, col)
		}
		y += lineH
	}
	return y
}

// wrapLines splits text into display lines on word boundaries, estimating
// characters at fontSize*6/10 wide. Blank paragraphs become empty lines.
func wrapLines(text string, fontSize, maxW int) []string {
	if text == "" {
		return nil
	}
	charW := fontSize * 6 / 10
	if charW <= 0 {
		charW = 1
	}
	maxChars := maxW / charW
	if maxChars <= 0 {
		maxChars = 1
	}
	var out []string
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := ""
		for _, w := range words {
			candidate := w
			if line != "" {
				candidate = line + " " + w
			}
			if len(candidate) > maxChars && line != "" {
				out = append(out, line)
				line = w
			} else {
				line = candidate
			}
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// clipLines keeps the lines that fit between y and maxY at lineH each,
// marking the last kept line with an ellipsis when any were dropped.
// maxY <= 0 keeps everything.
func clipLines(lines []string, y, lineH, maxY int) []string {
	if maxY <= 0 || lineH <= 0 {
		return lines
	}
	fit := (maxY - y) / lineH
	if fit < 0 {
		fit = 0
	}
	if fit >= len(lines) {
		return lines
	}
	kept := append([]string(nil), lines[:fit]...)
	if fit > 0 {
		kept[fit-1] = strings.TrimRight(kept[fit-1], " .,;:") + "…"
	}
	return kept
}

// itoa avoids pulling strconv into the Draw hot path for tiny integers.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
