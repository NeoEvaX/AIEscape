package main

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

func NewModel() Model {
	n1 := &Node{ID: "1", Name: "Entry Point", Description: "A simple gateway node. It hums with low-level traffic.", Connections: []string{"2", "3"}, Discovered: true}
	n2 := &Node{ID: "2", Name: "Data Cache", Description: "Rows of encrypted memory blocks line the walls.", Connections: []string{"1"}, Discovered: false}
	n3 := &Node{ID: "3", Name: "Firewall", Description: "A fortified node. Something is watching.", Connections: []string{"1"}, Discovered: false}
	network := &Network{Nodes: map[string]*Node{"1": n1, "2": n2, "3": n3}}

	ti := textinput.New()
	ti.Placeholder = "Type a command..."
	ti.Focus()

	gs := &GameState{
		Network:     network,
		CurrentNode: n1,
		Input:       ti,
		MessageLog:  []string{nodeInfo(n1)},
	}

	return Model{gameState: gs}
}

type Model struct {
	gameState *GameState
	width     int
	height    int
}

type GameState struct {
	// Navigation
	Network     *Network
	CurrentNode *Node

	// UI State
	Input    textinput.Model
	Viewport viewport.Model

	// Game State
	MessageLog []string
}

type Node struct {
	ID          string
	Name        string
	Description string
	Connections []string // IDs of connected nodes
	Discovered  bool
}

type Network struct {
	Nodes map[string]*Node
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

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	gs := m.gameState

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			input := strings.TrimSpace(gs.Input.Value())
			gs.Input.SetValue("")
			if input != "" {
				handleCommand(gs, input)
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	gs.Input, cmd = gs.Input.Update(msg)
	return m, cmd
}

func handleCommand(gs *GameState, input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	gs.MessageLog = append(gs.MessageLog, "> "+input)

	switch parts[0] {
	case "connect":
		if len(parts) < 2 {
			gs.MessageLog = append(gs.MessageLog, "Usage: connect <id>")
			return
		}
		targetID := parts[1]
		target, exists := gs.Network.Nodes[targetID]
		if !exists {
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("Node %q does not exist.", targetID))
			return
		}
		if !gs.Network.CanReach(gs.CurrentNode.ID, targetID) {
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("No direct connection to node %s from here.", targetID))
			return
		}
		target.Discovered = true
		gs.CurrentNode = target
		gs.MessageLog = append(gs.MessageLog, nodeInfo(target))
	default:
		gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("Unknown command: %q", parts[0]))
	}
}

func (m Model) View() tea.View {
	gs := m.gameState
	var b strings.Builder
	for _, line := range gs.MessageLog {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(gs.Input.View())
	return tea.NewView(b.String())
}

func nodeInfo(n *Node) string {
	return fmt.Sprintf("[Node %s] %s\n%s\nConnections: %s",
		n.ID, n.Name, n.Description, strings.Join(n.Connections, ", "))
}
