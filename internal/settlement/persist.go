package settlement

import (
	"encoding/json"
	"log"
	"time"

	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/mlge/task"
)

// savedTask is the on-disk form of a queued task. task.TaskScheduler keeps its
// queue unexported and task.Task.Data is an `any` holding typed requests, so
// neither survives encoding/json on its own. InProgress is not saved: a loaded
// worker no longer holds the task, so it goes back in the queue.
type savedTask struct {
	X               int
	Y               int
	Z               int
	Action          task.TaskAction
	Data            json.RawMessage `json:",omitempty"`
	Escalated       bool            `json:",omitempty"`
	Created         time.Time
	ManuallyStopped bool `json:",omitempty"`
	Interruptible   bool `json:",omitempty"`
}

// persistableData reports whether a task's Data can be restored from JSON. Tasks
// bound to live entities (attack and pickup targets, retrieve items, relocate
// endpoints, entity mining) can't: entities have no stable ID to re-link by.
func persistableData(data any) bool {
	switch d := data.(type) {
	case nil, task_requests.BuildRequest, task_requests.DigRequest, task_requests.CraftRequest,
		*task_requests.ResearchRequest, *task_requests.SleepRequest:
		return true
	case task_requests.MineRequest:
		return d.Target == nil
	}
	return false
}

// decodeTaskData restores Data to the exact type the worker handlers assert for
// the action. Pointer-typed requests stay pointers.
func decodeTaskData(action task.TaskAction, raw json.RawMessage) (any, error) {
	switch action {
	case task_requests.BuildAction:
		var d task_requests.BuildRequest
		return d, json.Unmarshal(raw, &d)
	case task_requests.DigAction:
		var d task_requests.DigRequest
		return d, json.Unmarshal(raw, &d)
	case task_requests.MineAction:
		var d task_requests.MineRequest
		err := json.Unmarshal(raw, &d)
		return d, err
	case task_requests.CraftAction:
		var d task_requests.CraftRequest
		err := json.Unmarshal(raw, &d)
		return d, err
	case task_requests.ResearchAction:
		d := &task_requests.ResearchRequest{}
		return d, json.Unmarshal(raw, d)
	case task_requests.SleepAction:
		d := &task_requests.SleepRequest{}
		err := json.Unmarshal(raw, d)
		// The mount state belonged to whichever worker held the task.
		d.OnBed, d.PrevX, d.PrevY, d.PrevZ = false, 0, 0, 0
		return d, err
	}
	return nil, &json.UnsupportedValueError{Str: "task action " + string(action)}
}

// MarshalJSON writes the settlement with its task queue, skipping completed tasks
// and those that can't be restored. Live tasks are not modified.
func (s *Settlement) MarshalJSON() ([]byte, error) {
	type plain Settlement
	saved := make([]savedTask, 0, len(s.Tasks.GetTasks()))
	for _, t := range s.Tasks.GetTasks() {
		if t == nil || t.Completed {
			continue
		}
		if !persistableData(t.Data) {
			log.Printf("settlement %s: not saving %s task at [%d,%d,%d]: bound to a live entity", s.Name, t.Action, t.X, t.Y, t.Z)
			continue
		}
		st := savedTask{
			X: t.X, Y: t.Y, Z: t.Z,
			Action:          t.Action,
			Escalated:       t.Escalated,
			Created:         t.Created,
			ManuallyStopped: t.ManuallyStopped,
			Interruptible:   t.Interruptible,
		}
		if t.Data != nil {
			raw, err := json.Marshal(t.Data)
			if err != nil {
				return nil, err
			}
			st.Data = raw
		}
		saved = append(saved, st)
	}
	return json.Marshal(struct {
		*plain
		Tasks []savedTask
	}{(*plain)(s), saved})
}

// UnmarshalJSON restores the settlement and its task queue. Saves written before
// queues were persisted hold "Tasks":{} and load with an empty queue.
func (s *Settlement) UnmarshalJSON(b []byte) error {
	type plain Settlement
	aux := struct {
		*plain
		Tasks json.RawMessage
	}{plain: (*plain)(s)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	var saved []savedTask
	if len(aux.Tasks) > 0 && aux.Tasks[0] == '[' {
		if err := json.Unmarshal(aux.Tasks, &saved); err != nil {
			return err
		}
	}
	for _, st := range saved {
		t := &task.Task{
			X: st.X, Y: st.Y, Z: st.Z,
			Action:          st.Action,
			Escalated:       st.Escalated,
			Created:         st.Created,
			ManuallyStopped: st.ManuallyStopped,
			Interruptible:   st.Interruptible,
		}
		if len(st.Data) > 0 {
			data, err := decodeTaskData(st.Action, st.Data)
			if err != nil {
				log.Printf("settlement %s: dropping saved %s task: %v", s.Name, st.Action, err)
				continue
			}
			t.Data = data
		}
		s.Tasks.AddTask(t)
	}
	return nil
}
