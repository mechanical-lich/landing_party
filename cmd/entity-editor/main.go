package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	mlge_text "github.com/mechanical-lich/mlge/text"
	"github.com/mechanical-lich/mlge/ui/minui"
)

const (
	screenW    = 1400
	screenH    = 900
	tileSize   = 24
	zoom       = 2
	panelList  = 260
	panelProps = 300
	sheetAreaY = 50
)

// ── Blueprint types ───────────────────────────────────────────────────────────

type Appearance struct {
	R          uint8  `json:"R"`
	G          uint8  `json:"G"`
	B          uint8  `json:"B"`
	Bounces    bool   `json:"Bounces,omitempty"`
	BounceAxis string `json:"BounceAxis,omitempty"`
	Resource   string `json:"Resource"`
	SpriteX    int    `json:"SpriteX"`
	SpriteY    int    `json:"SpriteY"`
	SpriteSize int    `json:"SpriteSize,omitempty"`
}

type EquipmentAppearanceData struct {
	Resource            string         `json:"Resource"`
	SpriteSize          int            `json:"SpriteSize"`
	BlockOriginX        int            `json:"BlockOriginX"`
	BlockOriginY        int            `json:"BlockOriginY"`
	AnimationFrames     int            `json:"AnimationFrames"`
	DefaultWeaponColumn int            `json:"DefaultWeaponColumn"`
	DefaultArmorRow     int            `json:"DefaultArmorRow"`
	WeaponColumns       map[string]int `json:"WeaponColumns,omitempty"`
	ArmorRows           []ArmorRowData `json:"ArmorRows,omitempty"`
}

type ArmorRowData struct {
	Row  int      `json:"Row"`
	Tags []string `json:"Tags"`
}

type WeaponData struct {
	AttackBonus        int    `json:"AttackBonus"`
	AttackDice         string `json:"AttackDice"`
	DamageType         string `json:"DamageType"`
	Range              int    `json:"Range"`
	Ranged             bool   `json:"Ranged"`
	ProjectileX        int    `json:"ProjectileX,omitempty"`
	ProjectileY        int    `json:"ProjectileY,omitempty"`
	ProjectileResource string `json:"ProjectileResource,omitempty"`
	Display            string `json:"Display,omitempty"`
}

type ArmorData struct {
	DefenseBonus  int      `json:"DefenseBonus"`
	Resistances   []string `json:"Resistances,omitempty"`
	StoppingPower int      `json:"StoppingPower,omitempty"`
	Tags          []string `json:"Tags,omitempty"`
}

var itemSheets = map[string]int{
	"scifi_items":   16,
	"fantasy_items": 16,
}

func spriteSize(resource string) int {
	if s, ok := itemSheets[resource]; ok {
		return s
	}
	return tileSize
}

type Blueprint map[string]json.RawMessage

// componentNames mirrors the RegisterComponent(...) calls in
// internal/factory/component_registry.go. Keep in sync when components are
// added there. Used to populate the "Add Component" picker.
var componentNames = []string{
	"AIMemory", "Appearance", "Armor", "Choppable", "CraftingStation",
	"Dead", "DefensiveAI", "Description", "Direction", "Door", "Drops",
	"EquipmentAppearance", "FactionAI", "FX", "Food", "Health", "HostileAI",
	"Hunger", "Inanimate", "Initiative", "Inventory", "Item", "LaserBeam",
	"Light", "MyTurn", "NeverSleep", "Nocturnal", "Position", "ResearchBuilding",
	"ResourceItem", "Script", "ScriptedAI", "Selected", "Settlement", "Skills",
	"Solid", "Stats", "Storage", "WanderAI", "Weapon", "Worker",
}

// defaultJSONFor returns a skeleton JSON body for a freshly added component.
// Typed-form components get a useful starting shape; everything else gets {}.
func defaultJSONFor(name string) string {
	switch name {
	case "Script":
		return `{"on_turn":""}`
	case "ScriptedAI":
		return `{"script":"","vars":{}}`
	case "Description":
		return `{"Name":""}`
	case "Health":
		return `{"MaxHealth":10,"Health":10}`
	case "Stats":
		return `{"AC":10}`
	case "Appearance":
		return `{"R":255,"G":255,"B":255,"Resource":"","SpriteX":0,"SpriteY":0}`
	default:
		return `{}`
	}
}

// typedFormComponents are the components with a dedicated field editor.
// Anything not listed falls back to the raw-JSON editor.
var typedFormComponents = map[string]bool{
	"Script": true, "ScriptedAI": true, "Description": true,
	"Health": true, "Stats": true, "Weapon": true, "Armor": true,
}

// ── Typed component shapes (only the fields the editor exposes) ────────────────

type ScriptData struct {
	OnTurn string `json:"on_turn"`
}

type ScriptedAIData struct {
	Script string         `json:"script"`
	Vars   map[string]any `json:"vars,omitempty"`
}

// DescriptionData / HealthData / StatsData use PascalCase keys with no struct
// tags because rlcomponents serialize with default Go field names.
type DescriptionData struct {
	Name            string
	Faction         string
	ID              string
	LongDescription string
}

type HealthData struct {
	MaxHealth int
	Health    int
	Energy    int
}

type StatsData struct {
	AC                int
	Str               int
	Dex               int
	Con               int
	Int               int
	Wis               int
	MeleeAttackBonus  int
	RangedAttackBonus int
}

// repoRelPath converts an absolute path to one relative to the project root
// (the dir containing data/), matching the blueprint convention
// (e.g. "data/scripts/xeno/egg.basic"). Falls back to the input on failure.
func repoRelPath(abs string) string {
	abs = filepath.Clean(abs)
	if i := strings.LastIndex(abs, string(filepath.Separator)+"data"+string(filepath.Separator)); i >= 0 {
		return filepath.ToSlash(abs[i+1:])
	}
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, abs); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(abs)
}

// ── Editor ────────────────────────────────────────────────────────────────────

type Editor struct {
	blueprints map[string]Blueprint
	bpPath     string
	assetsPath string
	bpNames    []string

	sheets    map[string]*ebiten.Image
	sheetKeys []string

	selectedBP  string
	activeSheet int

	sheetScrollX int
	sheetScrollY int
	listScrollY  int

	hoverSX, hoverSY int
	hoverValid       bool

	dragging    bool
	dragStartX  int
	dragStartY  int
	dragScrollX int
	dragScrollY int

	dirty bool

	tick   int
	bounce bool

	colorFocus int
	colorInput string

	// text input state — one active field at a time
	textInputTarget   string // "eac_weapon_key", "eac_row_tag", "weapon_display"
	textInputValue    string
	eacNewWeaponCol   int
	eacNewArmorRowNum int
	eacTagRowIdx      int // which ArmorRows index to add a tag to

	op *ebiten.DrawImageOptions

	gui         *minui.GUI
	fileModal   *minui.FileModal
	scriptModal *minui.FileModal

	// Authoring state
	selectedComponent string // component shown in the editor below the list
	overlay           string // "", "name", "addcomp", "confirm"
	overlayMode       string // for "name": "new" | "dup"; for "confirm": action key
	overlayMsg        string // confirm prompt / error text
	confirmAction     func() // executed when the confirm overlay is accepted
	confirmReturn     string // overlay to restore after a confirm prompt
	addCompScroll     int

	// Raw-JSON fallback editor
	jsonEdit       string // working buffer
	jsonEditTarget string // component the buffer belongs to ("" = inactive)
	jsonEditErr    string // last parse error

	// Manage Components modal state
	manageAdding    bool
	manageScroll    int
	manageGeomCache *manageGeom

	// Geometry captured during Draw for next-frame hit-testing
	compEditorGeom *compEditorGeometry
}

func NewEditor(bpPath, assetsPath string) (*Editor, error) {
	e := &Editor{
		bpPath:       bpPath,
		assetsPath:   assetsPath,
		sheets:       make(map[string]*ebiten.Image),
		op:           &ebiten.DrawImageOptions{},
		eacTagRowIdx: -1,
	}

	data, err := os.ReadFile(bpPath)
	if err != nil {
		return nil, fmt.Errorf("load blueprints: %w", err)
	}
	if err := json.Unmarshal(data, &e.blueprints); err != nil {
		return nil, fmt.Errorf("parse blueprints: %w", err)
	}
	for k := range e.blueprints {
		e.bpNames = append(e.bpNames, k)
	}
	sort.Strings(e.bpNames)
	if len(e.bpNames) > 0 {
		e.selectedBP = e.bpNames[0]
	}

	if err := e.reloadSheets(); err != nil {
		return nil, err
	}

	e.gui = minui.NewGUI()
	fm := minui.NewFileModal("open_file", "Open Blueprint File", 580, 400, "load")
	fm.SetPosition(panelList+40, 60)
	cwd, _ := os.Getwd()
	fm.SetDefaultPath(filepath.Join(cwd, "data", "blueprints"))
	fm.OnSelect = func(path string) {
		if err := e.loadFile(path); err != nil {
			log.Printf("open file: %v", err)
		}
	}
	fm.SetVisible(false)
	e.fileModal = fm
	e.gui.AddModal(fm.Modal)

	sm := minui.NewFileModal("script_file", "Choose Script (.basic)", 580, 400, "load")
	sm.SetPosition(panelList+40, 60)
	sm.SetDefaultPath(filepath.Join(cwd, "data", "scripts"))
	sm.OnSelect = func(path string) {
		e.setScriptPath(repoRelPath(path))
	}
	sm.SetVisible(false)
	e.scriptModal = sm
	e.gui.AddModal(sm.Modal)

	return e, nil
}

// rebuildNames refreshes the sorted blueprint name list and keeps a valid
// selection. Pass a preferred name to select after the rebuild.
func (e *Editor) rebuildNames(prefer string) {
	e.bpNames = e.bpNames[:0]
	for k := range e.blueprints {
		e.bpNames = append(e.bpNames, k)
	}
	sort.Strings(e.bpNames)
	if _, ok := e.blueprints[prefer]; ok {
		e.selectedBP = prefer
	} else if _, ok := e.blueprints[e.selectedBP]; !ok {
		if len(e.bpNames) > 0 {
			e.selectedBP = e.bpNames[0]
		} else {
			e.selectedBP = ""
		}
	}
}

// addBlueprint creates a new blueprint. If src is non-nil its components are
// deep-copied (duplicate). Returns false if the name is empty or taken.
func (e *Editor) addBlueprint(name string, src Blueprint) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if _, exists := e.blueprints[name]; exists {
		return false
	}
	bp := Blueprint{}
	for k, v := range src {
		cp := make(json.RawMessage, len(v))
		copy(cp, v)
		bp[k] = cp
	}
	e.blueprints[name] = bp
	e.dirty = true
	e.rebuildNames(name)
	e.reloadSheets()
	return true
}

func (e *Editor) deleteBlueprint(name string) {
	delete(e.blueprints, name)
	e.dirty = true
	e.selectedComponent = ""
	e.jsonEditTarget = ""
	e.rebuildNames("")
}

// setScriptPath writes the chosen path into the active script component,
// based on which Browse button opened the picker.
func (e *Editor) setScriptPath(path string) {
	bp, ok := e.blueprints[e.selectedBP]
	if !ok {
		return
	}
	switch e.selectedComponent {
	case "Script":
		s := ScriptData{OnTurn: path}
		raw, _ := json.Marshal(s)
		bp["Script"] = raw
		e.dirty = true
	case "ScriptedAI":
		var s ScriptedAIData
		if raw, ok := bp["ScriptedAI"]; ok {
			json.Unmarshal(raw, &s)
		}
		s.Script = path
		raw, _ := json.Marshal(s)
		bp["ScriptedAI"] = raw
		e.dirty = true
	}
}

func (e *Editor) reloadSheets() error {
	assetData, err := os.ReadFile(e.assetsPath)
	if err != nil {
		return fmt.Errorf("load assets: %w", err)
	}
	var assets map[string]string
	if err := json.Unmarshal(assetData, &assets); err != nil {
		return fmt.Errorf("parse assets: %w", err)
	}

	needed := map[string]bool{}
	for _, bp := range e.blueprints {
		if raw, ok := bp["Appearance"]; ok {
			var ap Appearance
			if json.Unmarshal(raw, &ap) == nil && ap.Resource != "" {
				needed[ap.Resource] = true
			}
		}
		if raw, ok := bp["EquipmentAppearance"]; ok {
			var eac EquipmentAppearanceData
			if json.Unmarshal(raw, &eac) == nil && eac.Resource != "" {
				needed[eac.Resource] = true
			}
		}
	}

	e.sheets = make(map[string]*ebiten.Image)
	e.sheetKeys = nil
	for key, path := range assets {
		if !needed[key] {
			continue
		}
		fullPath := filepath.Join(filepath.Dir(e.assetsPath), "..", path)
		f, err := os.Open(fullPath)
		if err != nil {
			log.Printf("warning: %s: %v", path, err)
			continue
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			log.Printf("warning: decode %s: %v", path, err)
			continue
		}
		e.sheets[key] = ebiten.NewImageFromImage(img)
		e.sheetKeys = append(e.sheetKeys, key)
	}
	sort.Strings(e.sheetKeys)
	return nil
}

