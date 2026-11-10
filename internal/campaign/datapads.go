package campaign

// DatapadRead is one datapad the crew picked up: when, where and what it
// said, kept so the player can read it again (the Legends tab's "Datapads"
// filter).
type DatapadRead struct {
	Day  int    `json:"day"`
	Site string `json:"site,omitempty"`
	Text string `json:"text"`
}

// RecordDatapad keeps a datapad's text, read today at site. Empty text is
// not kept.
func (c *Campaign) RecordDatapad(site, text string) {
	if c == nil || text == "" {
		return
	}
	c.DatapadsRead = append(c.DatapadsRead, DatapadRead{Day: c.Day(), Site: site, Text: text})
}

// QuestDef is the quest definition id, or nil; it changes nothing (a
// datapad's lead read where it lies).
func (c *Campaign) QuestDef(id string) *Quest {
	if c == nil || id == "" {
		return nil
	}
	return c.questByID[id]
}
