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
	if desc.Faction != "" {
		s += " Faction: " + desc.Faction + "."
	}
	return s
}

// drawWrapped renders text wrapping on word boundaries to fit maxW pixels per
// line, estimating characters at fontSize*6/10 wide. Returns the Y below the
// last line drawn so callers can stack further content.
func drawWrapped(screen *ebiten.Image, text string, fontSize, x, y, maxW, lineH int, col color.Color) int {
	if text == "" {
		return y
	}
	charW := fontSize * 6 / 10
	if charW <= 0 {
		charW = 1
	}
	maxChars := maxW / charW
	if maxChars <= 0 {
		maxChars = 1
	}
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			y += lineH
			continue
		}
		line := ""
		for _, w := range words {
			candidate := w
			if line != "" {
				candidate = line + " " + w
			}
			if len(candidate) > maxChars && line != "" {
				mlge_text.Draw(screen, line, float64(fontSize), x, y, col)
				y += lineH
				line = w
			} else {
				line = candidate
			}
		}
		if line != "" {
			mlge_text.Draw(screen, line, float64(fontSize), x, y, col)
			y += lineH
		}
	}
	return y
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