func (e *Editor) loadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var loaded map[string]Blueprint
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	e.blueprints = loaded
	e.bpNames = nil
	for k := range e.blueprints {
		e.bpNames = append(e.bpNames, k)
	}
	sort.Strings(e.bpNames)
	if len(e.bpNames) > 0 {
		e.selectedBP = e.bpNames[0]
	}
	e.bpPath = path
	e.dirty = false
	e.listScrollY = 0
	e.sheetScrollX = 0
	e.sheetScrollY = 0
	e.textInputTarget = ""
	e.textInputValue = ""
	return e.reloadSheets()
}

func (e *Editor) Save() error {
	data, err := json.MarshalIndent(e.blueprints, "", "    ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(e.bpPath, data, 0644); err != nil {
		return err
	}
	e.dirty = false
	return nil
}

// ── Component helpers ─────────────────────────────────────────────────────────

func (e *Editor) getAppearance() *Appearance {
	bp, ok := e.blueprints[e.selectedBP]
	if !ok {
		return nil
	}
	raw, ok := bp["Appearance"]
	if !ok {
		return nil
	}
	var ap Appearance
	if err := json.Unmarshal(raw, &ap); err != nil {
		return nil
	}
	return &ap
}

func (e *Editor) setAppearance(ap Appearance) {
	bp := e.blueprints[e.selectedBP]
	raw, err := json.Marshal(ap)
	if err != nil {
		return
	}
	bp["Appearance"] = json.RawMessage(raw)
	e.blueprints[e.selectedBP] = bp
	e.dirty = true
}

func (e *Editor) getEquipmentAppearance() *EquipmentAppearanceData {
	bp, ok := e.blueprints[e.selectedBP]
	if !ok {
		return nil
	}
	raw, ok := bp["EquipmentAppearance"]
	if !ok {
		return nil
	}
	var eac EquipmentAppearanceData
	if err := json.Unmarshal(raw, &eac); err != nil {
		return nil
	}
	return &eac
}

func (e *Editor) setEquipmentAppearance(eac EquipmentAppearanceData) {
	bp := e.blueprints[e.selectedBP]
	raw, err := json.Marshal(eac)
	if err != nil {
		return
	}
	bp["EquipmentAppearance"] = json.RawMessage(raw)
	e.blueprints[e.selectedBP] = bp
	e.dirty = true
}

func (e *Editor) getWeapon() *WeaponData {
	bp, ok := e.blueprints[e.selectedBP]
	if !ok {
		return nil
	}
	raw, ok := bp["Weapon"]
	if !ok {
		return nil
	}
	var w WeaponData
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil
	}
	return &w
}

func (e *Editor) setWeapon(w *WeaponData) {
	bp := e.blueprints[e.selectedBP]
	raw, err := json.Marshal(w)
	if err != nil {
		return
	}
	bp["Weapon"] = json.RawMessage(raw)
	e.blueprints[e.selectedBP] = bp
	e.dirty = true
}

func (e *Editor) getArmor() *ArmorData {
	bp, ok := e.blueprints[e.selectedBP]
	if !ok {
		return nil
	}
	raw, ok := bp["Armor"]
	if !ok {
		return nil
	}
	var a ArmorData
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil
	}
	return &a
}

func (e *Editor) setArmor(a *ArmorData) {
	bp := e.blueprints[e.selectedBP]
	raw, err := json.Marshal(a)
	if err != nil {
		return
	}
	bp["Armor"] = json.RawMessage(raw)
	e.blueprints[e.selectedBP] = bp
	e.dirty = true
}

// getComp/setComp round-trip a typed value through the selected blueprint's
// raw-JSON component map (same pattern as getWeapon/setWeapon).
func (e *Editor) getComp(name string, out any) bool {
	bp, ok := e.blueprints[e.selectedBP]
	if !ok {
		return false
	}
	raw, ok := bp[name]
	if !ok {
		return false
	}
	return json.Unmarshal(raw, out) == nil
}

func (e *Editor) setComp(name string, v any) {
	bp, ok := e.blueprints[e.selectedBP]
	if !ok {
		return
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return
	}
	bp[name] = json.RawMessage(raw)
	e.dirty = true
}

func (e *Editor) getScript() *ScriptData {
	var s ScriptData
	if !e.getComp("Script", &s) {
		return nil
	}
	return &s
}
func (e *Editor) setScript(s *ScriptData) { e.setComp("Script", s) }

func (e *Editor) getScriptedAI() *ScriptedAIData {
	var s ScriptedAIData
	if !e.getComp("ScriptedAI", &s) {
		return nil
	}
	return &s
}
func (e *Editor) setScriptedAI(s *ScriptedAIData) { e.setComp("ScriptedAI", s) }

func (e *Editor) getDescription() *DescriptionData {
	var d DescriptionData
	if !e.getComp("Description", &d) {
		return nil
	}
	return &d
}
func (e *Editor) setDescription(d *DescriptionData) { e.setComp("Description", d) }

func (e *Editor) getHealth() *HealthData {
	var h HealthData
	if !e.getComp("Health", &h) {
		return nil
	}
	return &h
}
func (e *Editor) setHealth(h *HealthData) { e.setComp("Health", h) }

func (e *Editor) getStats() *StatsData {
	var s StatsData
	if !e.getComp("Stats", &s) {
		return nil
	}
	return &s
}
func (e *Editor) setStats(s *StatsData) { e.setComp("Stats", s) }

func (e *Editor) hasEAC() bool {
	bp, ok := e.blueprints[e.selectedBP]
	if !ok {
		return false
	}
	_, ok = bp["EquipmentAppearance"]
	return ok
}

func (e *Editor) blueprintAppearance(name string) *Appearance {
	bp, ok := e.blueprints[name]
	if !ok {
		return nil
	}
	raw, ok := bp["Appearance"]
	if !ok {
		return nil
	}
	var ap Appearance
	if json.Unmarshal(raw, &ap) != nil {
		return nil
	}
	return &ap
}

func (e *Editor) blueprintEAC(name string) *EquipmentAppearanceData {
	bp, ok := e.blueprints[name]
	if !ok {
		return nil
	}
	raw, ok := bp["EquipmentAppearance"]
	if !ok {
		return nil
	}
	var eac EquipmentAppearanceData
	if json.Unmarshal(raw, &eac) != nil {
		return nil
	}
	return &eac
}

func (e *Editor) currentSheet() *ebiten.Image {
	if eac := e.getEquipmentAppearance(); eac != nil && eac.Resource != "" {
		if img, ok := e.sheets[eac.Resource]; ok {
			return img
		}
	}
	ap := e.getAppearance()
	if ap != nil && ap.Resource != "" {
		if img, ok := e.sheets[ap.Resource]; ok {
			return img
		}
	}
	if len(e.sheetKeys) == 0 {
		return nil
	}
	return e.sheets[e.sheetKeys[e.activeSheet]]
}

func (e *Editor) currentSpriteSize() int {
	if eac := e.getEquipmentAppearance(); eac != nil && eac.SpriteSize > 0 {
		return eac.SpriteSize
	}
	ap := e.getAppearance()
	if ap != nil {
		return spriteSize(ap.Resource)
	}
	return tileSize
}

func (e *Editor) currentResource() string {
	if eac := e.getEquipmentAppearance(); eac != nil {
		return eac.Resource
	}
	if ap := e.getAppearance(); ap != nil {
		return ap.Resource
	}
	if len(e.sheetKeys) > 0 {
		return e.sheetKeys[e.activeSheet]
	}
	return ""
}

// ── Text input ────────────────────────────────────────────────────────────────

