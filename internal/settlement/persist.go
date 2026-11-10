package settlement

import (
	"encoding/json"
	"log"
	"time"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/mlge/ecs"
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

// persistableData reports whether a task's Data can be restored from JSON alone.
func persistableData(data any) bool {
	switch d := data.(type) {
	case nil, task_requests.BuildRequest, task_requests.DigRequest, task_requests.ConvertRequest, task_requests.CraftRequest,
		*task_requests.ResearchRequest, *task_requests.SleepRequest:
		return true
	case task_requests.MineRequest:
		return d.Target == nil
	}
	return false
}

// Saved forms of entity-bound task data: the entity pointers become stable IDs,
// and the request's plain fields travel beside them.
type (
	savedTargetRef struct {
		TargetID string
	}
	savedMine struct {
		task_requests.MineRequest
		TargetID string `json:",omitempty"`
	}
	savedRetrieve struct {
		ItemID string `json:",omitempty"`
		DestID string `json:",omitempty"`
	}
	savedRelocate struct {
		task_requests.RelocateRequest
		SourceID string `json:",omitempty"`
		DestID   string `json:",omitempty"`
	}
)

func refOf(e *ecs.Entity) string {
	if e == nil {
		return ""
	}
	return components.EnsureStableID(e)
}

// encodeBoundData returns the saved form of entity-bound task data, or false if
// the data isn't entity-bound or has nothing left to do.
func encodeBoundData(data any) (any, bool) {
	switch d := data.(type) {
	case *ecs.Entity:
		if d == nil {
			return nil, false
		}
		return savedTargetRef{TargetID: refOf(d)}, true
	case task_requests.MineRequest:
		id := refOf(d.Target)
		d.Target = nil
		return savedMine{MineRequest: d, TargetID: id}, true
	case task_requests.RetrieveRequest:
		return savedRetrieve{ItemID: refOf(d.Item), DestID: refOf(d.Dest)}, true
	case task_requests.RelocateRequest:
		out := savedRelocate{SourceID: refOf(d.Source), DestID: refOf(d.DestEntity)}
		d.Source, d.DestEntity = nil, nil
		// What a worker already withdrew stays in their bag on load, and the worker
		// no longer holds the task. The rest of the order starts over from the source.
		if d.PickedUp {
			d.Qty -= d.Carried
			d.PickedUp, d.Carried = false, 0
			if d.Qty <= 0 {
				return nil, false
			}
		}
		out.RelocateRequest = d
		return out, true
	}
	return nil, false
}

// taskLink turns a loaded task's saved IDs back into its Data. It reports false
// when an entity the task names no longer exists.
type taskLink func(resolve func(id string) *ecs.Entity) (any, bool)

// resolveRef looks up an optional reference: an empty ID was never set, so only a
// non-empty ID that resolves to nothing counts as missing.
func resolveRef(resolve func(id string) *ecs.Entity, id string) (*ecs.Entity, bool) {
	if id == "" {
		return nil, true
	}
	e := resolve(id)
	return e, e != nil
}

// decodeBoundData returns the link for a saved entity-bound task, or nil if the
// saved data binds no entity and can be decoded directly.
func decodeBoundData(action task.TaskAction, raw json.RawMessage) (taskLink, error) {
	switch action {
	case task_requests.AttackAction, task_requests.PickupAction:
		var d savedTargetRef
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, err
		}
		return func(resolve func(string) *ecs.Entity) (any, bool) {
			e := resolve(d.TargetID)
			return e, e != nil
		}, nil
	case task_requests.MineAction:
		var d savedMine
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, err
		}
		if d.TargetID == "" {
			return nil, nil
		}
		return func(resolve func(string) *ecs.Entity) (any, bool) {
			e, ok := resolveRef(resolve, d.TargetID)
			d.MineRequest.Target = e
			return d.MineRequest, ok
		}, nil
	case task_requests.RetrieveAction:
		var d savedRetrieve
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, err
		}
		return func(resolve func(string) *ecs.Entity) (any, bool) {
			item, okItem := resolveRef(resolve, d.ItemID)
			dest, okDest := resolveRef(resolve, d.DestID)
			return task_requests.RetrieveRequest{Item: item, Dest: dest}, okItem && okDest
		}, nil
	case task_requests.RelocateAction:
		var d savedRelocate
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, err
		}
		return func(resolve func(string) *ecs.Entity) (any, bool) {
			src, okSrc := resolveRef(resolve, d.SourceID)
			dest, okDest := resolveRef(resolve, d.DestID)
			d.RelocateRequest.Source, d.RelocateRequest.DestEntity = src, dest
			return d.RelocateRequest, okSrc && okDest
		}, nil
	}
	return nil, nil
}

// boundTask is a loaded task waiting for its level's entities to exist.
type boundTask struct {
	task *task.Task
	link taskLink
}

// RelinkTasks queues the loaded entity-bound tasks whose entities still exist and
// drops the rest. The level's loader calls it once the entities are rebuilt; until
// then those tasks are held back, since a worker can't act on an unresolved target.
func (s *Settlement) RelinkTasks(resolve func(id string) *ecs.Entity) {
	for _, b := range s.pendingBound {
		data, ok := b.link(resolve)
		if !ok {
			log.Printf("settlement %s: dropping saved %s task at [%d,%d,%d]: its target no longer exists", s.Name, b.task.Action, b.task.X, b.task.Y, b.task.Z)
			continue
		}
		b.task.Data = data
		s.Tasks.AddTask(b.task)
	}
	s.pendingBound = nil
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
	case task_requests.ConvertAction:
		var d task_requests.ConvertRequest
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
		data := t.Data
		if !persistableData(data) {
			bound, ok := encodeBoundData(data)
			if !ok {
				log.Printf("settlement %s: not saving %s task at [%d,%d,%d]: its data can't be restored", s.Name, t.Action, t.X, t.Y, t.Z)
				continue
			}
			data = bound
		}
		st := savedTask{
			X: t.X, Y: t.Y, Z: t.Z,
			Action:          t.Action,
			Escalated:       t.Escalated,
			Created:         t.Created,
			ManuallyStopped: t.ManuallyStopped,
			Interruptible:   t.Interruptible,
		}
		if data != nil {
			raw, err := json.Marshal(data)
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
			link, err := decodeBoundData(st.Action, st.Data)
			if err != nil {
				log.Printf("settlement %s: dropping saved %s task: %v", s.Name, st.Action, err)
				continue
			}
			if link != nil {
				s.pendingBound = append(s.pendingBound, boundTask{task: t, link: link})
				continue
			}
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
