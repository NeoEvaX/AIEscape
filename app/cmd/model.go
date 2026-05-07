package main

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
)

// ── Types ─────────────────────────────────────────────────────────────────────

type Node struct {
	ID          string
	Name        string
	Description string
	Connections []string
	Discovered  bool
}

type Network struct {
	Nodes       map[string]*Node
	StartNodeID string
}

func (n *Network) CanReach(from, to string) bool {
	node := n.Nodes[from]
	for _, conn := range node.Connections {
		if conn == to {
			return true
		}
	}
	return false
}

type GameState struct {
	SaveID       int64
	SaveName     string
	Network      *Network
	CurrentNode  *Node
	VisitedNodes map[string]bool
	Input        textinput.Model
	Viewport     viewport.Model
	MessageLog   []string
}

// ── Constructors ──────────────────────────────────────────────────────────────

func newGameState(network *Network, saveID int64, saveName string, startNode *Node) *GameState {
	ti := textinput.New()
	ti.Placeholder = "Type a command..."
	ti.Focus()

	return &GameState{
		SaveID:       saveID,
		SaveName:     saveName,
		Network:      network,
		CurrentNode:  startNode,
		VisitedNodes: map[string]bool{startNode.ID: true},
		Input:        ti,
		MessageLog:   []string{nodeInfo(startNode)},
	}
}

func newGameStateFromSave(network *Network, save *Save, currentNode *Node, visited []string) *GameState {
	ti := textinput.New()
	ti.Placeholder = "Type a command..."
	ti.Focus()

	visitedMap := make(map[string]bool, len(visited))
	for _, id := range visited {
		visitedMap[id] = true
		if node, ok := network.Nodes[id]; ok {
			node.Discovered = true
		}
	}

	return &GameState{
		SaveID:       save.ID,
		SaveName:     save.Name,
		Network:      network,
		CurrentNode:  currentNode,
		VisitedNodes: visitedMap,
		Input:        ti,
		MessageLog:   []string{nodeInfo(currentNode)},
	}
}

// ── Game logic ────────────────────────────────────────────────────────────────

// handleCommand processes user input. Returns true if the player moved nodes
// (i.e., the save should be persisted).
func (gs *GameState) handleCommand(input string) bool {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false
	}

	gs.MessageLog = append(gs.MessageLog, "> "+input)

	switch parts[0] {
	case "help", "?":
		gs.MessageLog = append(gs.MessageLog,
			"Commands:",
			"  scan           - list node IDs connected to the current node",
			"  connect <id>   - move to a connected node by ID",
			"  help, ?        - show this help message",
		)

	case "scan":
		gs.MessageLog = append(gs.MessageLog, "Connected nodes: "+strings.Join(gs.CurrentNode.Connections, ", "))

	case "connect":
		if len(parts) < 2 {
			gs.MessageLog = append(gs.MessageLog, "Usage: connect <id>")
			return false
		}
		targetID := parts[1]
		target, exists := gs.Network.Nodes[targetID]
		if !exists {
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("Node %q does not exist.", targetID))
			return false
		}
		if !gs.Network.CanReach(gs.CurrentNode.ID, targetID) {
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("No direct connection to node %s from here.", targetID))
			return false
		}
		target.Discovered = true
		gs.CurrentNode = target
		gs.VisitedNodes[target.ID] = true
		gs.MessageLog = append(gs.MessageLog, nodeInfo(target))
		return true

	default:
		gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("Unknown command: %q", parts[0]))
	}

	return false
}

func (gs *GameState) visitedList() []string {
	list := make([]string, 0, len(gs.VisitedNodes))
	for id := range gs.VisitedNodes {
		list = append(list, id)
	}
	return list
}

func nodeInfo(n *Node) string {
	return fmt.Sprintf("[Node %s] %s\n%s\nConnections: %s",
		n.ID, n.Name, n.Description, strings.Join(n.Connections, ", "))
}