func (e *Editor) updateTextInput() {
	if e.textInputTarget == "" {
		return
	}
	for _, ch := range ebiten.AppendInputChars(nil) {
		e.textInputValue += string(ch)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(e.textInputValue) > 0 {
		runes := []rune(e.textInputValue)
		e.textInputValue = string(runes[:len(runes)-1])
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		e.textInputTarget = ""
		e.textInputValue = ""
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		e.commitTextInput()
	}
}

func (e *Editor) commitTextInput() {
	// Generic component-field commit: target is "<Component>.<label>".
	if e.selectedComponent != "" && strings.HasPrefix(e.textInputTarget, e.selectedComponent+".") {
		label := strings.TrimPrefix(e.textInputTarget, e.selectedComponent+".")
		for _, f := range e.fieldsFor(e.selectedComponent) {
			if f.label == label && f.set != nil {
				f.set(e.textInputValue)
				break
			}
		}
		e.textInputTarget, e.textInputValue = "", ""
		return
	}
	switch e.textInputTarget {
	case "eac_weapon_key":
		if e.textInputValue != "" {
			eac := e.getEquipmentAppearance()
			if eac != nil {
				if eac.WeaponColumns == nil {
					eac.WeaponColumns = make(map[string]int)
				}
				eac.WeaponColumns[e.textInputValue] = e.eacNewWeaponCol
				e.setEquipmentAppearance(*eac)
			}
			e.textInputValue = ""
			e.textInputTarget = ""
		}
	case "eac_row_tag":
		if e.textInputValue != "" {
			eac := e.getEquipmentAppearance()
			if eac != nil && e.eacTagRowIdx >= 0 && e.eacTagRowIdx < len(eac.ArmorRows) {
				eac.ArmorRows[e.eacTagRowIdx].Tags = append(eac.ArmorRows[e.eacTagRowIdx].Tags, e.textInputValue)
				e.setEquipmentAppearance(*eac)
			}
			e.textInputValue = ""
			e.textInputTarget = ""
			e.eacTagRowIdx = -1
		}
	case "bp_name":
		e.acceptNameOverlay()
	}
}

func (e *Editor) focusInput(target, initial string) {
	e.textInputTarget = target
	e.textInputValue = initial
}

// ── Overlays (New / Duplicate name, Add Component, Confirm) ───────────────────

func (e *Editor) openNameOverlay(mode string) {
	e.overlay = "name"
	e.overlayMode = mode
	e.overlayMsg = ""
	e.focusInput("bp_name", "")
}

func (e *Editor) acceptNameOverlay() {
	name := strings.TrimSpace(e.textInputValue)
	if name == "" {
		e.overlayMsg = "Name cannot be empty"
		return
	}
	if _, exists := e.blueprints[name]; exists {
		e.overlayMsg = "Name already exists"
		return
	}
	var src Blueprint
	if e.overlayMode == "dup" {
		src = e.blueprints[e.selectedBP]
	}
	e.addBlueprint(name, src)
	e.closeOverlay()
}

func (e *Editor) confirm(msg string, action func()) {
	e.confirmReturn = e.overlay // restore this overlay after the prompt resolves
	e.overlay = "confirm"
	e.overlayMsg = msg
	e.confirmAction = action
}

func (e *Editor) closeOverlay() {
	e.overlay = e.confirmReturn
	e.confirmReturn = ""
	e.overlayMode = ""
	e.overlayMsg = ""
	e.confirmAction = nil
	if e.textInputTarget == "bp_name" {
		e.textInputTarget, e.textInputValue = "", ""
	}
}

// overlayRect returns the centered modal rect for the current overlay.
func overlayRect() (x, y, w, h int) {
	w, h = 460, 320
	return (screenW - w) / 2, (screenH - h) / 2, w, h
}

func (e *Editor) drawOverlay(screen *ebiten.Image) {
	if e.overlay == "" {
		return
	}
	if e.overlay == "manage" {
		e.drawManageModal(screen)
		return
	}
	drawRect(screen, 0, 0, screenW, screenH, color.RGBA{0, 0, 0, 150})
	ox, oy, ow, oh := overlayRect()
	drawRect(screen, ox, oy, ow, oh, color.RGBA{34, 38, 50, 255})
	drawOutlineRect(screen, ox, oy, ow, oh, color.RGBA{90, 120, 170, 255})

	switch e.overlay {
	case "name":
		title := "New Blueprint"
		if e.overlayMode == "dup" {
			title = "Duplicate \"" + e.selectedBP + "\" as:"
		}
		mlge_text.Draw(screen, title, 16, ox+16, oy+16, color.RGBA{200, 220, 255, 255})
		drawRect(screen, ox+16, oy+50, ow-32, 26, color.RGBA{50, 60, 80, 255})
		drawOutlineRect(screen, ox+16, oy+50, ow-32, 26, color.RGBA{80, 110, 150, 255})
		mlge_text.Draw(screen, e.textInputValue+"|", 16, ox+22, oy+56, color.RGBA{240, 240, 255, 255})
		if e.overlayMsg != "" {
			mlge_text.Draw(screen, e.overlayMsg, 14, ox+16, oy+86, color.RGBA{255, 140, 140, 255})
		}
		e.drawOverlayButton(screen, ox+ow-200, oy+oh-46, 90, 30, "Create", color.RGBA{40, 90, 60, 255})
		e.drawOverlayButton(screen, ox+ow-104, oy+oh-46, 90, 30, "Cancel", color.RGBA{70, 50, 50, 255})
	case "confirm":
		mlge_text.Draw(screen, e.overlayMsg, 16, ox+16, oy+24, color.RGBA{255, 220, 180, 255})
		e.drawOverlayButton(screen, ox+ow-200, oy+oh-46, 90, 30, "Yes", color.RGBA{90, 50, 50, 255})
		e.drawOverlayButton(screen, ox+ow-104, oy+oh-46, 90, 30, "No", color.RGBA{50, 60, 80, 255})
	case "addcomp":
		mlge_text.Draw(screen, "Add Component", 16, ox+16, oy+16, color.RGBA{200, 220, 255, 255})
		bp := e.blueprints[e.selectedBP]
		listY := oy + 44
		listH := oh - 100
		rowH := 20
		visible := listH / rowH
		avail := e.availableComponents(bp)
		if e.addCompScroll > len(avail)-visible {
			e.addCompScroll = len(avail) - visible
		}
		if e.addCompScroll < 0 {
			e.addCompScroll = 0
		}
		mx, my := ebiten.CursorPosition()
		for i := 0; i < visible && i+e.addCompScroll < len(avail); i++ {
			name := avail[i+e.addCompScroll]
			ry := listY + i*rowH
			bg := color.RGBA{40, 44, 58, 255}
			if mx >= ox+16 && mx < ox+ow-16 && my >= ry && my < ry+rowH-1 {
				bg = color.RGBA{60, 90, 140, 255}
			}
			drawRect(screen, ox+16, ry, ow-32, rowH-1, bg)
			mlge_text.Draw(screen, name, 14, ox+22, ry+4, color.RGBA{220, 235, 255, 255})
		}
		mlge_text.Draw(screen, "scroll=wheel  click=add", 12, ox+16, oy+oh-58, color.RGBA{120, 140, 160, 255})
		e.drawOverlayButton(screen, ox+ow-104, oy+oh-46, 90, 30, "Close", color.RGBA{50, 60, 80, 255})
	}
}

func (e *Editor) drawOverlayButton(screen *ebiten.Image, x, y, w, h int, label string, c color.RGBA) {
	drawRect(screen, x, y, w, h, c)
	drawOutlineRect(screen, x, y, w, h, color.RGBA{150, 170, 200, 255})
	mlge_text.Draw(screen, label, 15, x+10, y+h/2-6, color.RGBA{230, 240, 255, 255})
}

func ptIn(mx, my, x, y, w, h int) bool {
	return mx >= x && mx < x+w && my >= y && my < y+h
}

func (e *Editor) availableComponents(bp Blueprint) []string {
	var out []string
	for _, n := range componentNames {
		if _, has := bp[n]; !has {
			out = append(out, n)
		}
	}
	return out
}

// handleOverlayInput processes input while an overlay is open and reports
// whether it consumed the frame's input.
func (e *Editor) handleOverlayInput() bool {
	if e.overlay == "" {
		return false
	}
	mx, my := ebiten.CursorPosition()

	if e.overlay == "manage" {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) &&
			e.selectedComponent != "" && strings.HasPrefix(e.textInputTarget, e.selectedComponent+".") {
			e.commitTextInput()
		}
		e.handleManageInput(mx, my)
		return true
	}

	ox, oy, ow, oh := overlayRect()
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		e.closeOverlay()
		return true
	}

	switch e.overlay {
	case "name":
		// text typing handled by updateTextInput (target bp_name)
		if click && ptIn(mx, my, ox+ow-200, oy+oh-46, 90, 30) {
			e.acceptNameOverlay()
		}
		if click && ptIn(mx, my, ox+ow-104, oy+oh-46, 90, 30) {
			e.closeOverlay()
		}
	case "confirm":
		if click && ptIn(mx, my, ox+ow-200, oy+oh-46, 90, 30) {
			act := e.confirmAction
			e.closeOverlay()
			if act != nil {
				act()
			}
		}
		if click && ptIn(mx, my, ox+ow-104, oy+oh-46, 90, 30) {
			e.closeOverlay()
		}
	case "addcomp":
		_, dy := ebiten.Wheel()
		e.addCompScroll -= int(dy)
		bp := e.blueprints[e.selectedBP]
		avail := e.availableComponents(bp)
		listY := oy + 44
		listH := oh - 100
		rowH := 20
		visible := listH / rowH
		if click && ptIn(mx, my, ox+ow-104, oy+oh-46, 90, 30) {
			e.closeOverlay()
			return true
		}
		if click {
			for i := 0; i < visible && i+e.addCompScroll < len(avail); i++ {
				ry := listY + i*rowH
				if ptIn(mx, my, ox+16, ry, ow-32, rowH-1) {
					name := avail[i+e.addCompScroll]
					bp[name] = json.RawMessage(defaultJSONFor(name))
					e.dirty = true
					e.selectedComponent = name
					e.jsonEditTarget = ""
					if name == "Appearance" || name == "EquipmentAppearance" {
						e.reloadSheets()
					}
					e.closeOverlay()
					break
				}
			}
		}
	}
	return true
}

// ── Sheet panel helpers ───────────────────────────────────────────────────────

func (e *Editor) sheetPanelX() int { return panelList }
func (e *Editor) sheetPanelW() int { return screenW - panelList - panelProps }

// ── Update ────────────────────────────────────────────────────────────────────

func (e *Editor) Update() error {
	e.tick++
	if e.tick%30 == 0 {
		e.bounce = !e.bounce
	}

	e.updateTextInput()
	e.updateJSONEditInput()

	mx, my := ebiten.CursorPosition()

	if inpututil.IsKeyJustPressed(ebiten.KeyS) && ebiten.IsKeyPressed(ebiten.KeyControl) {
		if err := e.Save(); err != nil {
			log.Printf("save error: %v", err)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyO) && ebiten.IsKeyPressed(ebiten.KeyControl) {
		e.fileModal.SetVisible(true)
	}

	e.gui.Update()

	if e.fileModal.IsVisible() || e.scriptModal.IsVisible() {
		return nil
	}

	if e.overlay != "" {
		e.handleOverlayInput()
		return nil
	}

	// Click anywhere outside the props panel clears text input focus
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && mx < screenW-panelProps {
		if e.textInputTarget == "weapon_display" {
			e.commitTextInput()
		}
	}

	// Click Open button
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) &&
		mx >= panelList-58 && mx < panelList-6 && my >= 4 && my < 26 {
		e.fileModal.SetVisible(true)
	}

	// Click New/Dup/Del toolbar
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && my >= 30 && my < 50 {
		for _, b := range listToolbar() {
			if mx >= b.x && mx < b.x+b.w {
				switch b.label {
				case "New":
					e.openNameOverlay("new")
				case "Dup":
					if e.selectedBP != "" {
						e.openNameOverlay("dup")
					}
				case "Del":
					if e.selectedBP != "" {
						name := e.selectedBP
						e.confirm("Delete blueprint \""+name+"\"?", func() {
							e.deleteBlueprint(name)
						})
					}
				}
			}
		}
	}

	// Click tile list
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && mx < panelList && my >= listTop {
		itemH := 22
		idx := (my + e.listScrollY - listTop) / itemH
		if idx >= 0 && idx < len(e.bpNames) {
			e.selectedBP = e.bpNames[idx]
			e.selectedComponent = ""
			e.jsonEditTarget = ""
			e.sheetScrollX = 0
			e.sheetScrollY = 0
			e.textInputTarget = ""
			e.textInputValue = ""
			e.eacTagRowIdx = -1
			// Scroll sheet to current sprite
			if eac := e.getEquipmentAppearance(); eac != nil {
				e.sheetScrollX = eac.BlockOriginX*zoom - e.sheetPanelW()/2 + tileSize*zoom/2
				e.sheetScrollY = eac.BlockOriginY*zoom - (screenH-sheetAreaY)/2 + tileSize*zoom/2
			} else if ap := e.getAppearance(); ap != nil {
				e.sheetScrollX = ap.SpriteX*zoom - e.sheetPanelW()/2 + tileSize*zoom/2
				e.sheetScrollY = ap.SpriteY*zoom - (screenH-sheetAreaY)/2 + tileSize*zoom/2
			}
			if e.sheetScrollX < 0 {
				e.sheetScrollX = 0
			}
			if e.sheetScrollY < 0 {
				e.sheetScrollY = 0
			}
		}
	}

	// List scroll
	if mx < panelList {
		_, dy := ebiten.Wheel()
		e.listScrollY -= int(dy) * 20
		if e.listScrollY < 0 {
			e.listScrollY = 0
		}
	}

	// Sheet panel
	spx := e.sheetPanelX()
	spw := e.sheetPanelW()
	if mx >= spx && mx < spx+spw && my >= sheetAreaY {
		sheet := e.currentSheet()
		if sheet != nil {
			sw := sheet.Bounds().Dx() * zoom
			sh := sheet.Bounds().Dy() * zoom
			areaH := screenH - sheetAreaY

			dx, dy := ebiten.Wheel()
			if ebiten.IsKeyPressed(ebiten.KeyShift) {
				e.sheetScrollX -= int(dx+dy) * tileSize * zoom
			} else {
				e.sheetScrollY -= int(dy) * tileSize * zoom
				e.sheetScrollX -= int(dx) * tileSize * zoom
			}
			maxX := sw - spw
			if maxX < 0 {
				maxX = 0
			}
			maxY := sh - areaH
			if maxY < 0 {
				maxY = 0
			}
			if e.sheetScrollX < 0 {
				e.sheetScrollX = 0
			}
			if e.sheetScrollX > maxX {
				e.sheetScrollX = maxX
			}
			if e.sheetScrollY < 0 {
				e.sheetScrollY = 0
			}
			if e.sheetScrollY > maxY {
				e.sheetScrollY = maxY
			}

			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) {
				e.dragging = true
				e.dragStartX, e.dragStartY = mx, my
				e.dragScrollX, e.dragScrollY = e.sheetScrollX, e.sheetScrollY
			}
			if e.dragging {
				if ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle) {
					e.sheetScrollX = e.dragScrollX - (mx - e.dragStartX)
					e.sheetScrollY = e.dragScrollY - (my - e.dragStartY)
					if e.sheetScrollX < 0 {
						e.sheetScrollX = 0
					}
					if e.sheetScrollX > maxX {
						e.sheetScrollX = maxX
					}
					if e.sheetScrollY < 0 {
						e.sheetScrollY = 0
					}
					if e.sheetScrollY > maxY {
						e.sheetScrollY = maxY
					}
				} else {
					e.dragging = false
				}
			}

			ss := e.currentSpriteSize()
			e.hoverSX = ((mx - spx + e.sheetScrollX) / (ss * zoom)) * ss
			e.hoverSY = ((my - sheetAreaY + e.sheetScrollY) / (ss * zoom)) * ss
			e.hoverValid = true

			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				if eac := e.getEquipmentAppearance(); eac != nil {
					eac.BlockOriginX = e.hoverSX
					eac.BlockOriginY = e.hoverSY
					if eac.SpriteSize == 0 {
						eac.SpriteSize = tileSize
					}
					e.setEquipmentAppearance(*eac)
				} else {
					ap := e.getAppearance()
					if ap == nil {
						ap = &Appearance{R: 255, G: 255, B: 255}
						if len(e.sheetKeys) > 0 {
							ap.Resource = e.sheetKeys[e.activeSheet]
						}
					}
					ap.SpriteX = e.hoverSX
					ap.SpriteY = e.hoverSY
					ap.SpriteSize = 0
					if s, ok := itemSheets[ap.Resource]; ok {
						ap.SpriteSize = s
					}
					e.setAppearance(*ap)
				}
			}
		}
	} else if mx >= spx && mx < spx+spw {
		e.hoverValid = false
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			btnX := spx + 4
			for i, k := range e.sheetKeys {
				w := len(k)*7 + 12
				if mx >= btnX && mx < btnX+w {
					e.activeSheet = i
					if eac := e.getEquipmentAppearance(); eac != nil {
						eac.Resource = k
						if eac.SpriteSize == 0 {
							eac.SpriteSize = tileSize
						}
						e.setEquipmentAppearance(*eac)
					} else if ap := e.getAppearance(); ap != nil {
						ap.Resource = k
						ap.SpriteSize = 0
						if s, ok := itemSheets[k]; ok {
							ap.SpriteSize = s
						}
						e.setAppearance(*ap)
					}
					e.sheetScrollX = 0
					e.sheetScrollY = 0
				}
				btnX += w + 4
			}
		}
	} else {
		e.hoverValid = false
	}

	ppx := screenW - panelProps
	if mx >= ppx {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && ptInR(mx, my, manageBtnRect()) && e.selectedBP != "" {
			e.overlay = "manage"
			e.manageAdding = false
			e.manageScroll = 0
		} else if e.hasEAC() {
			e.handleEACPropsInput(mx, my)
		} else {
			e.handleAppearancePropsInput(mx, my)
		}
	}

	return nil
}

