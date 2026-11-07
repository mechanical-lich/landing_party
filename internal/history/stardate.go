// Package history is a campaign's generated past and the record of play: alien
// civilizations, the figures and events of the prehistory, and each site's
// history ending in the map's present. See docs/brainstorm/lore_plan.md.
//
// It sits below internal/campaign (campaign imports history, never the other
// way round), so the generator, the dump tool and the data validation tests run
// without a campaign, and Campaign can carry a *History in its save.
package history

// StarDate is the campaign-wide day count: the colonists' own reckoning of
// time, independent of any planet. Day 0 is the campaign's start; prehistory
// is dated in negative star dates.
type StarDate int

// TicksPerDay is how many world ticks make one star-date day. It matches the
// levels' own day-night clock (MainState.advanceClock), so a planet's day and
// the campaign's run at the same rate.
const TicksPerDay = 750
