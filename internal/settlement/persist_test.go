package settlement

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/mlge/task"
)

func roundTrip(t *testing.T, s *Settlement) *Settlement {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Settlement
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &out
}

// Worker handlers type-assert Data, so each request must come back as the exact
// type (value or pointer) it was queued with.
func TestTaskDataRoundTripsWithExactType(t *testing.T) {
	cases := []struct {
		action task.TaskAction
		data   any
	}{
		{task_requests.BuildAction, task_requests.BuildRequest{Type: "hull_wall", X: 1, Y: 2, Z: 3, Progress: 4, Required: 5}},
		{task_requests.DigAction, task_requests.DigRequest{X: 1, Y: 2, Z: 3, Progress: 1, Required: 5}},
		{task_requests.MineAction, task_requests.MineRequest{X: 1, Y: 2, Z: 3, Required: 50}},
		{task_requests.CraftAction, task_requests.CraftRequest{RecipeID: "r", WorkbenchX: 7, Progress: 2, Required: 9}},
		{task_requests.ResearchAction, &task_requests.ResearchRequest{TechKey: "k", Building: "b", Progress: 3, Required: 8}},
		{task_requests.SleepAction, &task_requests.SleepRequest{X: 1, Y: 2, Z: 3, Required: 10}},
		{task_requests.ForageAction, nil},
	}
	for _, c := range cases {
		s := &Settlement{Name: "s"}
		s.Tasks.AddTask(&task.Task{Action: c.action, Data: c.data})
		got := roundTrip(t, s).Tasks.GetTasks()
		if len(got) != 1 {
			t.Errorf("%s: %d tasks after round trip, want 1", c.action, len(got))
			continue
		}
		if !reflect.DeepEqual(got[0].Data, c.data) {
			t.Errorf("%s: Data = %#v, want %#v", c.action, got[0].Data, c.data)
		}
	}
}

func TestTaskFieldsRoundTrip(t *testing.T) {
	created := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	s := &Settlement{Name: "s", CenterX: 9, KnownTechs: []string{"tech"}}
	s.Tasks.AddTask(&task.Task{X: 1, Y: 2, Z: 3, Action: task_requests.DigAction, Escalated: true,
		Created: created, ManuallyStopped: true, Interruptible: true})

	out := roundTrip(t, s)
	if out.Name != "s" || out.CenterX != 9 || !out.HasTech("tech") {
		t.Errorf("settlement fields lost: %+v", out)
	}
	got := out.Tasks.GetTasks()[0]
	if got.X != 1 || got.Y != 2 || got.Z != 3 || !got.Escalated || !got.ManuallyStopped || !got.Interruptible || !got.Created.Equal(created) {
		t.Errorf("task fields lost: %+v", got)
	}
}

// In-progress tasks return to the queue on load without touching the live task
// still being worked.
func TestInProgressTaskIsRequeued(t *testing.T) {
	live := &task.Task{Action: task_requests.DigAction, InProgress: true,
		Data: task_requests.DigRequest{Progress: 3, Required: 5}}
	s := &Settlement{Name: "s"}
	s.Tasks.AddTask(live)

	got := roundTrip(t, s).Tasks.GetTasks()[0]
	if got.InProgress {
		t.Error("loaded task still in progress: no worker holds it, so it would never run")
	}
	if got.Data.(task_requests.DigRequest).Progress != 3 {
		t.Error("work progress lost")
	}
	if !live.InProgress {
		t.Error("saving modified the live task")
	}
}

func TestCompletedTasksAreNotSaved(t *testing.T) {
	s := &Settlement{Name: "s"}
	s.Tasks.AddTask(&task.Task{Action: task_requests.DigAction, Completed: true})

	if got := roundTrip(t, s).Tasks.GetTasks(); len(got) != 0 {
		t.Errorf("%d tasks saved, want 0: %+v", len(got), got[0])
	}
}

func TestSleepMountStateIsNotRestored(t *testing.T) {
	s := &Settlement{Name: "s"}
	s.Tasks.AddTask(&task.Task{Action: task_requests.SleepAction,
		Data: &task_requests.SleepRequest{X: 1, Required: 10, OnBed: true, PrevX: 4, PrevY: 5, PrevZ: 6}})

	got := roundTrip(t, s).Tasks.GetTasks()[0].Data.(*task_requests.SleepRequest)
	if got.OnBed || got.PrevX != 0 || got.PrevY != 0 || got.PrevZ != 0 {
		t.Errorf("mount state restored: %+v", got)
	}
}

// Saves written before queues were persisted hold an empty object.
func TestLegacySaveLoadsWithEmptyQueue(t *testing.T) {
	var s Settlement
	raw := `{"Name":"old","Color":null,"CenterX":2,"CenterY":0,"CenterZ":0,"Tasks":{},"KnownTechs":["t"]}`
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("legacy save failed to load: %v", err)
	}
	if s.Name != "old" || s.CenterX != 2 || !s.HasTech("t") || s.Tasks.Count() != 0 {
		t.Errorf("legacy settlement = %+v", s)
	}
}