// ── Appearance props input ────────────────────────────────────────────────────

func (e *Editor) handleAppearancePropsInput(mx, my int) {
	ppx := screenW - panelProps
	ap := e.getAppearance()
	if ap == nil {
		return
	}

	bouncesY := 160
	axisY := 178
	slidersY := 195
	presetsY := 320

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && my >= bouncesY && my < bouncesY+14 && mx >= ppx+8 {
		ap.Bounces = !ap.Bounces
		e.setAppearance(*ap)
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && my >= axisY && my < axisY+14 && mx >= ppx+8 {
		if ap.BounceAxis == "y" {
			ap.BounceAxis = ""
		} else {
			ap.BounceAxis = "y"
		}
		e.setAppearance(*ap)
	}

	sliderX := ppx + 80
	sliderW := panelProps - 90
	for ch := 0; ch < 3; ch++ {
		rowY := slidersY + ch*28
		pressed := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
		if pressed && my >= rowY && my < rowY+20 && mx >= sliderX {
			val := float64(mx-sliderX) / float64(sliderW)
			if val < 0 {
				val = 0
			}
			if val > 1 {
				val = 1
			}
			v := uint8(math.Round(val * 255))
			switch ch {
			case 0:
				ap.R = v
			case 1:
				ap.G = v
			case 2:
				ap.B = v
			}
			e.setAppearance(*ap)
		}
	}

	presets := []struct {
		label   string
		r, g, b uint8
	}{
		{"White", 255, 255, 255},
		{"Gray", 160, 160, 160},
		{"Red", 220, 80, 80},
		{"Green", 80, 220, 80},
		{"Blue", 80, 160, 255},
		{"Yellow", 240, 220, 80},
		{"Orange", 240, 140, 60},
		{"Cyan", 80, 220, 220},
		{"Purple", 180, 80, 220},
	}
	btnY := presetsY
	btnX := ppx + 8
	for _, p := range presets {
		pw := len(p.label)*7 + 8
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && mx >= btnX && mx < btnX+pw && my >= btnY && my < btnY+20 {
			ap.R, ap.G, ap.B = p.r, p.g, p.b
			e.setAppearance(*ap)
		}
		btnX += pw + 4
		if btnX > screenW-8 {
			btnX = ppx + 8
			btnY += 24
		}
	}

	// Weapon/Armor are now edited in the Manage Components modal.
}

// ── EAC props input ───────────────────────────────────────────────────────────

func (e *Editor) handleEACPropsInput(mx, my int) {
	ppx := screenW - panelProps
	eac := e.getEquipmentAppearance()
	if eac == nil {
		return
	}
	changed := false

	// Coordinates mirror drawEACPropsPanel exactly.

	// SpriteSize +/- drawn at (ppx+100, 162)
	ss := eac.SpriteSize
	if ss <= 0 {
		ss = tileSize
	}
	if newSS := e.clickPlusMinus(mx, my, ppx+100, 162, ss, 8, 64); newSS != ss {
		eac.SpriteSize = newSS
		changed = true
	}

	// AnimationFrames +/- drawn at (ppx+100, 178)
	af := eac.AnimationFrames
	if af <= 0 {
		af = 1
	}
	if newAF := e.clickPlusMinus(mx, my, ppx+100, 178, af, 1, 8); newAF != af {
		eac.AnimationFrames = newAF
		changed = true
	}

	// DefaultWeaponColumn +/- drawn at (ppx+168, 194)
	if newDWC := e.clickPlusMinus(mx, my, ppx+168, 194, eac.DefaultWeaponColumn, 0, 99); newDWC != eac.DefaultWeaponColumn {
		eac.DefaultWeaponColumn = newDWC
		changed = true
	}

	// DefaultArmorRow +/- drawn at (ppx+108, 210)
	if newDAR := e.clickPlusMinus(mx, my, ppx+108, 210, eac.DefaultArmorRow, 0, 99); newDAR != eac.DefaultArmorRow {
		eac.DefaultArmorRow = newDAR
		changed = true
	}

	if changed {
		e.setEquipmentAppearance(*eac)
		eac = e.getEquipmentAppearance()
	}

	// WeaponColumns — entries start at y=250, each 16px tall
	y := 250
	keys := sortedStringKeys(eac.WeaponColumns)
	for _, k := range keys {
		col := eac.WeaponColumns[k]
		// [x] delete drawn at (ppx+panelProps-20, y-1)
		delX := ppx + panelProps - 20
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && mx >= delX && mx < delX+14 && my >= y-1 && my < y+13 {
			delete(eac.WeaponColumns, k)
			e.setEquipmentAppearance(*eac)
			return
		}
		// col +/- drawn at (ppx+118, y)
		if newCol := e.clickPlusMinus(mx, my, ppx+118, y, col, 0, 99); newCol != col {
			eac.WeaponColumns[k] = newCol
			e.setEquipmentAppearance(*eac)
			return
		}
		y += 16
	}

	// Add weapon form — drawn at addY = y+6
	addWeaponY := y + 6
	// Key field drawn at (ppx+8, addWeaponY, w=100)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && mx >= ppx+8 && mx < ppx+108 && my >= addWeaponY && my < addWeaponY+16 {
		e.focusInput("eac_weapon_key", "")
	}
	// Pending col +/- drawn at (ppx+140, addWeaponY+2)
	if newPendCol := e.clickPlusMinus(mx, my, ppx+140, addWeaponY+2, e.eacNewWeaponCol, 0, 99); newPendCol != e.eacNewWeaponCol {
		e.eacNewWeaponCol = newPendCol
	}
	// [Add] drawn at (ppx+panelProps-42, addWeaponY, w=38)
	addBtnX := ppx + panelProps - 42
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && mx >= addBtnX && mx < addBtnX+38 && my >= addWeaponY && my < addWeaponY+16 {
		if e.textInputTarget == "eac_weapon_key" && e.textInputValue != "" {
			if eac.WeaponColumns == nil {
				eac.WeaponColumns = make(map[string]int)
			}
			eac.WeaponColumns[e.textInputValue] = e.eacNewWeaponCol
			e.setEquipmentAppearance(*eac)
			e.textInputTarget = ""
			e.textInputValue = ""
		}
	}

	// ArmorRows — drawn at armorHeaderY+16 where armorHeaderY = (addWeaponY+30)+8
	armorY := addWeaponY + 30 + 8 + 16
	for i, row := range eac.ArmorRows {
		// Header line: Row +/- at (ppx+38, armorY), [x] at (ppx+panelProps-20, armorY-1)
		delX := ppx + panelProps - 20
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && mx >= delX && mx < delX+14 && my >= armorY-1 && my < armorY+13 {
			eac.ArmorRows = append(eac.ArmorRows[:i], eac.ArmorRows[i+1:]...)
			e.setEquipmentAppearance(*eac)
			if e.eacTagRowIdx >= len(eac.ArmorRows) {
				e.eacTagRowIdx = -1
			}
			return
		}
		if newRowNum := e.clickPlusMinus(mx, my, ppx+38, armorY, row.Row, 0, 99); newRowNum != row.Row {
			eac.ArmorRows[i].Row = newRowNum
			e.setEquipmentAppearance(*eac)
			return
		}
		armorY += 18

		// Tags line: chips start at tagX=ppx+16, chipW = len(tag)*8+18
		tagX := ppx + 16
		for ti, tag := range row.Tags {
			chipW := len(tag)*8 + 18
			delTagX := tagX + chipW - 13
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && mx >= delTagX && mx < delTagX+11 && my >= armorY-1 && my < armorY+13 {
				eac.ArmorRows[i].Tags = append(row.Tags[:ti], row.Tags[ti+1:]...)
				e.setEquipmentAppearance(*eac)
				return
			}
			tagX += chipW + 3
		}
		// [+] drawn at tagX, 14×14
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && mx >= tagX && mx < tagX+14 && my >= armorY-1 && my < armorY+13 {
			if e.eacTagRowIdx == i {
				e.eacTagRowIdx = -1
				e.textInputTarget = ""
				e.textInputValue = ""
			} else {
				e.eacTagRowIdx = i
				e.focusInput("eac_row_tag", "")
			}
		}
		armorY += 18

		if e.eacTagRowIdx == i {
			tfW := panelProps - 70
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && mx >= ppx+8 && mx < ppx+8+tfW && my >= armorY && my < armorY+16 {
				e.focusInput("eac_row_tag", e.textInputValue)
			}
			addTagX := ppx + 8 + tfW + 4
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && mx >= addTagX && mx < addTagX+44 && my >= armorY && my < armorY+16 {
				if e.textInputValue != "" {
					eac.ArmorRows[i].Tags = append(eac.ArmorRows[i].Tags, e.textInputValue)
					e.setEquipmentAppearance(*eac)
					e.textInputValue = ""
					e.textInputTarget = ""
					e.eacTagRowIdx = -1
				}
			}
			armorY += 22
		}
	}

	// Add armor row form — drawn at addRowY = armorY+6
	addRowY := armorY + 6
	// Row +/- drawn at (ppx+44, addRowY+2)
	if newPendRow := e.clickPlusMinus(mx, my, ppx+44, addRowY+2, e.eacNewArmorRowNum, 0, 99); newPendRow != e.eacNewArmorRowNum {
		e.eacNewArmorRowNum = newPendRow
	}
	// [Add Row] drawn at (ppx+panelProps-64, addRowY, w=60)
	addRowBtnX := ppx + panelProps - 64
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && mx >= addRowBtnX && mx < addRowBtnX+60 && my >= addRowY && my < addRowY+16 {
		eac.ArmorRows = append(eac.ArmorRows, ArmorRowData{Row: e.eacNewArmorRowNum})
		e.setEquipmentAppearance(*eac)
	}
}

// clickPlusMinus checks if [-] or [+] was clicked. Mirrors drawPlusMinus button positions.
func (e *Editor) clickPlusMinus(mx, my, x, y, val, min, max int) int {
	vw := len(fmt.Sprintf("%d", val))*7 + 2
	minX := x + vw
	plusX := x + vw + 18
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if mx >= minX && mx < minX+14 && my >= y-2 && my < y+12 && val > min {
			return val - 1
		}
		if mx >= plusX && mx < plusX+14 && my >= y-2 && my < y+12 && val < max {
			return val + 1
		}
	}
	return val
}

// ── Draw ──────────────────────────────────────────────────────────────────────

func (e *Editor) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{20, 22, 30, 255})
	e.drawList(screen)
	e.drawSheetPanel(screen)
	e.drawPropsPanel(screen)
	e.drawOverlay(screen)
	e.gui.Draw(screen) // file/script pickers render on top of overlays
}

const listTop = 54

// listToolbar returns the New/Dup/Del button rects (kept in one place so
// drawing and hit-testing stay in sync).
func listToolbar() []struct {
	label string
	x, w  int
	c     color.RGBA
} {
	return []struct {
		label string
		x, w  int
		c     color.RGBA
	}{
		{"New", 4, 58, color.RGBA{40, 90, 60, 255}},
		{"Dup", 66, 58, color.RGBA{50, 70, 110, 255}},
		{"Del", 128, 58, color.RGBA{90, 50, 50, 255}},
	}
}

func (e *Editor) drawList(screen *ebiten.Image) {
	drawRect(screen, 0, 0, panelList, screenH, color.RGBA{30, 32, 42, 255})
	mlge_text.Draw(screen, "Entity Blueprints", 14, 6, 10, color.RGBA{160, 200, 255, 255})
	drawRect(screen, panelList-58, 4, 52, 22, color.RGBA{50, 80, 120, 255})
	mlge_text.Draw(screen, "Open", 14, panelList-50, 10, color.RGBA{180, 210, 255, 255})

	for _, b := range listToolbar() {
		drawRect(screen, b.x, 30, b.w, 20, b.c)
		drawOutlineRect(screen, b.x, 30, b.w, 20, color.RGBA{120, 140, 170, 255})
		mlge_text.Draw(screen, b.label, 14, b.x+8, 35, color.RGBA{220, 235, 255, 255})
	}

	itemH := 22
	for i, name := range e.bpNames {
		y := listTop + i*itemH - e.listScrollY
		if y < listTop-2 || y > screenH {
			continue
		}
		bg := color.RGBA{30, 32, 42, 0}
		if name == e.selectedBP {
			bg = color.RGBA{60, 90, 140, 255}
		}
		drawRect(screen, 1, y, panelList-2, itemH-1, bg)

		drawn := false
		if eac := e.blueprintEAC(name); eac != nil && eac.Resource != "" {
			if sheet, ok := e.sheets[eac.Resource]; ok {
				e.drawSpriteSized(screen, sheet, eac.BlockOriginX, eac.BlockOriginY, eac.SpriteSize, 2, y, 20, 20, 255, 255, 255)
				drawn = true
			}
		}
		if !drawn {
			if ap := e.blueprintAppearance(name); ap != nil {
				if sheet, ok := e.sheets[ap.Resource]; ok {
					e.drawSpriteTinted(screen, sheet, ap.SpriteX, ap.SpriteY, 2, y, 20, 20, ap.R, ap.G, ap.B)
				}
			}
		}

		nameCol := color.RGBA{200, 220, 255, 255}
		prefix := ""
		if _, ok := e.blueprints[name]["EquipmentAppearance"]; ok {
			prefix = "⚙ "
			nameCol = color.RGBA{200, 240, 200, 255}
		}
		mlge_text.Draw(screen, prefix+name, 15, 26, y+5, nameCol)
	}
}

