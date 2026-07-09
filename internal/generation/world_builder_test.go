package generation

import "testing"

// A bad terrain name must not nil-panic the caller: BuildWorld allocates the
// level before the primer lookup, so it returns the empty (unprimed) level plus
// an error rather than nil.
func TestBuildWorld_UnknownTerrainReturnsNonNilLevel(t *testing.T) {
	level, err := BuildWorld(BuildWorldOptions{Width: 8, Height: 8, Depth: 4, Terrain: "no_such_terrain"})
	if err == nil {
		t.Fatal("expected error for unknown terrain")
	}
	if level == nil {
		t.Fatal("expected non-nil level even on primer-lookup failure")
	}
	if level.GetWidth() != 8 || level.GetHeight() != 8 || level.GetDepth() != 4 {
		t.Fatalf("level dims = %dx%dx%d, want 8x8x4", level.GetWidth(), level.GetHeight(), level.GetDepth())
	}
}

// Invalid dimensions can't allocate, so they remain the one nil-returning path.
func TestBuildWorld_InvalidDimsReturnNil(t *testing.T) {
	level, err := BuildWorld(BuildWorldOptions{Width: 0, Height: 8, Depth: 4, Terrain: "planet"})
	if err == nil {
		t.Fatal("expected error for invalid dimensions")
	}
	if level != nil {
		t.Fatal("expected nil level for invalid dimensions")
	}
}
