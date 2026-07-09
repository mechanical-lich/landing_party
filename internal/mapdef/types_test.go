package mapdef

import "testing"

func TestSizeBlockMidArea(t *testing.T) {
	// midW = (360+512)/2 = 436, midH = (360+512)/2 = 436 → 190096.
	s := SizeBlock{
		W: IntRange{Min: 360, Max: 512},
		H: IntRange{Min: 360, Max: 512},
		Z: IntRange{Min: 8, Max: 10},
	}
	if got, want := s.MidArea(), 436*436; got != want {
		t.Fatalf("MidArea() = %d, want %d", got, want)
	}

	// Fixed-size map (min == max) reduces to width*height.
	fixed := SizeBlock{W: IntRange{Min: 100, Max: 100}, H: IntRange{Min: 80, Max: 80}}
	if got, want := fixed.MidArea(), 100*80; got != want {
		t.Fatalf("MidArea() fixed = %d, want %d", got, want)
	}
}
