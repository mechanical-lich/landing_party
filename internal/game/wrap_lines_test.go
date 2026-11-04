package game

import (
	"strings"
	"testing"
)

func TestWrapLinesFitsWidth(t *testing.T) {
	text := "Your landing site sits within the territory of an ancient yeti clan. The elders are extremely territorial."
	lines := wrapLines(text, 12, 248) // 7px per char estimate -> 35 chars
	if len(lines) < 3 {
		t.Fatalf("expected several lines, got %q", lines)
	}
	for _, l := range lines {
		if len(l) > 35 {
			t.Errorf("line %q is %d chars, over 35", l, len(l))
		}
	}
	if got := strings.Join(lines, " "); got != text {
		t.Errorf("rejoined text changed:\n got %q\nwant %q", got, text)
	}
}

func TestClipLinesStopsAboveFloor(t *testing.T) {
	lines := []string{"one two", "three four", "five six", "seven"}
	// Room for two 16px lines between y=100 and maxY=140.
	got := clipLines(lines, 100, 16, 140)
	if len(got) != 2 {
		t.Fatalf("kept %d lines, want 2: %q", len(got), got)
	}
	if got[1] != "three four…" {
		t.Errorf("last kept line = %q, want an ellipsis", got[1])
	}
	if lines[1] != "three four" {
		t.Errorf("clipLines modified its input: %q", lines[1])
	}
	if all := clipLines(lines, 100, 16, 0); len(all) != 4 {
		t.Errorf("maxY 0 should keep all lines, kept %d", len(all))
	}
	if all := clipLines(lines, 100, 16, 1000); len(all) != 4 || all[3] != "seven" {
		t.Errorf("lines that fit should be untouched, got %q", all)
	}
}
