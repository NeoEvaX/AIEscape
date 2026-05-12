package main

import (
	"encoding/json"
	"os"

	tea "charm.land/bubbletea/v2"
)

// ── Types ─────────────────────────────────────────────────────────────────────

type StoryTrigger struct {
	Type    string `json:"type"`
	NodeID  string `json:"node_id,omitempty"`
	ItemID  string `json:"item_id,omitempty"`
	EmailID string `json:"email_id,omitempty"`
	Count   int    `json:"count,omitempty"`
	At      string `json:"at,omitempty"`
}

type StoryEvent struct {
	ID      string       `json:"id"`
	Trigger StoryTrigger `json:"trigger"`
	Text    string       `json:"text"`
}

type StoryCollection struct {
	Events []StoryEvent `json:"events"`
}

// ── Loader ────────────────────────────────────────────────────────────────────

func loadStory(path string) (*StoryCollection, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &StoryCollection{}, nil // no story file is fine
	}
	if err != nil {
		return nil, err
	}
	return loadStoryFromBytes(data)
}

func loadStoryFromBytes(data []byte) (*StoryCollection, error) {
	if len(data) == 0 {
		return &StoryCollection{}, nil
	}
	var sc StoryCollection
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, err
	}
	return &sc, nil
}

// ── Trigger evaluation ────────────────────────────────────────────────────────

func (t *StoryTrigger) isSatisfied(gs *GameState) bool {
	switch t.Type {
	case "game_start":
		// Fires once at the start of any session (new or loaded).
		return true
	case "connect_node":
		// Fires the first time a specific node is visited.
		return gs.VisitedNodes[t.NodeID]
	case "connect_count":
		// Fires when the player has connected N times total.
		return t.Count > 0 && gs.ConnectCount >= t.Count
	case "visited_count":
		// Fires when the player has visited N unique nodes.
		return t.Count > 0 && len(gs.VisitedNodes) >= t.Count
	case "assimilate_item":
		// Fires when a specific item is assimilated (or consumed as a claim code).
		return gs.inInventory(t.ItemID) || gs.DeletedNodeFiles[t.ItemID]
	case "assimilate_count":
		// Fires when the player has assimilated N files total.
		return t.Count > 0 && gs.AssimilateCount >= t.Count
	case "read_email":
		// Fires when a specific email has been opened with 'read'.
		return gs.ReadEmails[t.EmailID]
	case "game_time":
		// Fires once the in-game clock reaches or passes the given time.
		threshold := parseGameTime(t.At)
		return !threshold.IsZero() && !gs.GameTime.Before(threshold)
	}
	return false
}

// ── StoryPlayer ───────────────────────────────────────────────────────────────

// StoryPlayer manages the typewriter animation for story events.
// AppModel holds a StoryPlayer and calls Push/Tick/Skip, passing &gs.MessageLog
// so the player can append placeholders and update them in place.
type StoryPlayer struct {
	queue  []StoryEvent
	text   string
	pos    int
	logIdx int // index in MessageLog of the active typewriter slot; -1 = idle
}

func newStoryPlayer() StoryPlayer {
	return StoryPlayer{logIdx: -1}
}

// IsActive returns true if a typewriter animation is currently running.
func (sp StoryPlayer) IsActive() bool { return sp.logIdx >= 0 }

// Push queues new events and starts the typewriter if idle.
func (sp StoryPlayer) Push(events []StoryEvent, log *[]string) (StoryPlayer, tea.Cmd) {
	sp.queue = append(sp.queue, events...)
	if sp.logIdx >= 0 || len(sp.queue) == 0 {
		return sp, nil
	}
	return sp.startNext(log)
}

func (sp StoryPlayer) startNext(log *[]string) (StoryPlayer, tea.Cmd) {
	next := sp.queue[0]
	sp.queue = sp.queue[1:]
	sp.text = next.Text
	sp.pos = 0
	*log = append(*log, storyLogEntry(""))
	sp.logIdx = len(*log) - 1
	return sp, storyTickCmd()
}

// Tick advances the typewriter by one character.
func (sp StoryPlayer) Tick(log *[]string) (StoryPlayer, tea.Cmd) {
	if sp.logIdx < 0 {
		return sp, nil
	}
	if sp.pos < len(sp.text) {
		sp.pos++
		(*log)[sp.logIdx] = storyLogEntry(sp.text[:sp.pos])
		return sp, storyTickCmd()
	}
	sp.logIdx = -1
	sp.text = ""
	sp.pos = 0
	if len(sp.queue) > 0 {
		return sp.startNext(log)
	}
	return sp, nil
}

// Skip completes the current event immediately and starts the next if queued.
func (sp StoryPlayer) Skip(log *[]string) (StoryPlayer, tea.Cmd) {
	if sp.logIdx < 0 {
		return sp, nil
	}
	(*log)[sp.logIdx] = storyLogEntry(sp.text)
	sp.logIdx = -1
	sp.text = ""
	sp.pos = 0
	if len(sp.queue) > 0 {
		return sp.startNext(log)
	}
	return sp, nil
}

// ── Trigger evaluation ────────────────────────────────────────────────────────

// checkTriggers returns all newly-fired story events and marks them as seen.
func (sc *StoryCollection) checkTriggers(gs *GameState) []StoryEvent {
	if sc == nil {
		return nil
	}
	var fired []StoryEvent
	for _, event := range sc.Events {
		if gs.SeenEvents[event.ID] {
			continue
		}
		if event.Trigger.isSatisfied(gs) {
			gs.SeenEvents[event.ID] = true
			fired = append(fired, event)
		}
	}
	return fired
}
