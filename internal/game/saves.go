package game

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

const savesDir = "saves"

type SaveMeta struct {
	Name       string    `json:"name"`
	ScenarioID string    `json:"scenario_id"`
	MapID      string    `json:"map_id"`
	Seed       int64     `json:"seed"`
	SavedAt    time.Time `json:"saved_at"`
	MapSizeW   int       `json:"map_size_w"`
	MapSizeH   int       `json:"map_size_h"`
	MapSizeZ   int       `json:"map_size_z"`
}

type saveFile struct {
	Meta      SaveMeta       `json:"meta"`
	WorldData world.SaveData `json:"world_data"`
}

func sanitizeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune('_')
		}
	}
	s := b.String()
	if s == "" {
		s = "settlement"
	}
	return s
}

func saveFilePath(name string) string {
	return filepath.Join(savesDir, sanitizeName(name)+".json")
}

func metaFilePath(name string) string {
	return filepath.Join(savesDir, sanitizeName(name)+".meta.json")
}

func marshalSaveFile(sf saveFile) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if err := json.NewEncoder(gz).Encode(sf); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func unmarshalSaveFile(b []byte) (saveFile, error) {
	gz, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		var sf saveFile
		return sf, json.Unmarshal(b, &sf)
	}
	defer gz.Close()
	raw, err := io.ReadAll(gz)
	if err != nil {
		return saveFile{}, err
	}
	var sf saveFile
	return sf, json.Unmarshal(raw, &sf)
}

func SaveSettlement(level *world.Level, meta SaveMeta) error {
	if err := os.MkdirAll(savesDir, 0755); err != nil {
		return fmt.Errorf("saves dir: %w", err)
	}
	meta.SavedAt = time.Now()

	metaData, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	if err := os.WriteFile(metaFilePath(meta.Name), metaData, 0644); err != nil {
		return fmt.Errorf("write meta: %w", err)
	}

	sf := saveFile{
		Meta:      meta,
		WorldData: world.SaveLevel(level),
	}
	compressed, err := marshalSaveFile(sf)
	if err != nil {
		return fmt.Errorf("compress save: %w", err)
	}
	return os.WriteFile(saveFilePath(meta.Name), compressed, 0644)
}

func ListSaves() ([]SaveMeta, error) {
	entries, err := os.ReadDir(savesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list saves: %w", err)
	}

	var metas []SaveMeta
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".meta.json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(savesDir, e.Name()))
		if err != nil {
			continue
		}
		var m SaveMeta
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		metas = append(metas, m)
	}
	return metas, nil
}

func LoadSave(name string) (*MainState, error) {
	raw, err := os.ReadFile(saveFilePath(name))
	if err != nil {
		return nil, fmt.Errorf("read save: %w", err)
	}
	sf, err := unmarshalSaveFile(raw)
	if err != nil {
		return nil, fmt.Errorf("parse save: %w", err)
	}

	level := world.LoadSaveData(sf.WorldData)
	cfg := SettlementConfig{
		Name:       sf.Meta.Name,
		ScenarioID: sf.Meta.ScenarioID,
		MapID:      sf.Meta.MapID,
		Seed:       sf.Meta.Seed,
	}
	return newMainStateFromLevel(level, cfg)
}
