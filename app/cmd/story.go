package main

import (
	"encoding/json"
	"os"
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
