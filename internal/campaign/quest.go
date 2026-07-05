package campaign

import (
	"fmt"

	"github.com/mechanical-lich/landing_party/internal/objective"
)

// QuestStatus is the lifecycle of a quest for a given campaign.
type QuestStatus string

const (
	QuestHidden    QuestStatus = "hidden"    // prerequisite not yet met
	QuestAvailable QuestStatus = "available" // offered, not yet accepted
	QuestActive    QuestStatus = "active"    // accepted, objective in progress
	QuestCompleted QuestStatus = "completed"
)

// QuestReward is granted when a quest completes. Fuel and Resources are added
// to the ship hold.
type QuestReward struct {
	Fuel      int            `json:"fuel,omitempty"`
	Resources map[string]int `json:"resources,omitempty"`
	// SpawnSystems, when > 0, procedurally generates that many new systems
	// (each with its own quests) on completion — the campaign expands as the
	// player explores. Deterministic via Campaign.GenSeq.
	SpawnSystems int `json:"spawn_systems,omitempty"`
}

// Quest is a data-driven objective. Objective reuses the internal/objective
// rule engine (resource_gathered, entity_killed, tech_researched,
// structure_built, days_survived, ...) so quests share one evaluation path.
type Quest struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Objective     objective.Rule `json:"objective"`
	Reward        QuestReward    `json:"reward"`
	AutoAccept    bool           `json:"auto_accept,omitempty"`
	RequiresQuest string         `json:"requires_quest,omitempty"`
	// Location, when set, binds the quest to one overworld location: accepting
	// the quest reveals that location, and the objective is only evaluated
	// while the landing party is actually there.
	Location string `json:"location,omitempty"`
	// Delivery is how the quest reaches the player: "" = offered in the pool,
	// "datapad" = hidden until its datapad clue is recovered at Location.
	Delivery string `json:"delivery,omitempty"`
	// TitlePrefix is set for quests whose target is named by a generation
	// script (see Campaign.BindQuestTargetName). Until the structure runs the
	// title reads "<prefix> — <loc>"; once the script marks its NPC it
	// becomes "<prefix>: <npc> — <loc>".
	TitlePrefix string `json:"title_prefix,omitempty"`
}

// QuestProgress is the serialized per-campaign state for one quest.
type QuestProgress struct {
	Status QuestStatus `json:"status"`
}

// BindQuests (re)builds the runtime quest indices/evaluators from
// c.QuestDefs and reconciles the serialized progress map: new quests get
// their starting status, existing progress is preserved, prerequisite gating
// is re-validated, and accepted/active quests keep their location revealed.
// Call once per session after QuestDefs is populated (generated or loaded).
func (c *Campaign) BindQuests() {
	c.questByID = make(map[string]*Quest, len(c.QuestDefs))
	c.questEval = make(map[string]*objective.Evaluator, len(c.QuestDefs))
	for i := range c.QuestDefs {
		q := &c.QuestDefs[i]
		c.questByID[q.ID] = q
		c.questEval[q.ID] = objective.New(objective.RuleSet{Rules: []objective.Rule{q.Objective}})
	}
	if c.Quests == nil {
		c.Quests = make(map[string]*QuestProgress, len(c.QuestDefs))
	}
	for i := range c.QuestDefs {
		q := &c.QuestDefs[i]
		if _, ok := c.Quests[q.ID]; ok {
			continue
		}
		c.Quests[q.ID] = &QuestProgress{Status: startingStatus(q)}
	}
	// Re-validate prerequisite gating against the (possibly stale) saved
	// progress and current definitions — a quest may have gained/changed a
	// requires_quest, or a save predates it.
	c.reconcileQuestGates()
	// Any quest already accepted/active keeps its location revealed.
	for i := range c.QuestDefs {
		q := &c.QuestDefs[i]
		if p := c.Quests[q.ID]; p != nil && (p.Status == QuestActive || p.Status == QuestCompleted) {
			c.revealQuestLocation(q)
		}
	}
}

// reconcileQuestGates enforces the prerequisite invariant against the current
// definitions and (possibly stale) saved progress, to a fixpoint so chained
// prerequisites resolve in any order:
//   - a non-completed quest whose requires_quest isn't completed -> Hidden
//   - a Hidden quest whose requires_quest is completed -> unlocked
//     (Active if auto_accept, else Available)
//
// Completed quests are never downgraded.
func (c *Campaign) reconcileQuestGates() {
	for pass := 0; pass < len(c.QuestDefs)+1; pass++ {
		changed := false
		for i := range c.QuestDefs {
			q := &c.QuestDefs[i]
			if q.RequiresQuest == "" {
				continue
			}
			p := c.Quests[q.ID]
			if p == nil || p.Status == QuestCompleted {
				continue
			}
			pre := c.Quests[q.RequiresQuest]
			preDone := pre != nil && pre.Status == QuestCompleted
			switch {
			case !preDone && p.Status != QuestHidden:
				p.Status = QuestHidden
				changed = true
			case preDone && p.Status == QuestHidden:
				if q.AutoAccept {
					p.Status = QuestActive
				} else {
					p.Status = QuestAvailable
				}
				changed = true
			}
		}
		if !changed {
			break
		}
	}
}

