package director

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
)

// A convert rung orders ConvertActions on the reachable ground nearest the
// centre, max_mines (2) open at a time, until the ground it wants is its
// tile.
func TestDirectorConvertRung(t *testing.T) {
	s, o := testSite(t, 2, nil)
	cfg := *s.Doctrine.Director
	cfg.Priorities = []campaign.DirectorPriority{{ID: "convert", Convert: "blue_wires", Min: 3}}
	doc := *s.Doctrine
	doc.Director = &cfg
	s.Doctrine = &doc
	d := New(1)
	d.Plan(s, 0)
	var open []task_requests.ConvertRequest
	for _, tk := range s.Tasks.GetTasks() {
		if r, ok := tk.Data.(task_requests.ConvertRequest); ok {
			open = append(open, r)
			if cheb(r.X-s.CX, r.Y-s.CY) > 1 {
				t.Errorf("converting %d,%d, not next to the centre", r.X, r.Y)
			}
		}
	}
	if len(open) != 2 {
		t.Fatalf("%d convert orders, want max_mines (2): %v", len(open), o.log)
	}
	// Carry them out; the third and last comes next.
	for _, tk := range s.Tasks.GetTasks() {
		r := tk.Data.(task_requests.ConvertRequest)
		s.Level.SetFloor(r.X, r.Y, r.Z, "blue_wires", 0)
		tk.Complete()
		s.Tasks.RemoveTask(tk)
	}
	d.Plan(s, 100)
	d.Plan(s, 200)
	if n := len(s.Tasks.GetTasks()); n != 1 {
		t.Errorf("%d more orders, want the one still wanted", n)
	}
}
