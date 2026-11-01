package workerai

import (
	"encoding/json"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/mlge/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A half-delivered task must keep its tally through the settlement's saved queue,
// with each request coming back as the exact type the handlers assert.
func TestDeliveredSurvivesSettlementRoundTrip(t *testing.T) {
	delivered := map[string]int{"metal_ore": 3}
	data := []struct {
		action task.TaskAction
		data   any
	}{
		{task_requests.BuildAction, task_requests.BuildRequest{Type: "hull_wall", Required: 5, Delivered: delivered}},
		{task_requests.CraftAction, task_requests.CraftRequest{RecipeID: "r", Required: 5, Delivered: delivered}},
		{task_requests.ResearchAction, &task_requests.ResearchRequest{TechKey: "k", Required: 5, Delivered: delivered}},
	}
	for _, c := range data {
		s := &settlement.Settlement{Name: "s"}
		s.Tasks.AddTask(&task.Task{Action: c.action, Data: c.data})
		raw, err := json.Marshal(s)
		require.NoError(t, err)
		got := &settlement.Settlement{}
		require.NoError(t, json.Unmarshal(raw, got))
		require.Len(t, got.Tasks.GetTasks(), 1)
		assert.Equal(t, c.data, got.Tasks.GetTasks()[0].Data)
	}
}
