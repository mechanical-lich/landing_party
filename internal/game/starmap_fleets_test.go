package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
)

// The Star Map names a ship by what players know it as and its class.
func TestFleetTags(t *testing.T) {
	fd := campaign.LoadFactionData()
	grey, mirror := fd.Doctrine("greyish"), fd.Doctrine("mirror")
	for _, c := range []struct {
		d    *campaign.Doctrine
		f    campaign.Fleet
		want string
	}{
		{grey, campaign.Fleet{ShipTier: 0}, "GREY SCOUT"},
		{grey, campaign.Fleet{ShipTier: 1}, "GREY FREIGHTER"},
		{grey, campaign.Fleet{ShipTier: 2, Mothership: true}, "GREY MOTHERSHIP"},
		{mirror, campaign.Fleet{}, "RIVAL COLONY SHIP"},
	} {
		if got := fleetTag(c.d, &c.f); got != c.want {
			t.Errorf("fleetTag = %q, want %q", got, c.want)
		}
	}
	if plural(1, "DAY") != "1 DAY" || plural(3, "DAY") != "3 DAYS" {
		t.Error("plural")
	}
}
