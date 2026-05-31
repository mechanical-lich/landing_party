package game

import (
	"image/color"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/state"
	mlge_text "github.com/mechanical-lich/mlge/text"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// Tech gate read by the Encyclopedia entry point on the Star Map.
const techEncyclopedia = "encyclopedia"

// EncyclopediaState is the bestiary "Pokédex" surface — a dedicated screen
// rather than a modal, since reading is a focused activity and the surface
// will grow (datapads tab is on the roadmap). Reached from the Star Map and
// gated behind the "encyclopedia" research; entity registration happens at
// hover time in MainState regardless of unlock, so the list is already
// populated when the player finishes the research.
type EncyclopediaState struct {
	campaign *campaign.Campaign
	wm       *WorldManager

	list      *minui.ListBox
	blueprints []string // parallel to list items
	backBtn   *minui.Button

	// Cached blueprint → DescriptionComponent so we don't allocate a probe
	// entity for every row on every Draw.
	descCache map[string]*rlcomponents.DescriptionComponent

	done bool
	next state.StateInterface
}

func NewEncyclopediaState(c *campaign.Campaign, wm *WorldManager) *EncyclopediaState {
	e := &EncyclopediaState{
		campaign:  c,
		wm:        wm,
		descCache: map[string]*rlcomponents.DescriptionComponent{},
	}
	cfg := config.Global()
	cx := cfg.ScreenWidth / 2

	e.list = minui.NewListBox("ency_list", nil)
	e.list.SetBounds(minui.Rect{X: 40, Y: 210, Width: 320, Height: 470})
	e.list.Layout()
	e.list.OnSelect = func(_ int, _ string) {}

	e.backBtn = minui.NewButton("ency_back", "Back to Star Map")
	e.backBtn.SetPosition(cx-105, cfg.ScreenHeight-70)
	e.backBtn.SetSize(210, 36)
	e.backBtn.OnClick = func() {
		e.next = NewOverworldState(e.campaign, e.wm)
		e.done = true
	}

	e.populate()
	return e
}

// populate refreshes the entity list from Campaign.KnownEntities. Sorted by
// display name so the list is stable across sessions.
func (e *EncyclopediaState) populate() {
	type entry struct {
		bp   string
		name string
	}
	var entries []entry
	for _, bp := range e.campaign.KnownEntities {
		entries = append(entries, entry{bp: bp, name: displayName(bp,e.lookupDescription(bp))})
	}
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].name) < strings.ToLower(entries[j].name)
	})

	items := make([]string, 0, len(entries))
	bps := make([]string, 0, len(entries))
	for _, en := range entries {
		items = append(items, en.name)
		bps = append(bps, en.bp)
	}
	if len(items) == 0 {
		items = []string{"(no entries yet — hover entities to register them)"}
		bps = []string{""}
	}
	e.blueprints = bps
	e.list.SetItems(items)
	if len(bps) > 0 && bps[0] != "" {
		e.list.SelectedIndex = 0
	}
}

// lookupDescription returns a cached DescriptionComponent for blueprint, or
// nil if the blueprint has no Description (or doesn't exist). Uses
// factory.GetDescription so the result is the BLUEPRINT's archetype lore,
// not a freshly-rolled instance (factory.Create would resolve "<mutant>"
// placeholders into a random name each call).
func (e *EncyclopediaState) lookupDescription(blueprint string) *rlcomponents.DescriptionComponent {
	if blueprint == "" {
		return nil
	}
	if d, ok := e.descCache[blueprint]; ok {
		return d
	}
	d := factory.GetDescription(blueprint)
	e.descCache[blueprint] = d
	return d
}

// displayName returns the headline for an entry: the description's archetype
// DisplayName (Species + Classification) if any, otherwise the literal Name,
// otherwise the blueprint ID as a last resort.
func displayName(blueprint string, desc *rlcomponents.DescriptionComponent) string {
	if desc == nil {
		return blueprint
	}
	if n := desc.DisplayName(); n != "" {
		return n
	}
	return blueprint
}

// selectedBlueprint returns the blueprint of the currently-selected list row,
// or "" if nothing real is selected (e.g. the empty-list placeholder).
func (e *EncyclopediaState) selectedBlueprint() string {
	i := e.list.SelectedIndex
	if i < 0 || i >= len(e.blueprints) {
		return ""
	}
	return e.blueprints[i]
}

