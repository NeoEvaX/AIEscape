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
	if _, ok := nodes[startID]; !ok {
		return nil, fmt.Errorf("start node %q not found in nodes", startID)
	}

	return &Network{Nodes: nodes, StartNodeID: startID}, nil
}