func (e *Editor) drawSheetPanel(screen *ebiten.Image) {
	spx := e.sheetPanelX()
	spw := e.sheetPanelW()
	drawRect(screen, spx, 0, spw, screenH, color.RGBA{18, 20, 28, 255})

	eac := e.getEquipmentAppearance()
	ap := e.getAppearance()

	activeResource := e.currentResource()
	btnX := spx + 4
	for i, k := range e.sheetKeys {
		w := len(k)*7 + 12
		isActive := k == activeResource || (activeResource == "" && i == e.activeSheet)
		bg := color.RGBA{40, 44, 58, 255}
		if isActive {
			bg = color.RGBA{70, 110, 170, 255}
		}
		drawRect(screen, btnX, 4, w, 22, bg)
		col := color.RGBA{180, 200, 230, 255}
		if isActive {
			col = color.RGBA{255, 255, 255, 255}
		}
		mlge_text.Draw(screen, k, 14, btnX+6, 10, col)
		btnX += w + 4
	}

	statusX := btnX + 8
	if e.dirty {
		mlge_text.Draw(screen, "*unsaved* (Ctrl+S)", 14, statusX, 10, color.RGBA{220, 220, 80, 255})
	}

	sheet := e.currentSheet()
	if sheet == nil {
		return
	}

	sw, sh := sheet.Bounds().Dx(), sheet.Bounds().Dy()
	areaH := screenH - sheetAreaY

	subScreen := screen.SubImage(image.Rect(spx, sheetAreaY, spx+spw, screenH)).(*ebiten.Image)
	e.op.GeoM.Reset()
	e.op.ColorScale.Reset()
	e.op.GeoM.Scale(float64(zoom), float64(zoom))
	e.op.GeoM.Translate(float64(spx-e.sheetScrollX), float64(sheetAreaY-e.sheetScrollY))
	subScreen.DrawImage(sheet, e.op)

	ss := e.currentSpriteSize()
	cellPx := ss * zoom
	gridCol := color.RGBA{60, 60, 80, 150}
	for gx := 0; gx*cellPx < spw+e.sheetScrollX; gx++ {
		px := spx + gx*cellPx - e.sheetScrollX
		if px >= spx && px < spx+spw {
			drawRect(screen, px, sheetAreaY, 1, areaH, gridCol)
		}
	}
	for gy := 0; gy*cellPx < areaH+e.sheetScrollY; gy++ {
		py := sheetAreaY + gy*cellPx - e.sheetScrollY
		if py >= sheetAreaY && py < screenH {
			drawRect(screen, spx, py, spw, 1, gridCol)
		}
	}

	// Highlight current position
	if eac != nil {
		// Block origin highlight (green)
		hx := spx + eac.BlockOriginX*zoom - e.sheetScrollX
		hy := sheetAreaY + eac.BlockOriginY*zoom - e.sheetScrollY
		hw := ss * zoom
		drawOutlineRect(screen, hx, hy, hw, hw, color.RGBA{80, 255, 120, 220})

		// Block extent highlight (dim green rect covering all rows x all weapon cols)
		if len(eac.WeaponColumns) > 0 || len(eac.ArmorRows) > 0 {
			maxCol := eac.DefaultWeaponColumn
			for _, col := range eac.WeaponColumns {
				if col > maxCol {
					maxCol = col
				}
			}
			maxRow := eac.DefaultArmorRow
			for _, row := range eac.ArmorRows {
				if row.Row > maxRow {
					maxRow = row.Row
				}
			}
			af := eac.AnimationFrames
			if af < 1 {
				af = 1
			}
			blockW := (maxCol + 1) * af * ss * zoom
			blockH := (maxRow + 1) * ss * zoom
			bx := spx + eac.BlockOriginX*zoom - e.sheetScrollX
			by := sheetAreaY + eac.BlockOriginY*zoom - e.sheetScrollY
			drawOutlineRect(screen, bx, by, blockW, blockH, color.RGBA{80, 200, 100, 80})
		}
	} else if ap != nil && ap.Resource != "" {
		if _, ok := e.sheets[ap.Resource]; ok {
			hx := spx + ap.SpriteX*zoom - e.sheetScrollX
			hy := sheetAreaY + ap.SpriteY*zoom - e.sheetScrollY
			hw := spriteSize(ap.Resource) * zoom
			drawOutlineRect(screen, hx, hy, hw, hw, color.RGBA{80, 220, 255, 220})
		}
	}

	if e.hoverValid {
		hx := spx + e.hoverSX*zoom - e.sheetScrollX
		hy := sheetAreaY + e.hoverSY*zoom - e.sheetScrollY
		hw := cellPx
		drawOutlineRect(screen, hx, hy, hw, hw, color.RGBA{255, 220, 60, 200})
		mlge_text.Draw(screen, fmt.Sprintf("(%d, %d)", e.hoverSX, e.hoverSY), 10, hx, hy-14, color.RGBA{255, 220, 60, 255})
	}

	dispW := sw * zoom
	dispH := sh * zoom
	if dispW > spw {
		barW := spw * spw / dispW
		barX := spx + e.sheetScrollX*spw/dispW
		drawRect(screen, spx, screenH-6, spw, 6, color.RGBA{30, 34, 48, 255})
		drawRect(screen, barX, screenH-6, barW, 6, color.RGBA{100, 120, 180, 200})
	}
	if dispH > areaH {
		barH := areaH * areaH / dispH
		barY := sheetAreaY + e.sheetScrollY*areaH/dispH
		drawRect(screen, spx+spw-6, sheetAreaY, 6, areaH, color.RGBA{30, 34, 48, 255})
		drawRect(screen, spx+spw-6, barY, 6, barH, color.RGBA{100, 120, 180, 200})
	}

	hint := "scroll=wheel  horiz=Shift+wheel or middle-drag  click=assign sprite"
	if eac != nil {
		hint = "scroll=wheel  horiz=Shift+wheel or middle-drag  click=set block origin"
	}
	mlge_text.Draw(screen, hint, 14, spx+4, screenH-16, color.RGBA{120, 140, 160, 255})
}

// ── Props panel ───────────────────────────────────────────────────────────────

func (e *Editor) drawPropsPanel(screen *ebiten.Image) {
	ppx := screenW - panelProps
	drawRect(screen, ppx, 0, panelProps, screenH, color.RGBA{28, 30, 40, 255})
	mlge_text.Draw(screen, "Properties", 14, ppx+8, 8, color.RGBA{160, 200, 255, 255})

	mb := manageBtnRect()
	e.drawOverlayButton(screen, mb.Min.X, mb.Min.Y, mb.Dx(), mb.Dy(), "Manage Components", color.RGBA{45, 75, 110, 255})

	if e.hasEAC() {
		e.drawEACPropsPanel(screen)
		return
	}

	ap := e.getAppearance()
	if ap == nil {
		mlge_text.Draw(screen, e.selectedBP, 15, ppx+8, 28, color.RGBA{240, 240, 160, 255})
		mlge_text.Draw(screen, "No Appearance component", 14, ppx+8, 44, color.RGBA{180, 130, 130, 255})
		mlge_text.Draw(screen, "Use \"Manage Components\" below to", 13, ppx+8, 64, color.RGBA{150, 170, 190, 255})
		mlge_text.Draw(screen, "add components or set scripts.", 13, ppx+8, 78, color.RGBA{150, 170, 190, 255})
		return
	}

	mlge_text.Draw(screen, e.selectedBP, 15, ppx+8, 28, color.RGBA{240, 240, 160, 255})

	previewSize := 72
	px1 := ppx + 20
	px2 := ppx + 20 + previewSize + 16
	py := 50

	sheet := e.currentSheet()
	if sheet != nil {
		drawRect(screen, px1-2, py-2, previewSize+4, previewSize+4, color.RGBA{40, 44, 58, 255})
		e.drawSpriteTinted(screen, sheet, ap.SpriteX, ap.SpriteY, px1, py, previewSize, previewSize, ap.R, ap.G, ap.B)
		mlge_text.Draw(screen, "frame 1", 15, px1, py+previewSize+2, color.RGBA{120, 140, 160, 255})

		bsx, bsy := ap.SpriteX, ap.SpriteY
		if ap.Bounces {
			if ap.BounceAxis == "y" {
				bsy += tileSize
			} else {
				bsx += tileSize
			}
		}
		drawRect(screen, px2-2, py-2, previewSize+4, previewSize+4, color.RGBA{40, 44, 58, 255})
		e.drawSpriteTinted(screen, sheet, bsx, bsy, px2, py, previewSize, previewSize, ap.R, ap.G, ap.B)
		mlge_text.Draw(screen, "frame 2", 15, px2, py+previewSize+2, color.RGBA{120, 140, 160, 255})

		pxa := px2 + previewSize + 16
		drawRect(screen, pxa-2, py-2, previewSize+4, previewSize+4, color.RGBA{50, 54, 70, 255})
		animSX, animSY := ap.SpriteX, ap.SpriteY
		if ap.Bounces && e.bounce {
			if ap.BounceAxis == "y" {
				animSY += tileSize
			} else {
				animSX += tileSize
			}
		}
		e.drawSpriteTinted(screen, sheet, animSX, animSY, pxa, py, previewSize, previewSize, ap.R, ap.G, ap.B)
		mlge_text.Draw(screen, "live", 15, pxa, py+previewSize+2, color.RGBA{180, 220, 180, 255})
	}

	mlge_text.Draw(screen, "Sprite:", 15, ppx+8, 130, color.RGBA{160, 180, 210, 255})
	mlge_text.Draw(screen, fmt.Sprintf("X=%d  Y=%d  Resource: %s", ap.SpriteX, ap.SpriteY, ap.Resource), 11, ppx+8, 144, color.RGBA{220, 220, 255, 255})

	bouncesY := 160
	bLabel := "[ ] Bounces"
	if ap.Bounces {
		bLabel = "[x] Bounces"
	}
	mlge_text.Draw(screen, bLabel, 15, ppx+8, bouncesY, color.RGBA{200, 230, 200, 255})

	axisLabel := "Axis: X (default)"
	if ap.BounceAxis == "y" {
		axisLabel = "Axis: Y"
	}
	mlge_text.Draw(screen, "[toggle] "+axisLabel, 15, ppx+8, 178, color.RGBA{200, 230, 200, 255})

	channels := []struct {
		label string
		val   uint8
	}{{"R", ap.R}, {"G", ap.G}, {"B", ap.B}}
	sliderX := ppx + 80
	sliderW := panelProps - 90
	chanCols := []color.RGBA{{220, 80, 80, 255}, {80, 220, 80, 255}, {80, 140, 255, 255}}
	slidersY := 195
	for i, ch := range channels {
		ry := slidersY + i*28
		mlge_text.Draw(screen, fmt.Sprintf("%s: %3d", ch.label, ch.val), 11, ppx+8, ry+4, chanCols[i])
		drawRect(screen, sliderX, ry+6, sliderW, 8, color.RGBA{40, 44, 58, 255})
		filled := int(float64(ch.val) / 255.0 * float64(sliderW))
		drawRect(screen, sliderX, ry+6, filled, 8, chanCols[i])
		drawRect(screen, sliderX+filled-2, ry+4, 4, 12, color.RGBA{240, 240, 240, 255})
	}

	swatchY := 280
	mlge_text.Draw(screen, "Color:", 15, ppx+8, swatchY, color.RGBA{160, 180, 210, 255})
	drawRect(screen, ppx+60, swatchY-2, 40, 20, color.RGBA{ap.R, ap.G, ap.B, 255})

	presetsY := 320
	mlge_text.Draw(screen, "Presets:", 14, ppx+8, presetsY-14, color.RGBA{140, 160, 180, 255})
	presets := []struct {
		label   string
		r, g, b uint8
	}{
		{"White", 255, 255, 255}, {"Gray", 160, 160, 160}, {"Red", 220, 80, 80},
		{"Green", 80, 220, 80}, {"Blue", 80, 160, 255}, {"Yellow", 240, 220, 80},
		{"Orange", 240, 140, 60}, {"Cyan", 80, 220, 220}, {"Purple", 180, 80, 220},
	}
	y := presetsY
	btnX := ppx + 8
	for _, p := range presets {
		pw := len(p.label)*7 + 8
		if btnX+pw > screenW-4 {
			btnX = ppx + 8
			y += 24
		}
		drawRect(screen, btnX, y, pw, 20, color.RGBA{p.r / 2, p.g / 2, p.b / 2, 255})
		drawOutlineRect(screen, btnX, y, pw, 20, color.RGBA{p.r, p.g, p.b, 200})
		mlge_text.Draw(screen, p.label, 15, btnX+4, y+5, color.RGBA{p.r, p.g, p.b, 255})
		btnX += pw + 4
	}

	// Gallery
	y += 34
	mlge_text.Draw(screen, "Other blueprints on this sheet:", 14, ppx+8, y, color.RGBA{140, 160, 180, 255})
	y += 14
	gx, gy := ppx+8, y
	for _, name := range e.bpNames {
		other := e.blueprintAppearance(name)
		if other == nil || other.Resource != ap.Resource || name == e.selectedBP {
			continue
		}
		if sheet != nil {
			drawRect(screen, gx-1, gy-1, tileSize+2, tileSize+2, color.RGBA{40, 44, 58, 255})
			e.drawSpriteTinted(screen, sheet, other.SpriteX, other.SpriteY, gx, gy, tileSize, tileSize, other.R, other.G, other.B)
		}
		cmx, cmy := ebiten.CursorPosition()
		if cmx >= gx && cmx < gx+tileSize && cmy >= gy && cmy < gy+tileSize {
			mlge_text.Draw(screen, name, 14, gx, gy-12, color.RGBA{255, 240, 160, 255})
		}
		gx += tileSize + 4
		if gx+tileSize > screenW-4 {
			gx = ppx + 8
			gy += tileSize + 4
		}
		if gy > screenH-20 {
			break
		}
	}

	_ = gy
}