// revealQuestLocation makes a quest's bound location visible on the star map.
func (c *Campaign) revealQuestLocation(q *Quest) {
	if q.Location == "" {
		return
	}
	if loc := c.Locations[q.Location]; loc != nil {
		loc.Discovered = true
	}
}

func startingStatus(q *Quest) QuestStatus {
	if q.RequiresQuest != "" || q.Delivery == "datapad" {
		return QuestHidden
	}
	if q.AutoAccept {
		return QuestActive
	}
	return QuestAvailable
}

func (c *Campaign) questsByStatus(status QuestStatus) []*Quest {
	var out []*Quest
	for i := range c.QuestDefs {
		q := &c.QuestDefs[i]
		if p := c.Quests[q.ID]; p != nil && p.Status == status {
			out = append(out, q)
		}
	}
	return out
}

func (c *Campaign) AvailableQuests() []*Quest { return c.questsByStatus(QuestAvailable) }
func (c *Campaign) ActiveQuests() []*Quest    { return c.questsByStatus(QuestActive) }
func (c *Campaign) CompletedQuests() []*Quest { return c.questsByStatus(QuestCompleted) }

// BindQuestTargetName finalizes a structure-spawned quest's title once the
// generation script names and marks its target NPC. No-op if the quest has no
// TitlePrefix (i.e. it wasn't a script-named quest).
func (c *Campaign) BindQuestTargetName(questID, npc string) {
	q := c.questByID[questID]
	if q == nil || q.TitlePrefix == "" || npc == "" {
		return
	}
	locName := ""
	if loc := c.Locations[q.Location]; loc != nil {
		locName = loc.Name
	}
	q.Name = fmt.Sprintf("%s: %s — %s", q.TitlePrefix, npc, locName)
}

// AcceptQuest moves an available quest to active.
func (c *Campaign) AcceptQuest(id string) bool {
	p := c.Quests[id]
	if p == nil || p.Status != QuestAvailable {
		return false
	}
	p.Status = QuestActive
	if q := c.questByID[id]; q != nil {
		c.revealQuestLocation(q)
	}
	return true
}

// UnlockQuest moves a hidden datapad quest into the available pool (NOT
// accepted) when its clue is recovered. Returns the quest definition (for the
// "new lead" message), or nil if it can't be unlocked.
func (c *Campaign) UnlockQuest(id string) *Quest {
	p := c.Quests[id]
	q := c.questByID[id]
	if p == nil || q == nil || p.Status != QuestHidden {
		return nil
	}
	p.Status = QuestAvailable
	return q
}

// EvaluateQuests checks every active quest's objective against ctx and returns
// the quests that just completed (their status is set to completed and any
// dependent hidden quests are unlocked to their starting status). Reward
// application is the caller's responsibility (it needs the ship/locations).
func (c *Campaign) EvaluateQuests(ctx objective.EvalContext) []*Quest {
	var done []*Quest
	for _, q := range c.ActiveQuests() {
		// Defence in depth: never complete a quest whose prerequisite is not
		// itself completed (guards against any stale/forced status).
		if q.RequiresQuest != "" {
			if pre := c.Quests[q.RequiresQuest]; pre == nil || pre.Status != QuestCompleted {
				continue
			}
		}
		// Location-bound quests only progress while the party is there.
		if q.Location != "" && q.Location != c.CurrentLocationID {
			continue
		}
		ev := c.questEval[q.ID]
		if ev == nil {
			ev = objective.New(objective.RuleSet{Rules: []objective.Rule{q.Objective}})
			c.questEval[q.ID] = ev
		}
		if _, ok := ev.Evaluate(ctx); ok {
			c.Quests[q.ID].Status = QuestCompleted
			done = append(done, q)
		}
	}
	if len(done) == 0 {
		return nil
	}
	// Unlock quests gated on the ones that just completed.
	for i := range c.QuestDefs {
		q := &c.QuestDefs[i]
		p := c.Quests[q.ID]
		if p == nil || p.Status != QuestHidden || q.RequiresQuest == "" {
			continue
		}
		if rp := c.Quests[q.RequiresQuest]; rp != nil && rp.Status == QuestCompleted {
			if q.AutoAccept {
				p.Status = QuestActive
				c.revealQuestLocation(q)
			} else {
				p.Status = QuestAvailable
			}
		}
	}
	return done
}
