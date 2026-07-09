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
