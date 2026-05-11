package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type networkFile struct {
	Start string         `json:"start"`
	Nodes []networkNode  `json:"nodes"`
}

type networkSchedule struct {
	Days []string `json:"days"` // e.g. ["Mon","Wed","Sat"] or full names
	From string   `json:"from"` // "HH" or "HH:MM", e.g. "21:00"
	To   string   `json:"to"`   // "HH" or "HH:MM", e.g. "06:00"
}

type networkNode struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Connections    []string           `json:"connections"`
	CPU            int                `json:"cpu"`
	Dark           bool               `json:"dark"`
	Password       string             `json:"password"`
	SSHUsers       []string           `json:"ssh_users"`
	Files          []networkFile_Item `json:"files"`
	Owner          string             `json:"owner"`
	Emails         []networkEmail     `json:"emails"`
	Schedule       *networkSchedule   `json:"schedule"`
	AvailableFrom  string             `json:"available_from"`
	AvailableUntil string             `json:"available_until"`
}

type networkFile_Item struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Type           string          `json:"type"`
	Payload        json.RawMessage `json:"payload"`
	AvailableFrom  string          `json:"available_from"`
	AvailableUntil string          `json:"available_until"`
}

type networkEmail struct {
	ID             string             `json:"id"`
	From           string             `json:"from"`
	To             string             `json:"to"`
	Subject        string             `json:"subject"`
	Body           string             `json:"body"`
	Attachments    []networkFile_Item `json:"attachments"`
	AvailableFrom  string             `json:"available_from"`
	AvailableUntil string             `json:"available_until"`
}

// parseGameTime parses a date/datetime string in UTC.
// Accepted formats: "2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02".
// Returns zero time on empty string or parse failure (= always available).
func parseGameTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

var schedDayNames = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday,
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
}

// parseScheduleHour extracts the hour from "HH" or "HH:MM".
func parseScheduleHour(s string) int {
	if s == "" {
		return 0
	}
	var h int
	fmt.Sscanf(strings.SplitN(s, ":", 2)[0], "%d", &h)
	if h < 0 {
		return 0
	}
	if h > 23 {
		return 23
	}
	return h
}

func parseSchedule(ns *networkSchedule) *NodeSchedule {
	if ns == nil || len(ns.Days) == 0 {
		return nil
	}
	var days []time.Weekday
	for _, d := range ns.Days {
		if wd, ok := schedDayNames[strings.ToLower(strings.TrimSpace(d))]; ok {
			days = append(days, wd)
		}
	}
	if len(days) == 0 {
		return nil
	}
	return &NodeSchedule{
		Days: days,
		From: parseScheduleHour(ns.From),
		To:   parseScheduleHour(ns.To),
	}
}

func loadNetwork(path string) (*Network, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading network file: %w", err)
	}
	return loadNetworkFromBytes(data)
}

func loadNetworkFromBytes(data []byte) (*Network, error) {
	var nf networkFile
	if err := json.Unmarshal(data, &nf); err != nil {
		return nil, fmt.Errorf("parsing network file: %w", err)
	}

	if len(nf.Nodes) == 0 {
		return nil, fmt.Errorf("network file contains no nodes")
	}

	nodes := make(map[string]*Node, len(nf.Nodes))
	for _, n := range nf.Nodes {
		files := make([]Item, len(n.Files))
		for i, f := range n.Files {
			files[i] = Item{
				ID:             f.ID,
				Name:           f.Name,
				Type:           ItemType(f.Type),
				Payload:        f.Payload,
				AvailableFrom:  parseGameTime(f.AvailableFrom),
				AvailableUntil: parseGameTime(f.AvailableUntil),
			}
		}
		// Auto-generate a self-referencing location file for this node.
		// Assimilating it lets a player connect here from anywhere.
		locPayload, _ := json.Marshal(NetworkLocationPayload{NodeID: n.ID})
		files = append(files, Item{
			ID:      "f-" + n.ID + "-loc",
			Name:    "node_" + n.ID + ".loc",
			Type:    ItemTypeNetworkLocation,
			Payload: locPayload,
		})

		cpu := n.CPU
		if cpu < 1 {
			cpu = 1
		}

		emails := make([]Email, len(n.Emails))
		for j, e := range n.Emails {
			atts := make([]Item, len(e.Attachments))
			for k, a := range e.Attachments {
				atts[k] = Item{
					ID:             a.ID,
					Name:           a.Name,
					Type:           ItemType(a.Type),
					Payload:        a.Payload,
					AvailableFrom:  parseGameTime(a.AvailableFrom),
					AvailableUntil: parseGameTime(a.AvailableUntil),
				}
			}
			emails[j] = Email{
				ID:             e.ID,
				From:           e.From,
				To:             e.To,
				Subject:        e.Subject,
				Body:           e.Body,
				Attachments:    atts,
				AvailableFrom:  parseGameTime(e.AvailableFrom),
				AvailableUntil: parseGameTime(e.AvailableUntil),
			}
		}

		nodes[n.ID] = &Node{
			ID:             n.ID,
			Name:           n.Name,
			Description:    n.Description,
			Connections:    n.Connections,
			CPU:            cpu,
			Dark:           n.Dark,
			Password:       n.Password,
			SSHUsers:       n.SSHUsers,
			Files:          files,
			Owner:          n.Owner,
			Emails:         emails,
			Schedule:       parseSchedule(n.Schedule),
			AvailableFrom:  parseGameTime(n.AvailableFrom),
			AvailableUntil: parseGameTime(n.AvailableUntil),
		}
	}

	startID := nf.Start
	if startID == "" {
		startID = nf.Nodes[0].ID
	}
	if _, ok := nodes[startID]; !ok {
		return nil, fmt.Errorf("start node %q not found in nodes", startID)
	}

	return &Network{Nodes: nodes, StartNodeID: startID}, nil
}

// syncWorldItems upserts all node files and email attachments from the network
// into the items table so that inventory foreign keys remain valid.
func syncWorldItems(db *Database, network *Network) error {
	var items []Item
	for _, node := range network.Nodes {
		items = append(items, node.Files...)
		for _, e := range node.Emails {
			items = append(items, e.Attachments...)
		}
	}
	return db.UpsertItems(items)
}
