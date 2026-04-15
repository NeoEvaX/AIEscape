package main

import (
	"fmt"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

func NewModel() Model {
	n1 := Node{ID: "1", Name: "Node1", Description: "Node One", Connections: []string{"2"}, Discovered: false}
	n2 := Node{ID: "2", Name: "Node2", Description: "Node Two", Connections: []string{"1"}, Discovered: false}
	n3 := Node{ID: "3", Name: "Node3", Description: "Node Three", Connections: []string{"4"}, Discovered: false}
	network := Network{Nodes: map[string]*Node{"1": &n1, "2": &n2, "3": &n3}}
	gs := GameState{Network: &network, CurrentNode: &n1}
	m := Model{gameState: &gs}
	return m
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
	// ActivePane Pane
	Input    textinput.Model
	Viewport viewport.Model

	// Game State
	// Player     Player
	MessageLog []string
}

type Node struct {
	ID          string
	Name        string
	Description string
	Connections []string // IDs of connected nodes
	// Services    []Service // what can you *do* here
	Discovered bool
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
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// commands := make([]tea.Cmd, 0)
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		default:
			return m, nil
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	s := fmt.Sprintf("\n Press q to quit")
	return tea.NewView(s)
}
