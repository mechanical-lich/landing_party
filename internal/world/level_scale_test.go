package world

import "testing"

func TestFeatureAreaMultiplier_DefaultsToOne(t *testing.T) {
	l := NewLevel(4, 4, 2)
	// Unset (0) must read as 1 so placement never scales density to zero on
	// levels built without a reference area.
	if got := l.FeatureAreaMultiplier(); got != 1 {
		t.Fatalf("unset multiplier = %v, want 1", got)
	}
	l.FeatureAreaScale = -0.5 // guard against nonsense values
	if got := l.FeatureAreaMultiplier(); got != 1 {
		t.Fatalf("negative multiplier = %v, want 1", got)
	}
	l.FeatureAreaScale = 1.5
	if got := l.FeatureAreaMultiplier(); got != 1.5 {
		t.Fatalf("multiplier = %v, want 1.5", got)
	}
}

func TestColumnsWithBiome(t *testing.T) {
	l := NewLevel(4, 4, 1)
	l.AllocTerrain()
	l.SetBiome(1, 2, "desert")
	l.SetBiome(3, 0, "desert")

	cols := l.ColumnsWithBiome("desert")
	if len(cols) != 2 {
		t.Fatalf("got %d desert columns, want 2", len(cols))
	}
	// Row-major order: (3,0) precedes (1,2).
	if cols[0] != [2]int{3, 0} || cols[1] != [2]int{1, 2} {
		t.Fatalf("unexpected columns/order: %v", cols)
	}
	if got := l.ColumnsWithBiome("absent"); len(got) != 0 {
		t.Fatalf("absent biome should yield no columns, got %v", got)
	}
}