func (e *Editor) drawEACPropsPanel(screen *ebiten.Image) {
	ppx := screenW - panelProps
	eac := e.getEquipmentAppearance()
	if eac == nil {
		return
	}

	mlge_text.Draw(screen, e.selectedBP, 15, ppx+8, 28, color.RGBA{200, 255, 200, 255})

	// Preview: frame 1 at block origin, frame 2 at block origin + spriteSize
	ss := eac.SpriteSize
	if ss <= 0 {
		ss = tileSize
	}
	af := eac.AnimationFrames
	if af < 1 {
		af = 1
	}
	previewSize := 64
	sheet := e.currentSheet()
	if sheet != nil {
		px1 := ppx + 8
		drawRect(screen, px1-1, 49, previewSize+2, previewSize+2, color.RGBA{40, 44, 58, 255})
		e.drawSpriteSized(screen, sheet, eac.BlockOriginX, eac.BlockOriginY, ss, px1, 50, previewSize, previewSize, 255, 255, 255)
		mlge_text.Draw(screen, "origin", 15, px1, 50+previewSize+2, color.RGBA{120, 140, 160, 255})

		if af > 1 {
			px2 := px1 + previewSize + 10
			drawRect(screen, px2-1, 49, previewSize+2, previewSize+2, color.RGBA{40, 44, 58, 255})
			e.drawSpriteSized(screen, sheet, eac.BlockOriginX+ss, eac.BlockOriginY, ss, px2, 50, previewSize, previewSize, 255, 255, 255)
			mlge_text.Draw(screen, "frame 2", 15, px2, 50+previewSize+2, color.RGBA{120, 140, 160, 255})
		}
	}

	labelCol := color.RGBA{160, 180, 210, 255}
	valCol := color.RGBA{220, 240, 255, 255}
	headerCol := color.RGBA{140, 220, 160, 255}

	mlge_text.Draw(screen, "Resource: "+eac.Resource, 14, ppx+8, 132, labelCol)
	mlge_text.Draw(screen, fmt.Sprintf("Origin: (%d, %d) — click sheet to set", eac.BlockOriginX, eac.BlockOriginY), 10, ppx+8, 146, valCol)

	// SpriteSize control
	mlge_text.Draw(screen, "SpriteSize:", 14, ppx+8, 162, labelCol)
	e.drawPlusMinus(screen, ppx+100, 162, ss)

	// AnimFrames control
	mlge_text.Draw(screen, "AnimFrames:", 14, ppx+8, 178, labelCol)
	e.drawPlusMinus(screen, ppx+100, 178, af)

	// DefaultWeaponColumn
	mlge_text.Draw(screen, "Def.WeaponCol:", 14, ppx+8, 194, labelCol)
	e.drawPlusMinus(screen, ppx+112, 194, eac.DefaultWeaponColumn)

	// DefaultArmorRow
	mlge_text.Draw(screen, "Def.ArmorRow:", 14, ppx+8, 210, labelCol)
	e.drawPlusMinus(screen, ppx+108, 210, eac.DefaultArmorRow)

	// Separator
	drawRect(screen, ppx+4, 228, panelProps-8, 1, color.RGBA{60, 80, 100, 255})

	// WeaponColumns
	mlge_text.Draw(screen, "── WeaponColumns ──", 14, ppx+8, 236, headerCol)
	y := 250
	keys := sortedStringKeys(eac.WeaponColumns)
	for _, k := range keys {
		col := eac.WeaponColumns[k]
		mlge_text.Draw(screen, k, 14, ppx+8, y, valCol)
		mlge_text.Draw(screen, "→", 14, ppx+100, y, labelCol)
		e.drawPlusMinus(screen, ppx+118, y, col)
		// [x] delete
		drawRect(screen, ppx+panelProps-20, y-1, 14, 14, color.RGBA{120, 40, 40, 255})
		mlge_text.Draw(screen, "x", 15, ppx+panelProps-16, y+1, color.RGBA{255, 140, 140, 255})
		y += 16
	}

	// Add weapon form
	addY := y + 6
	drawRect(screen, ppx+4, addY-4, panelProps-8, 1, color.RGBA{40, 50, 60, 200})
	// Key input
	keyFocused := e.textInputTarget == "eac_weapon_key"
	keyDisplay := e.textInputValue
	if !keyFocused {
		keyDisplay = "key..."
	}
	keyBg := color.RGBA{35, 38, 50, 255}
	if keyFocused {
		keyBg = color.RGBA{50, 60, 80, 255}
	}
	drawRect(screen, ppx+8, addY, 100, 16, keyBg)
	drawOutlineRect(screen, ppx+8, addY, 100, 16, color.RGBA{70, 90, 120, 200})
	keyText := keyDisplay
	if keyFocused {
		keyText += "|"
	}
	kCol := valCol
	if !keyFocused && keyDisplay == "key..." {
		kCol = color.RGBA{90, 100, 120, 255}
	}
	mlge_text.Draw(screen, keyText, 14, ppx+12, addY+3, kCol)
	mlge_text.Draw(screen, "col:", 15, ppx+112, addY+3, labelCol)
	e.drawPlusMinus(screen, ppx+140, addY+2, e.eacNewWeaponCol)
	// [Add]
	drawRect(screen, ppx+panelProps-42, addY, 38, 16, color.RGBA{40, 80, 50, 255})
	drawOutlineRect(screen, ppx+panelProps-42, addY, 38, 16, color.RGBA{80, 180, 100, 200})
	mlge_text.Draw(screen, "Add", 14, ppx+panelProps-34, addY+3, color.RGBA{140, 255, 160, 255})

	// Separator
	sep2Y := addY + 26
	drawRect(screen, ppx+4, sep2Y, panelProps-8, 1, color.RGBA{60, 80, 100, 255})

	// ArmorRows
	armorHeaderY := sep2Y + 8
	mlge_text.Draw(screen, "── ArmorRows ──", 14, ppx+8, armorHeaderY, headerCol)
	y = armorHeaderY + 16

	for i, row := range eac.ArmorRows {
		// Row header line: "Row N - + [x delete]"
		mlge_text.Draw(screen, "Row", 14, ppx+8, y, labelCol)
		e.drawPlusMinus(screen, ppx+38, y, row.Row)
		drawRect(screen, ppx+panelProps-20, y-1, 14, 14, color.RGBA{120, 40, 40, 255})
		mlge_text.Draw(screen, "x", 15, ppx+panelProps-16, y+1, color.RGBA{255, 140, 140, 255})
		y += 18

		// Tags line (indented)
		tagX := ppx + 16
		for _, tag := range row.Tags {
			chipW := len(tag)*8 + 18
			drawRect(screen, tagX, y-1, chipW, 16, color.RGBA{40, 60, 80, 255})
			drawOutlineRect(screen, tagX, y-1, chipW, 16, color.RGBA{80, 140, 180, 180})
			mlge_text.Draw(screen, tag, 12, tagX+4, y+2, color.RGBA{160, 220, 255, 255})
			drawRect(screen, tagX+chipW-13, y, 11, 12, color.RGBA{100, 30, 30, 200})
			mlge_text.Draw(screen, "x", 12, tagX+chipW-10, y+1, color.RGBA{255, 130, 130, 255})
			tagX += chipW + 3
		}
		// [+] add tag
		drawRect(screen, tagX, y-1, 14, 14, color.RGBA{30, 70, 40, 255})
		drawOutlineRect(screen, tagX, y-1, 14, 14, color.RGBA{80, 180, 100, 180})
		mlge_text.Draw(screen, "+", 14, tagX+3, y, color.RGBA{120, 255, 140, 255})
		y += 18

		// Tag input for this row (if open)
		if e.eacTagRowIdx == i {
			tfFocused := e.textInputTarget == "eac_row_tag"
			tfBg := color.RGBA{35, 38, 50, 255}
			if tfFocused {
				tfBg = color.RGBA{50, 60, 80, 255}
			}
			tfW := panelProps - 70
			drawRect(screen, ppx+8, y, tfW, 16, tfBg)
			drawOutlineRect(screen, ppx+8, y, tfW, 16, color.RGBA{70, 90, 120, 200})
			tfText := e.textInputValue
			if tfFocused {
				tfText += "|"
			} else if tfText == "" {
				tfText = "tag name..."
			}
			tfCol := valCol
			if !tfFocused && e.textInputValue == "" {
				tfCol = color.RGBA{90, 100, 120, 255}
			}
			mlge_text.Draw(screen, tfText, 14, ppx+12, y+3, tfCol)
			addTagX := ppx + 8 + tfW + 4
			drawRect(screen, addTagX, y, 44, 16, color.RGBA{40, 80, 50, 255})
			drawOutlineRect(screen, addTagX, y, 44, 16, color.RGBA{80, 180, 100, 200})
			mlge_text.Draw(screen, "Add", 15, addTagX+8, y+3, color.RGBA{140, 255, 160, 255})
			y += 22
		}
	}

	// Add armor row form
	addRowY := y + 6
	drawRect(screen, ppx+4, addRowY-4, panelProps-8, 1, color.RGBA{40, 50, 60, 200})
	mlge_text.Draw(screen, "Row:", 14, ppx+8, addRowY+2, labelCol)
	e.drawPlusMinus(screen, ppx+44, addRowY+2, e.eacNewArmorRowNum)
	addRowBtnX := ppx + panelProps - 64
	drawRect(screen, addRowBtnX, addRowY, 60, 16, color.RGBA{40, 80, 50, 255})
	drawOutlineRect(screen, addRowBtnX, addRowY, 60, 16, color.RGBA{80, 180, 100, 200})
	mlge_text.Draw(screen, "Add Row", 15, addRowBtnX+6, addRowY+3, color.RGBA{140, 255, 160, 255})
}

// drawPlusMinus draws the value + [-][+] buttons at (x,y).
func (e *Editor) drawPlusMinus(screen *ebiten.Image, x, y, val int) {
	valStr := fmt.Sprintf("%d", val)
	mlge_text.Draw(screen, valStr, 14, x, y, color.RGBA{220, 240, 255, 255})
	vw := len(valStr)*7 + 2
	drawRect(screen, x+vw, y-2, 14, 14, color.RGBA{45, 50, 65, 255})
	mlge_text.Draw(screen, "-", 14, x+vw+3, y, color.RGBA{200, 200, 200, 255})
	drawRect(screen, x+vw+18, y-2, 14, 14, color.RGBA{45, 50, 65, 255})
	mlge_text.Draw(screen, "+", 14, x+vw+21, y, color.RGBA{200, 200, 200, 255})
}

func ptInR(mx, my int, r image.Rectangle) bool {
	return mx >= r.Min.X && mx < r.Max.X && my >= r.Min.Y && my < r.Max.Y
}

// ── Manage Components modal ───────────────────────────────────────────────────

type manageGeom struct {
	panel    image.Rectangle
	addBtn   image.Rectangle
	closeBtn image.Rectangle
	rows     []compRowGeom // current components (list) or add candidates
	editorX  int
	editorY  int
	editorW  int
	editorB  int
}

type compRowGeom struct {
	name string
	row  image.Rectangle
	del  image.Rectangle
}

