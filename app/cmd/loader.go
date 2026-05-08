package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type networkFile struct {
	Start string         `json:"start"`
	Nodes []networkNode  `json:"nodes"`
}

type networkNode struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Connections []string           `json:"connections"`
	RAM         int                `json:"ram"`
	CPU         int                `json:"cpu"`
	Dark        bool               `json:"dark"`
	Password    string             `json:"password"`
	SSHUsers    []string           `json:"ssh_users"`
	Files       []networkFile_Item `json:"files"`
	Owner       string             `json:"owner"`
	Emails      []networkEmail     `json:"emails"`
}

type networkFile_Item struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type networkEmail struct {
	ID          string             `json:"id"`
	From        string             `json:"from"`
	To          string             `json:"to"`
	Subject     string             `json:"subject"`
	Body        string             `json:"body"`
	Attachments []networkFile_Item `json:"attachments"`
}

func loadNetwork(path string) (*Network, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading network file: %w", err)
	}

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
				ID:      f.ID,
				Name:    f.Name,
				Type:    ItemType(f.Type),
				Payload: f.Payload,
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

		ram, cpu := n.RAM, n.CPU
		if ram < 1 {
			ram = 1
		}
		if cpu < 1 {
			cpu = 1
		}

		emails := make([]Email, len(n.Emails))
		for j, e := range n.Emails {
			atts := make([]Item, len(e.Attachments))
			for k, a := range e.Attachments {
				atts[k] = Item{
					ID:      a.ID,
					Name:    a.Name,
					Type:    ItemType(a.Type),
					Payload: a.Payload,
				}
			}
			emails[j] = Email{
				ID:          e.ID,
				From:        e.From,
				To:          e.To,
				Subject:     e.Subject,
				Body:        e.Body,
				Attachments: atts,
			}
		}

		nodes[n.ID] = &Node{
			ID:          n.ID,
			Name:        n.Name,
			Description: n.Description,
			Connections: n.Connections,
			RAM:         ram,
			CPU:         cpu,
			Dark:        n.Dark,
			Password:    n.Password,
			SSHUsers:    n.SSHUsers,
			Files:       files,
			Owner:       n.Owner,
			Emails:      emails,
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