func (e *EncyclopediaState) Update() state.StateInterface {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		e.next = NewOverworldState(e.campaign, e.wm)
		e.done = true
		return e.next
	}
	e.list.Update()
	e.backBtn.Update()
	return e.next
}

func (e *EncyclopediaState) Draw(screen *ebiten.Image) {
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	cfg := config.Global()
	screen.Fill(color.RGBA{6, 8, 16, 255})

	cx := cfg.ScreenWidth / 2

	title := "Encyclopedia"
	mlge_text.Draw(screen, title, 40, cx-len(title)*40*3/10/2, 100, color.RGBA{120, 200, 255, 255})

	// Subtitle: discovered count and list label, stacked above the listbox
	// (which starts at y=210) with comfortable separation.
	hdr := "Discovered: 0"
	if n := len(e.campaign.KnownEntities); n > 0 {
		hdr = "Discovered: " + itoa(n)
	}
	mlge_text.Draw(screen, hdr, 14, 40, 152, color.RGBA{160, 185, 215, 255})
	mlge_text.Draw(screen, "Catalogue", 16, 40, 188, color.RGBA{200, 220, 255, 255})

	e.list.Draw(screen)
	e.backBtn.Draw(screen)

	// Right pane: details for the selected entry.
	detailX := 400
	detailY := 180
	detailW := cfg.ScreenWidth - detailX - 40

	bp := e.selectedBlueprint()
	if bp == "" {
		mlge_text.Draw(screen, "Select an entry from the catalogue.", 14, detailX, detailY+12, color.RGBA{160, 185, 215, 255})
		minui.FlushOverlays(screen)
		return
	}
	desc := e.lookupDescription(bp)

	// Icon up top of the detail pane. The sprite's actual source size lives
	// on the AppearanceComponent (24 is the game-wide default tile size;
	// items sometimes specify 16). We back-compute a scale so the displayed
	// icon is always ~portraitPx pixels wide regardless of source resolution,
	// otherwise a 16px fallback would crop bigger sprites and a 24px sprite
	// scaled the same as a 16px one would look mismatched next to it.
	const portraitPx = 72
	if ac := factory.GetAppearance(bp); ac != nil && ac.Resource != "" {
		size := ac.SpriteSize
		if size <= 0 {
			size = 24
		}
		scale := float64(portraitPx) / float64(size)
		icon := minui.NewIconWithScale(ac.Resource, ac.SpriteX, ac.SpriteY, size, size, scale)
		if ac.R != 0 || ac.G != 0 || ac.B != 0 {
			icon.Tint = color.RGBA{R: ac.R, G: ac.G, B: ac.B, A: 255}
		}
		icon.Draw(screen, detailX, detailY-4)
	}

	titleX := detailX + portraitPx + 16
	titleY := detailY + 10
	mlge_text.Draw(screen, displayName(bp,desc), 24, titleX, titleY, color.RGBA{230, 240, 255, 255})

	subY := titleY + 30
	if desc != nil && desc.Faction != "" {
		mlge_text.Draw(screen, "Faction: "+desc.Faction, 13, titleX, subY, color.RGBA{170, 200, 240, 255})
		subY += 18
	}
	if desc != nil && len(desc.Tags) > 0 {
		mlge_text.Draw(screen, "Tags: "+strings.Join(desc.Tags, ", "), 13, titleX, subY, color.RGBA{170, 200, 240, 255})
		subY += 18
	}

	// Body: LongDescription, wrapped roughly to fit the detail pane width.
	bodyY := detailY + 110
	bodyText := "No description recorded."
	if desc != nil && desc.LongDescription != "" {
		bodyText = desc.LongDescription
	}
	drawWrapped(screen, bodyText, 14, detailX, bodyY, detailW, 22, color.RGBA{200, 220, 240, 255})

	// Footnote: blueprint ID in muted text so the player (and devs) can
	// cross-reference with data files.
	mlge_text.Draw(screen, "id: "+bp, 11, detailX, cfg.ScreenHeight-90, color.RGBA{90, 110, 130, 200})

	minui.FlushOverlays(screen)
}

func (e *EncyclopediaState) Done() bool { return e.done }

// drawWrapped renders text wrapping on word boundaries to fit maxW pixels per
// line. Uses the existing mlge_text.Draw renderer; characters are estimated at
// fontSize * 6/10 wide so longer lines wrap before overflowing.
func drawWrapped(screen *ebiten.Image, text string, fontSize, x, y, maxW, lineH int, col color.Color) {
	if text == "" {
		return
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
