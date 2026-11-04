package gui

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	mlge_text "github.com/mechanical-lich/mlge/text"
	"golang.org/x/image/font/sfnt"
)

// uiFont parses the TTF mlge/text draws every string with. The font is
// embedded unexported, so it is read from beside the package's source.
func uiFont(t *testing.T) *sfnt.Font {
	t.Helper()
	file, _ := runtime.FuncForPC(reflect.ValueOf(mlge_text.Draw).Pointer()).FileLine(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "Roboto-Regular.ttf"))
	if err != nil {
		t.Skipf("UI font not found beside mlge/text (%v)", err)
	}
	f, err := sfnt.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// missingGlyphs returns the runes of s the font has no glyph for. Control
// characters are ignored.
func missingGlyphs(f *sfnt.Font, s string) string {
	var buf sfnt.Buffer
	var out []rune
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			continue
		}
		if i, err := f.GlyphIndex(&buf, r); err != nil || i == 0 {
			if !strings.ContainsRune(string(out), r) {
				out = append(out, r)
			}
		}
	}
	return string(out)
}

// Every string literal in the game's Go code and every string in data/*.json
// draws with glyphs the font has. Roboto has no arrows, so "← Back" rendered
// as " Back" and "▼ Beam Down" as " Beam Down".
func TestUIStringsUseFontGlyphs(t *testing.T) {
	f := uiFont(t)
	root := filepath.Join("..", "..")
	report := func(where, s string) {
		if m := missingGlyphs(f, s); m != "" {
			t.Errorf("%s: %q has no glyph for %q", where, s, m)
		}
	}

	fset := token.NewFileSet()
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if s, err := strconv.Unquote(lit.Value); err == nil {
				report(fset.Position(lit.Pos()).String(), s)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	jsonFiles, _ := filepath.Glob(filepath.Join(root, "data", "*.json"))
	for _, path := range jsonFiles {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var v any
		if err := json.Unmarshal(b, &v); err != nil {
			continue // not every data file is plain JSON
		}
		var walk func(any)
		walk = func(v any) {
			switch x := v.(type) {
			case string:
				report(filepath.Base(path), x)
			case []any:
				for _, e := range x {
					walk(e)
				}
			case map[string]any:
				for _, e := range x {
					walk(e)
				}
			}
		}
		walk(v)
	}
}
