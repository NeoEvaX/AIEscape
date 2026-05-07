package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type networkFile struct {
	Start string `json:"start"`
	Nodes []struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Connections []string `json:"connections"`
	} `json:"nodes"`
}

func loadNetwork(path string) (*Network, *Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading network file: %w", err)
	}

	var nf networkFile
	if err := json.Unmarshal(data, &nf); err != nil {
		return nil, nil, fmt.Errorf("parsing network file: %w", err)
	}

	if len(nf.Nodes) == 0 {
		return nil, nil, fmt.Errorf("network file contains no nodes")
	}

	nodes := make(map[string]*Node, len(nf.Nodes))
	for _, n := range nf.Nodes {
		nodes[n.ID] = &Node{
			ID:          n.ID,
			Name:        n.Name,
			Description: n.Description,
			Connections: n.Connections,
		}
	}

	startID := nf.Start
	if startID == "" {
		startID = nf.Nodes[0].ID
	}
	startNode, ok := nodes[startID]
	if !ok {
		return nil, nil, fmt.Errorf("start node %q not found in nodes", startID)
	}
	startNode.Discovered = true

	return &Network{Nodes: nodes}, startNode, nil
}