func manageBtnRect() image.Rectangle {
	ppx := screenW - panelProps
	return image.Rect(ppx+8, screenH-32, screenW-8, screenH-8)
}

func manageModalRect() image.Rectangle {
	w, h := 940, 660
	if w > screenW-40 {
		w = screenW - 40
	}
	if h > screenH-40 {
		h = screenH - 40
	}
	x := (screenW - w) / 2
	y := (screenH - h) / 2
	return image.Rect(x, y, x+w, y+h)
}

func (e *Editor) computeManage() manageGeom {
	g := manageGeom{}
	p := manageModalRect()
	g.panel = p
	listX := p.Min.X + 16
	listW := 250
	g.addBtn = image.Rect(listX, p.Min.Y+44, listX+listW, p.Min.Y+44+22)
	g.closeBtn = image.Rect(p.Max.X-90, p.Min.Y+10, p.Max.X-14, p.Min.Y+10+22)

	cy := p.Min.Y + 78
	rowH := 22
	var names []string
	if e.manageAdding {
		names = e.availableComponents(e.blueprints[e.selectedBP])
	} else {
		names = sortedKeys(e.blueprints[e.selectedBP])
	}
	for i, k := range names {
		ry := cy + i*rowH - e.manageScroll
		if ry < p.Min.Y+70 || ry > p.Max.Y-16 {
			g.rows = append(g.rows, compRowGeom{name: k, row: image.Rect(-1, -1, -1, -1)})
			continue
		}
		row := image.Rect(listX, ry, listX+listW-20, ry+rowH-2)
		del := image.Rect(listX+listW-18, ry, listX+listW, ry+rowH-2)
		g.rows = append(g.rows, compRowGeom{name: k, row: row, del: del})
	}

	g.editorX = listX + listW + 20
	g.editorY = p.Min.Y + 70
	g.editorW = p.Max.X - g.editorX - 16
	g.editorB = p.Max.Y - 16
	return g
}

func (e *Editor) drawManageModal(screen *ebiten.Image) {
	g := e.computeManage()
	e.manageGeomCache = &g
	p := g.panel

	drawRect(screen, 0, 0, screenW, screenH, color.RGBA{0, 0, 0, 150})
	drawRect(screen, p.Min.X, p.Min.Y, p.Dx(), p.Dy(), color.RGBA{32, 36, 48, 255})
	drawOutlineRect(screen, p.Min.X, p.Min.Y, p.Dx(), p.Dy(), color.RGBA{90, 120, 170, 255})
	mlge_text.Draw(screen, "Manage Components — "+e.selectedBP, 16, p.Min.X+16, p.Min.Y+14, color.RGBA{200, 220, 255, 255})

	e.drawOverlayButton(screen, g.closeBtn.Min.X, g.closeBtn.Min.Y, g.closeBtn.Dx(), g.closeBtn.Dy(), "Close", color.RGBA{60, 70, 90, 255})

	addLabel := "+ Add Component"
	addCol := color.RGBA{40, 80, 50, 255}
	if e.manageAdding {
		addLabel = "‹ Back to list"
		addCol = color.RGBA{70, 60, 40, 255}
	}
	e.drawOverlayButton(screen, g.addBtn.Min.X, g.addBtn.Min.Y, g.addBtn.Dx(), g.addBtn.Dy(), addLabel, addCol)

	// Vertical divider
	divX := g.editorX - 10
	drawRect(screen, divX, p.Min.Y+40, 1, p.Dy()-56, color.RGBA{60, 70, 90, 255})

	for _, r := range g.rows {
		if r.row.Min.X < 0 {
			continue
		}
		if !e.manageAdding && r.name == e.selectedComponent {
			drawRect(screen, r.row.Min.X, r.row.Min.Y, r.row.Dx(), r.row.Dy(), color.RGBA{55, 75, 110, 255})
		}
		col := color.RGBA{205, 222, 240, 255}
		if typedFormComponents[r.name] {
			col = color.RGBA{200, 245, 205, 255}
		}
		mlge_text.Draw(screen, r.name, 14, r.row.Min.X+6, r.row.Min.Y+4, col)
		if !e.manageAdding {
			drawRect(screen, r.del.Min.X, r.del.Min.Y, r.del.Dx(), r.del.Dy(), color.RGBA{100, 40, 40, 255})
			mlge_text.Draw(screen, "x", 14, r.del.Min.X+4, r.del.Min.Y+4, color.RGBA{255, 150, 150, 255})
		}
	}

	if e.manageAdding {
		mlge_text.Draw(screen, "Click a component to add it", 13, g.editorX, g.editorY, color.RGBA{150, 170, 190, 255})
		return
	}
	if e.selectedComponent != "" {
		if _, ok := e.blueprints[e.selectedBP][e.selectedComponent]; ok {
			e.drawComponentEditor(screen, g.editorX, g.editorY+14, g.editorW, g.editorB)
		}
	} else {
		mlge_text.Draw(screen, "Select a component on the left", 13, g.editorX, g.editorY, color.RGBA{150, 170, 190, 255})
	}
}

func (e *Editor) handleManageInput(mx, my int) {
	g := e.manageGeomCache
	if g == nil {
		return
	}
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)

	if !ptInR(mx, my, g.panel) {
		// scroll list when hovering the list column
		_, dy := ebiten.Wheel()
		e.manageScroll -= int(dy) * 22
		if e.manageScroll < 0 {
			e.manageScroll = 0
		}
		if click {
			e.overlay = ""
			e.manageAdding = false
		}
		return
	}
	_, dy := ebiten.Wheel()
	e.manageScroll -= int(dy) * 22
	if e.manageScroll < 0 {
		e.manageScroll = 0
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		e.overlay = ""
		e.manageAdding = false
		return
	}

	if click && ptInR(mx, my, g.closeBtn) {
		e.overlay = ""
		e.manageAdding = false
		return
	}
	if click && ptInR(mx, my, g.addBtn) {
		e.manageAdding = !e.manageAdding
		e.manageScroll = 0
		return
	}

	if e.manageAdding {
		if click {
			for _, r := range g.rows {
				if r.row.Min.X >= 0 && ptInR(mx, my, r.row) {
					bp := e.blueprints[e.selectedBP]
					bp[r.name] = json.RawMessage(defaultJSONFor(r.name))
					e.dirty = true
					e.selectedComponent = r.name
					e.jsonEditTarget = ""
					if r.name == "Appearance" || r.name == "EquipmentAppearance" {
						e.reloadSheets()
					}
					e.manageAdding = false
					e.manageScroll = 0
					return
				}
			}
		}
		return
	}

	if click {
		for _, r := range g.rows {
			if r.row.Min.X < 0 {
				continue
			}
			if ptInR(mx, my, r.del) {
				name := r.name
				e.confirm("Remove component \""+name+"\"?", func() {
					delete(e.blueprints[e.selectedBP], name)
					e.dirty = true
					if e.selectedComponent == name {
						e.selectedComponent = ""
					}
					if e.jsonEditTarget == name {
						e.jsonEditTarget = ""
					}
				})
				return
			}
			if ptInR(mx, my, r.row) {
				if e.textInputTarget != "" {
					e.commitTextInput()
				}
				e.selectedComponent = r.name
				e.jsonEditTarget = ""
				e.textInputTarget = ""
				return
			}
		}
	}

	if e.selectedComponent != "" {
		e.handleComponentEditorInput(mx, my, click)
	}
}

// ── Component editor (typed forms + JSON fallback) ────────────────────────────

type compField struct {
	label string
	kind  string // "str" | "script" | "strlist" | "int" | "bool"
	get   func() string
	set   func(string)
	geti  func() int
	seti  func(int)
	getb  func() bool
	setb  func(bool)
}

type compFieldGeom struct {
	f      compField
	box    image.Rectangle // str/script input box
	browse image.Rectangle // script Browse button
	pm     image.Rectangle // int +/- anchor (x,y at Min)
}

type compEditorGeometry struct {
	isJSON    bool
	fields    []compFieldGeom
	jsonBox   image.Rectangle
	applyBtn  image.Rectangle
	revertBtn image.Rectangle
}

// fieldsFor returns the typed-form field list for a component, or nil to use
// the raw-JSON fallback editor.
func (e *Editor) fieldsFor(name string) []compField {
	switch name {
	case "Script":
		return []compField{{label: "on_turn", kind: "script",
			get: func() string {
				s := e.getScript()
				if s == nil {
					return ""
				}
				return s.OnTurn
			},
			set: func(v string) {
				s := e.getScript()
				if s == nil {
					s = &ScriptData{}
				}
				s.OnTurn = v
				e.setScript(s)
			}}}
	case "ScriptedAI":
		return []compField{{label: "script", kind: "script",
			get: func() string {
				s := e.getScriptedAI()
				if s == nil {
					return ""
				}
				return s.Script
			},
			set: func(v string) {
				s := e.getScriptedAI()
				if s == nil {
					s = &ScriptedAIData{}
				}
				s.Script = v
				e.setScriptedAI(s)
			}}}
	case "Description":
		d := func() *DescriptionData {
			x := e.getDescription()
			if x == nil {
				x = &DescriptionData{}
			}
			return x
		}
		return []compField{
			{label: "Name", kind: "str", get: func() string { return d().Name }, set: func(v string) { x := d(); x.Name = v; e.setDescription(x) }},
			{label: "Faction", kind: "str", get: func() string { return d().Faction }, set: func(v string) { x := d(); x.Faction = v; e.setDescription(x) }},
			{label: "ID", kind: "str", get: func() string { return d().ID }, set: func(v string) { x := d(); x.ID = v; e.setDescription(x) }},
			{label: "LongDescription", kind: "str", get: func() string { return d().LongDescription }, set: func(v string) { x := d(); x.LongDescription = v; e.setDescription(x) }},
		}
	case "Health":
		h := func() *HealthData {
			x := e.getHealth()
			if x == nil {
				x = &HealthData{}
			}
			return x
		}
		return []compField{
			{label: "MaxHealth", kind: "int", geti: func() int { return h().MaxHealth }, seti: func(v int) { x := h(); x.MaxHealth = v; e.setHealth(x) }},
			{label: "Health", kind: "int", geti: func() int { return h().Health }, seti: func(v int) { x := h(); x.Health = v; e.setHealth(x) }},
			{label: "Energy", kind: "int", geti: func() int { return h().Energy }, seti: func(v int) { x := h(); x.Energy = v; e.setHealth(x) }},
		}
	case "Stats":
		s := func() *StatsData {
			x := e.getStats()
			if x == nil {
				x = &StatsData{}
			}
			return x
		}
		mk := func(label string, gp func(*StatsData) *int) compField {
			return compField{label: label, kind: "int",
				geti: func() int { return *gp(s()) },
				seti: func(v int) { x := s(); *gp(x) = v; e.setStats(x) }}
		}
		return []compField{
			mk("AC", func(x *StatsData) *int { return &x.AC }),
			mk("Str", func(x *StatsData) *int { return &x.Str }),
			mk("Dex", func(x *StatsData) *int { return &x.Dex }),
			mk("Con", func(x *StatsData) *int { return &x.Con }),
			mk("Int", func(x *StatsData) *int { return &x.Int }),
			mk("Wis", func(x *StatsData) *int { return &x.Wis }),
			mk("MeleeAtkBonus", func(x *StatsData) *int { return &x.MeleeAttackBonus }),
			mk("RangedAtkBonus", func(x *StatsData) *int { return &x.RangedAttackBonus }),
		}
	case "Weapon":
		w := func() *WeaponData {
			x := e.getWeapon()
			if x == nil {
				x = &WeaponData{}
			}
			return x
		}
		return []compField{
			{label: "AttackBonus", kind: "int", geti: func() int { return w().AttackBonus }, seti: func(v int) { x := w(); x.AttackBonus = v; e.setWeapon(x) }},
			{label: "AttackDice", kind: "str", get: func() string { return w().AttackDice }, set: func(v string) { x := w(); x.AttackDice = v; e.setWeapon(x) }},
			{label: "DamageType", kind: "str", get: func() string { return w().DamageType }, set: func(v string) { x := w(); x.DamageType = v; e.setWeapon(x) }},
			{label: "Range", kind: "int", geti: func() int { return w().Range }, seti: func(v int) { x := w(); x.Range = v; e.setWeapon(x) }},
			{label: "Ranged", kind: "bool", getb: func() bool { return w().Ranged }, setb: func(v bool) { x := w(); x.Ranged = v; e.setWeapon(x) }},
			{label: "ProjectileResource", kind: "str", get: func() string { return w().ProjectileResource }, set: func(v string) { x := w(); x.ProjectileResource = v; e.setWeapon(x) }},
			{label: "ProjectileX", kind: "int", geti: func() int { return w().ProjectileX }, seti: func(v int) { x := w(); x.ProjectileX = v; e.setWeapon(x) }},
			{label: "ProjectileY", kind: "int", geti: func() int { return w().ProjectileY }, seti: func(v int) { x := w(); x.ProjectileY = v; e.setWeapon(x) }},
			{label: "Display", kind: "str", get: func() string { return w().Display }, set: func(v string) { x := w(); x.Display = v; e.setWeapon(x) }},
		}
	case "Armor":
		a := func() *ArmorData {
			x := e.getArmor()
			if x == nil {
				x = &ArmorData{}
			}
			return x
		}
		return []compField{
			{label: "DefenseBonus", kind: "int", geti: func() int { return a().DefenseBonus }, seti: func(v int) { x := a(); x.DefenseBonus = v; e.setArmor(x) }},
			{label: "StoppingPower", kind: "int", geti: func() int { return a().StoppingPower }, seti: func(v int) { x := a(); x.StoppingPower = v; e.setArmor(x) }},
			{label: "Tags", kind: "strlist", get: func() string { return joinList(a().Tags) }, set: func(v string) { x := a(); x.Tags = splitList(v); e.setArmor(x) }},
			{label: "Resistances", kind: "strlist", get: func() string { return joinList(a().Resistances) }, set: func(v string) { x := a(); x.Resistances = splitList(v); e.setArmor(x) }},
		}
	}
	return nil
}

