package game

import (
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Rations are Item-only entries: they list with their count, and a tag filter
// hides them because they have no tags.
func TestInspectorEntries_ListsNonStackablesWithCount(t *testing.T) {
	sc := &components.StorageComponent{}
	for i := 0; i < 5; i++ {
		r, err := factory.Create("ration_pack", 0, 0, 0)
		require.NoError(t, err)
		require.True(t, sc.AddItem(r))
	}
	ore, err := factory.Create("metal_ore", 0, 0, 0)
	require.NoError(t, err)
	ore.GetComponent(components.Material).(*components.MaterialComponent).Quantity = 12
	require.True(t, sc.AddItem(ore))

	assert.Equal(t, []inspectorEntry{
		{bp: "metal_ore", name: "Metal Ore", qty: 12},
		{bp: "ration_pack", name: "Ration Pack", qty: 5},
	}, inspectorEntries(sc, ""))

	assert.Equal(t, []inspectorEntry{{bp: "metal_ore", name: "Metal Ore", qty: 12}}, inspectorEntries(sc, "metal"))
}

func TestBeamDown_ThreeOfFiveRations(t *testing.T) {
	wm, sc := beamWorld(t, 0, 0)
	hold := wm.shipLocker()
	hold.DeductResource("ration_pack", hold.CountResource("ration_pack"))
	wm.addToShipHold("ration_pack", 5)

	require.NoError(t, wm.BeamResourceDown("ration_pack", 3))
	assert.Equal(t, 3, sc.CountResource("ration_pack"))
	assert.Equal(t, 2, hold.CountResource("ration_pack"))
}

// A site holding only a tag-filtered container refuses, says why, and keeps the
// rations aboard.
func TestBeamDown_TagFilteredSiteRefusesWithReason(t *testing.T) {
	wm := shipTestWorld(t)
	require.NoError(t, wm.EnterStartingOrbit())
	t.Cleanup(wm.Current().teardown)
	bin, err := factory.Create("mineral_bin", 2, 2, 0)
	require.NoError(t, err)
	sc := bin.GetComponent(components.Storage).(*components.StorageComponent)
	require.NotEmpty(t, sc.AllowedTags)
	sc.OwnedBy = wm.currentColonyName()
	wm.Current().level.AddEntity(bin)

	hold := wm.shipLocker()
	wm.addToShipHold("ration_pack", 5)
	before := hold.CountResource("ration_pack")

	err = wm.BeamResourceDown("ration_pack", 3)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "accepts Ration Pack"), err.Error())
	assert.Equal(t, before, hold.CountResource("ration_pack"))
	assert.Equal(t, 0, sc.CountResource("ration_pack"))
}

// Beam-up used to debit first, so a full hold destroyed the cargo.
func TestBeamUp_FullHoldKeepsWhatDidNotLand(t *testing.T) {
	wm, sc := beamWorld(t, 0, 5)
	locker := wm.SiteHoldEntity()
	hold := wm.shipLocker()
	hold.Capacity = len(hold.Items) + 2

	err := wm.BeamResourceUpFrom(locker, "ration_pack", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2 of 5")
	assert.Equal(t, 3, sc.CountResource("ration_pack"))
}
