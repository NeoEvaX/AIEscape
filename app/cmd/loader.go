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
	Files       []networkFile_Item `json:"files"`
}

type networkFile_Item struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
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
		locName := "node_" + n.ID + ".loc"
		locPayload, _ := json.Marshal(NetworkLocationPayload{NodeID: n.ID})
		files = append(files, Item{
			ID:      "f-" + n.ID + "-loc",
			Name:    locName,
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

		nodes[n.ID] = &Node{
			ID:          n.ID,
			Name:        n.Name,
			Description: n.Description,
			Connections: n.Connections,
			RAM:         ram,
			CPU:         cpu,
			Dark:        n.Dark,
			Files:       files,
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

// syncWorldItems upserts all node files from the network into the items table
// so that inventory foreign keys remain valid.
func syncWorldItems(db *Database, network *Network) error {
	for _, node := range network.Nodes {
		for _, item := range node.Files {
			if err := db.UpsertItem(item); err != nil {
				return fmt.Errorf("syncing item %q: %w", item.ID, err)
			}
		}
	}
	return nil
}