func joinList(s []string) string { return strings.Join(s, ", ") }

func splitList(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// computeCompEditor lays out the editor for e.selectedComponent inside the
// rectangle (x, y) .. (x+w, bottom). Shared by draw and input.
func (e *Editor) computeCompEditor(x, y, w, bottom int) compEditorGeometry {
	g := compEditorGeometry{}
	fields := e.fieldsFor(e.selectedComponent)
	if fields == nil {
		g.isJSON = true
		boxH := bottom - (y + 16) - 40
		if boxH < 60 {
			boxH = 60
		}
		g.jsonBox = image.Rect(x+8, y+16, x+w-8, y+16+boxH)
		g.applyBtn = image.Rect(x+8, g.jsonBox.Max.Y+8, x+8+70, g.jsonBox.Max.Y+8+22)
		g.revertBtn = image.Rect(x+86, g.jsonBox.Max.Y+8, x+86+70, g.jsonBox.Max.Y+8+22)
		return g
	}
	cy := y + 6
	for _, f := range fields {
		fg := compFieldGeom{f: f}
		switch f.kind {
		case "int":
			fg.pm = image.Rect(x+150, cy, x+150, cy) // anchor only
			cy += 20
		case "bool":
			fg.pm = image.Rect(x+150, cy, x+150, cy) // label baseline
			fg.box = image.Rect(x+150, cy-10, x+150+16, cy+6)
			cy += 20
		default: // str, script, strlist
			boxW := w - 20
			if f.kind == "script" {
				boxW = w - 20 - 60
			}
			fg.box = image.Rect(x+8, cy+12, x+8+boxW, cy+12+16)
			if f.kind == "script" {
				fg.browse = image.Rect(fg.box.Max.X+4, cy+12, fg.box.Max.X+4+52, cy+12+16)
			}
			cy += 34
		}
		g.fields = append(g.fields, fg)
	}
	return g
}

func (e *Editor) drawComponentEditor(screen *ebiten.Image, x, y, w, bottom int) {
	g := e.computeCompEditor(x, y, w, bottom)
	e.compEditorGeom = &g
	labelCol := color.RGBA{160, 180, 210, 255}
	valCol := color.RGBA{225, 240, 255, 255}
	ppx := x

	mlge_text.Draw(screen, "▸ "+e.selectedComponent, 14, ppx+8, y-12, color.RGBA{200, 235, 205, 255})

	if g.isJSON {
		editing := e.jsonEditTarget == e.selectedComponent
		body := e.jsonEdit
		if !editing {
			body = e.prettyComponentJSON(e.selectedComponent)
		}
		b := g.jsonBox
		drawRect(screen, b.Min.X, b.Min.Y, b.Dx(), b.Dy(), color.RGBA{24, 26, 34, 255})
		bord := color.RGBA{70, 90, 120, 200}
		if editing {
			bord = color.RGBA{120, 170, 110, 220}
		}
		drawOutlineRect(screen, b.Min.X, b.Min.Y, b.Dx(), b.Dy(), bord)
		ty := b.Min.Y + 4
		for _, line := range strings.Split(body, "\n") {
			if ty > b.Max.Y-12 {
				break
			}
			mlge_text.Draw(screen, line, 12, b.Min.X+4, ty, valCol)
			ty += 13
		}
		if !editing {
			mlge_text.Draw(screen, "(click to edit)", 12, b.Min.X+4, b.Max.Y-14, color.RGBA{110, 130, 150, 255})
		}
		e.drawOverlayButton(screen, g.applyBtn.Min.X, g.applyBtn.Min.Y, g.applyBtn.Dx(), g.applyBtn.Dy(), "Apply", color.RGBA{40, 90, 60, 255})
		e.drawOverlayButton(screen, g.revertBtn.Min.X, g.revertBtn.Min.Y, g.revertBtn.Dx(), g.revertBtn.Dy(), "Revert", color.RGBA{70, 55, 55, 255})
		if e.jsonEditErr != "" {
			mlge_text.Draw(screen, e.jsonEditErr, 12, g.applyBtn.Min.X, g.applyBtn.Max.Y+6, color.RGBA{255, 140, 140, 255})
		}
		return
	}

	for _, fg := range g.fields {
		f := fg.f
		if f.kind == "int" {
			ly := fg.pm.Min.Y
			mlge_text.Draw(screen, f.label+":", 12, ppx+8, ly, labelCol)
			e.drawPlusMinus(screen, fg.pm.Min.X, ly, f.geti())
			continue
		}
		if f.kind == "bool" {
			ly := fg.pm.Min.Y
			mlge_text.Draw(screen, f.label+":", 12, ppx+8, ly, labelCol)
			drawRect(screen, fg.box.Min.X, fg.box.Min.Y, fg.box.Dx(), fg.box.Dy(), color.RGBA{40, 44, 58, 255})
			drawOutlineRect(screen, fg.box.Min.X, fg.box.Min.Y, fg.box.Dx(), fg.box.Dy(), color.RGBA{70, 90, 120, 200})
			mark := " "
			if f.getb() {
				mark = "x"
			}
			mlge_text.Draw(screen, mark, 12, fg.box.Min.X+5, fg.box.Min.Y+3, valCol)
			continue
		}
		ly := fg.box.Min.Y - 12
		mlge_text.Draw(screen, f.label+":", 12, ppx+8, ly, labelCol)
		focused := e.textInputTarget == e.selectedComponent+"."+f.label
		txt := f.get()
		if focused {
			txt = e.textInputValue + "|"
		}
		bg := color.RGBA{35, 38, 50, 255}
		if focused {
			bg = color.RGBA{50, 60, 80, 255}
		}
		drawRect(screen, fg.box.Min.X, fg.box.Min.Y, fg.box.Dx(), fg.box.Dy(), bg)
		drawOutlineRect(screen, fg.box.Min.X, fg.box.Min.Y, fg.box.Dx(), fg.box.Dy(), color.RGBA{70, 90, 120, 200})
		mlge_text.Draw(screen, txt, 12, fg.box.Min.X+4, fg.box.Min.Y+3, valCol)
		if f.kind == "script" {
			e.drawOverlayButton(screen, fg.browse.Min.X, fg.browse.Min.Y, fg.browse.Dx(), fg.browse.Dy(), "Browse", color.RGBA{50, 70, 110, 255})
		}
	}
}

func (e *Editor) handleComponentEditorInput(mx, my int, click bool) {
	g := e.compEditorGeom
	if g == nil {
		return
	}
	if g.isJSON {
		if !click {
			return
		}
		if ptInR(mx, my, g.jsonBox) {
			if e.jsonEditTarget != e.selectedComponent {
				e.jsonEdit = e.prettyComponentJSON(e.selectedComponent)
				e.jsonEditTarget = e.selectedComponent
				e.jsonEditErr = ""
			}
			return
		}
		if ptInR(mx, my, g.applyBtn) && e.jsonEditTarget == e.selectedComponent {
			var probe any
			if err := json.Unmarshal([]byte(e.jsonEdit), &probe); err != nil {
				e.jsonEditErr = "Invalid JSON: " + err.Error()
				return
			}
			e.blueprints[e.selectedBP][e.selectedComponent] = json.RawMessage(e.jsonEdit)
			e.dirty = true
			e.jsonEditErr = ""
			e.jsonEditTarget = ""
			return
		}
		if ptInR(mx, my, g.revertBtn) {
			e.jsonEditTarget = ""
			e.jsonEditErr = ""
		}
		return
	}

	for _, fg := range g.fields {
		f := fg.f
		if f.kind == "int" {
			if nv := e.clickPlusMinus(mx, my, fg.pm.Min.X, fg.pm.Min.Y, f.geti(), -999, 9999); nv != f.geti() {
				f.seti(nv)
				return
			}
			continue
		}
		if !click {
			continue
		}
		if f.kind == "bool" {
			if ptInR(mx, my, fg.box) {
				f.setb(!f.getb())
				return
			}
			continue
		}
		if f.kind == "script" && ptInR(mx, my, fg.browse) {
			e.scriptModal.SetVisible(true)
			return
		}
		if ptInR(mx, my, fg.box) {
			e.focusInput(e.selectedComponent+"."+f.label, f.get())
			return
		}
	}
}

// prettyComponentJSON returns indented JSON for a component (or "{}").
func (e *Editor) prettyComponentJSON(name string) string {
	bp, ok := e.blueprints[e.selectedBP]
	if !ok {
		return "{}"
	}
	raw, ok := bp[name]
	if !ok {
		return "{}"
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return string(raw)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(out)
}

// updateJSONEditInput feeds keyboard input into the raw-JSON buffer when it
// is the active edit target (multi-line; Enter inserts a newline).
func (e *Editor) updateJSONEditInput() {
	if e.jsonEditTarget == "" || e.jsonEditTarget != e.selectedComponent {
		return
	}
	for _, ch := range ebiten.AppendInputChars(nil) {
		e.jsonEdit += string(ch)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		e.jsonEdit += "\n"
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		e.jsonEdit += "  "
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(e.jsonEdit) > 0 {
		r := []rune(e.jsonEdit)
		e.jsonEdit = string(r[:len(r)-1])
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		e.jsonEditTarget = ""
		e.jsonEditErr = ""
	}
}

// ── Sprite drawing helpers ────────────────────────────────────────────────────

func (e *Editor) drawSpriteTinted(dst, sheet *ebiten.Image, sx, sy, dx, dy, dw, dh int, r, g, b uint8) {
	sheetKey := ""
	for k, img := range e.sheets {
		if img == sheet {
			sheetKey = k
			break
		}
	}
	ss := spriteSize(sheetKey)
	e.drawSpriteSized(dst, sheet, sx, sy, ss, dx, dy, dw, dh, r, g, b)
}

func (e *Editor) drawSpriteSized(dst, sheet *ebiten.Image, sx, sy, ss, dx, dy, dw, dh int, r, g, b uint8) {
	if ss <= 0 {
		ss = tileSize
	}
	bounds := sheet.Bounds()
	if sx+ss > bounds.Dx() || sy+ss > bounds.Dy() || sx < 0 || sy < 0 {
		return
	}
	src := sheet.SubImage(image.Rect(sx, sy, sx+ss, sy+ss)).(*ebiten.Image)
	e.op.GeoM.Reset()
	e.op.ColorScale.Reset()
	e.op.ColorScale.SetR(float32(r) / 255.0)
	e.op.ColorScale.SetG(float32(g) / 255.0)
	e.op.ColorScale.SetB(float32(b) / 255.0)
	e.op.GeoM.Scale(float64(dw)/float64(ss), float64(dh)/float64(ss))
	e.op.GeoM.Translate(float64(dx), float64(dy))
	dst.DrawImage(src, e.op)
}

// ── Generic helpers ───────────────────────────────────────────────────────────

var rectImg = ebiten.NewImage(1, 1)

func drawRect(dst *ebiten.Image, x, y, w, h int, c color.RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	rectImg.Fill(c)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(w), float64(h))
	op.GeoM.Translate(float64(x), float64(y))
	dst.DrawImage(rectImg, op)
}

func drawOutlineRect(dst *ebiten.Image, x, y, w, h int, c color.RGBA) {
	drawRect(dst, x, y, w, 2, c)
	drawRect(dst, x, y+h-2, w, 2, c)
	drawRect(dst, x, y, 2, h, c)
	drawRect(dst, x+w-2, y, 2, h, c)
}

func sortedKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedStringKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (e *Editor) Layout(ow, oh int) (int, int) { return screenW, screenH }

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	ed, err := NewEditor("data/blueprints/entities/colonists.json", "data/assets.json")
	if err != nil {
		log.Fatalf("init: %v", err)
	}
	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("Entity Blueprint Editor")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(ed); err != nil {
		log.Fatal(err)
	}
}

