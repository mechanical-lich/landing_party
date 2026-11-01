package gui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueueRowText_GatheringUntilPaidThenPercent(t *testing.T) {
	assert.Equal(t, "Rifle  gathering 0/25", queueRowText("Rifle", 0, 0, 25))
	assert.Equal(t, "Rifle  gathering 20/25", queueRowText("Rifle", 0, 20, 25))
	assert.Equal(t, "Rifle  0%", queueRowText("Rifle", 0, 25, 25))
	assert.Equal(t, "Rifle  40%", queueRowText("Rifle", 0.4, 25, 25))
	// A task with no material cost was never gathering.
	assert.Equal(t, "Rifle  40%", queueRowText("Rifle", 0.4, 0, 0))
}

func TestTaskNeedLines(t *testing.T) {
	half := &HoveredTileInfo{TaskNeeds: []TaskNeed{
		{Blueprint: "crystal", Delivered: 0, Required: 1},
		{Blueprint: "metal_ore", Delivered: 3, Required: 5},
	}}
	assert.Equal(t, []string{"Needs:", "  crystal 0/1", "  metal_ore 3/5"}, taskNeedLines(half))
	assert.Equal(t, []string{"Materials delivered"}, taskNeedLines(&HoveredTileInfo{TaskPaid: true}))
	assert.Empty(t, taskNeedLines(&HoveredTileInfo{}))
}

func TestTileInfoEqual_SeesDeliveryChanges(t *testing.T) {
	a := HoveredTileInfo{TaskNeeds: []TaskNeed{{Blueprint: "metal_ore", Delivered: 1, Required: 5}}}
	b := HoveredTileInfo{TaskNeeds: []TaskNeed{{Blueprint: "metal_ore", Delivered: 2, Required: 5}}}
	assert.False(t, tileInfoEqual(a, b), "the panel must rebuild as material arrives")
	assert.True(t, tileInfoEqual(a, a))
	assert.False(t, tileInfoEqual(HoveredTileInfo{}, HoveredTileInfo{TaskPaid: true}))
}

func TestWorkerStateLabel(t *testing.T) {
	assert.Equal(t, "gathering", workerStateLabel("gather_materials"))
	assert.Equal(t, "idle", workerStateLabel("idle"))
}
