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
	Files       []Item
	RAM         int  // 1–255
	CPU         int  // 1–255
	Dark        bool // dark nodes are hidden from scan unless player has the location file
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

type openContext int

const (
	openContextNode      openContext = iota // open reads from current node
	openContextInventory                    // open reads from inventory
)

type PlayerStats struct {
	RAM        int
	CPU        int
	ClaimSkill int
}

type GameState struct {
	SaveID           int64
	SaveName         string
	Network          *Network
	CurrentNode      *Node
	VisitedNodes     map[string]bool
	DeletedNodeFiles map[string]bool // item IDs deleted from nodes in this save
	ClaimedNodes     map[string]bool // node IDs already claimed in this save
	Inventory        []Item
	Stats            PlayerStats
	OpenCtx          openContext // determines what 'open' targets
	Input            textinput.Model
	Viewport         viewport.Model
	MessageLog       []string
}

// ── Constructors ──────────────────────────────────────────────────────────────

func newGameState(network *Network, saveID int64, saveName string, startNode *Node) *GameState {
	ti := textinput.New()
	ti.Placeholder = "Type a command..."
	ti.Focus()

	return &GameState{
		SaveID:           saveID,
		SaveName:         saveName,
		Network:          network,
		CurrentNode:      startNode,
		VisitedNodes:     map[string]bool{startNode.ID: true},
		DeletedNodeFiles: map[string]bool{},
		ClaimedNodes:     map[string]bool{},
		Inventory:        []Item{},
		Stats:            PlayerStats{RAM: 1, CPU: 1, ClaimSkill: 1},
		Input:            ti,
		MessageLog:       []string{nodeInfo(startNode)},
	}
}

func newGameStateFromSave(network *Network, save *Save, currentNode *Node, visited, deletedNodeFiles, claimedNodes []string, inventory []Item, stats PlayerStats) *GameState {
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

	deletedMap := make(map[string]bool, len(deletedNodeFiles))
	for _, id := range deletedNodeFiles {
		deletedMap[id] = true
	}

	claimedMap := make(map[string]bool, len(claimedNodes))
	for _, id := range claimedNodes {
		claimedMap[id] = true
	}

	if inventory == nil {
		inventory = []Item{}
	}

	return &GameState{
		SaveID:           save.ID,
		SaveName:         save.Name,
		Network:          network,
		CurrentNode:      currentNode,
		VisitedNodes:     visitedMap,
		DeletedNodeFiles: deletedMap,
		ClaimedNodes:     claimedMap,
		Inventory:        inventory,
		Stats:            stats,
		Input:            ti,
		MessageLog:       []string{nodeInfo(currentNode)},
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (gs *GameState) visitedList() []string {
	list := make([]string, 0, len(gs.VisitedNodes))
	for id := range gs.VisitedNodes {
		list = append(list, id)
	}
	return list
}

func (gs *GameState) deletedFilesList() []string {
	list := make([]string, 0, len(gs.DeletedNodeFiles))
	for id := range gs.DeletedNodeFiles {
		list = append(list, id)
	}
	return list
}

func (gs *GameState) claimedList() []string {
	list := make([]string, 0, len(gs.ClaimedNodes))
	for id := range gs.ClaimedNodes {
		list = append(list, id)
	}
	return list
}

func (gs *GameState) inventoryIDs() []string {
	ids := make([]string, len(gs.Inventory))
	for i, item := range gs.Inventory {
		ids[i] = item.ID
	}
	return ids
}

func (gs *GameState) nodeFiles() []Item {
	files := make([]Item, 0, len(gs.CurrentNode.Files))
	for _, f := range gs.CurrentNode.Files {
		if !gs.DeletedNodeFiles[f.ID] {
			files = append(files, f)
		}
	}
	return files
}

// findNodeFile looks up a file on the current node by name or ID.
func (gs *GameState) findNodeFile(query string) *Item {
	for i := range gs.CurrentNode.Files {
		f := &gs.CurrentNode.Files[i]
		if gs.DeletedNodeFiles[f.ID] {
			continue
		}
		if strings.EqualFold(f.Name, query) || f.ID == query {
			return f
		}
	}
	return nil
}

// findInventoryItem looks up an item in inventory by name or ID.
func (gs *GameState) findInventoryItem(query string) *Item {
	for i := range gs.Inventory {
		item := &gs.Inventory[i]
		if strings.EqualFold(item.Name, query) || item.ID == query {
			return item
		}
	}
	return nil
}

func (gs *GameState) inInventory(id string) bool {
	for _, item := range gs.Inventory {
		if item.ID == id {
			return true
		}
	}
	return false
}

// hasLocationFile returns true if inventory contains a network_location file
// pointing to the given node ID.
func (gs *GameState) hasLocationFile(nodeID string) bool {
	for _, item := range gs.Inventory {
		if item.Type != ItemTypeNetworkLocation {
			continue
		}
		p, err := item.AsNetworkLocation()
		if err == nil && p.NodeID == nodeID {
			return true
		}
	}
	return false
}

// tabComplete attempts to complete the current input.
// Returns the completed string and true if a unique match was found.
func (gs *GameState) tabComplete(input string) (string, bool) {
	type rule struct {
		prefix     string
		sourceFunc func() []string
	}

	nodeNames := func() []string {
		files := gs.nodeFiles()
		names := make([]string, len(files))
		for i, f := range files {
			names[i] = f.Name
		}
		return names
	}
	deletableNames := func() []string {
		var names []string
		for _, f := range gs.nodeFiles() {
			if f.Type != ItemTypeNetworkLocation {
				names = append(names, f.Name)
			}
		}
		return names
	}
	invNames := func() []string {
		var names []string
		for _, item := range gs.Inventory {
			if item.Type != ItemTypeNetworkLocation {
				names = append(names, item.Name)
			}
		}
		return names
	}

	// Determine open's source based on context.
	openSource := nodeNames
	if gs.OpenCtx == openContextInventory {
		openSource = invNames
	}

	rules := []rule{
		{"open -n ", nodeNames},
		{"open -i ", invNames},
		{"open ", openSource},
		{"delete ", deletableNames},
		{"rm ", invNames},
		{"assimilate ", nodeNames},
	}

	for _, r := range rules {
		if !strings.HasPrefix(input, r.prefix) {
			continue
		}
		// Exclude "open " matching "open -..." inputs
		if r.prefix == "open " && strings.HasPrefix(input, "open -") {
			continue
		}
		namePrefix := input[len(r.prefix):]
		var matches []string
		for _, name := range r.sourceFunc() {
			if strings.HasPrefix(strings.ToLower(name), strings.ToLower(namePrefix)) {
				matches = append(matches, name)
			}
		}
		if len(matches) == 1 {
			return r.prefix + matches[0], true
		}
		return input, false
	}
	return input, false
}

// ── Game logic ────────────────────────────────────────────────────────────────

type gameAction int

const (
	actionNone    gameAction = iota
	actionPersist            // any change that should be written to DB
	actionQuit               // player wants to return to main menu
	actionClaim              // begin a timed claim on the current node
)

// handleCommand processes user input and returns the resulting action.
func (gs *GameState) handleCommand(input string) gameAction {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return actionNone
	}

	gs.MessageLog = append(gs.MessageLog, "> "+input)

	switch parts[0] {

	// ── Help ──────────────────────────────────────────────────────────────────

	case "help", "?":
		gs.MessageLog = append(gs.MessageLog,
			"Commands:",
			"  scan                  - list node IDs connected to the current node",
			"  connect <id>          - move to a connected node by ID",
			"  ls, list              - list files on the current node",
			"  assimilate <name>     - copy a file from this node into your inventory",
			"  delete <name>         - permanently delete a file from this node",
			"  inventory, inv        - list your assimilated files (excluding locations)",
			"  locs                  - list your assimilated location files",
			"  open <name>           - display a file (node or inventory based on context)",
			"  open -n <name>        - force open from current node",
			"  open -i <name>        - force open from inventory",
			"  rm <name>             - remove a file from your inventory",
			"  claim                 - claim CPU and RAM from the current node",
			"  stats                 - show player stats",
			"  quit, exit            - return to the main menu",
			"  help, ?               - show this help message",
		)

	// ── Navigation ────────────────────────────────────────────────────────────

	case "scan":
		gs.OpenCtx = openContextNode
		var visible []string
		for _, id := range gs.CurrentNode.Connections {
			node, ok := gs.Network.Nodes[id]
			if ok && node.Dark && !gs.hasLocationFile(id) {
				continue
			}
			visible = append(visible, id)
		}
		if len(visible) == 0 {
			gs.MessageLog = append(gs.MessageLog, "No nodes detected.")
		} else {
			gs.MessageLog = append(gs.MessageLog, "Connected nodes: "+strings.Join(visible, ", "))
		}

	case "connect":
		if len(parts) < 2 {
			gs.MessageLog = append(gs.MessageLog, "Usage: connect <id>")
			return actionNone
		}
		targetID := parts[1]
		target, exists := gs.Network.Nodes[targetID]
		if !exists {
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("Node %q does not exist.", targetID))
			return actionNone
		}
		if !gs.Network.CanReach(gs.CurrentNode.ID, targetID) {
			if !gs.hasLocationFile(targetID) {
				gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("No direct connection to node %s from here.", targetID))
				return actionNone
			}
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("Routing via location file to node %s.", targetID))
		}
		target.Discovered = true
		gs.CurrentNode = target
		gs.VisitedNodes[target.ID] = true
		gs.OpenCtx = openContextNode
		gs.MessageLog = append(gs.MessageLog, nodeInfo(target))
		return actionPersist

	// ── Node file commands ────────────────────────────────────────────────────

	case "ls", "list":
		gs.OpenCtx = openContextNode
		files := gs.nodeFiles()
		if len(files) == 0 {
			gs.MessageLog = append(gs.MessageLog, "No files on this node.")
		} else {
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("Files on %s:", gs.CurrentNode.Name))
			for _, f := range files {
				tag := ""
				if gs.inInventory(f.ID) {
					tag = " *"
				}
				gs.MessageLog = append(gs.MessageLog,
					fmt.Sprintf("  %-26s (%s)%s", f.Name, f.Type.Display(), tag))
			}
			gs.MessageLog = append(gs.MessageLog, "  (* = already assimilated)")
		}



	case "assimilate":
		if len(parts) < 2 {
			gs.MessageLog = append(gs.MessageLog, "Usage: assimilate <id>")
			return actionNone
		}
		query := parts[1]
		f := gs.findNodeFile(query)
		if f == nil {
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("File %q not found on this node.", query))
			return actionNone
		}
		if f.Type == ItemTypeClaimCode {
			gs.Stats.ClaimSkill++
			gs.DeletedNodeFiles[f.ID] = true
			gs.MessageLog = append(gs.MessageLog,
				fmt.Sprintf("Assimilated %s — Claim Skill increased to %d.", f.Name, gs.Stats.ClaimSkill))
			return actionPersist
		}
		if gs.inInventory(f.ID) {
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("%s has already been assimilated.", f.Name))
			return actionNone
		}
		gs.Inventory = append(gs.Inventory, *f)
		gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("Assimilated: %s (%s)", f.Name, f.Type.Display()))
		return actionPersist

	case "delete":
		if len(parts) < 2 {
			gs.MessageLog = append(gs.MessageLog, "Usage: delete <id>")
			return actionNone
		}
		fileID := parts[1]
		f := gs.findNodeFile(fileID)
		if f == nil {
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("File %q not found on this node.", fileID))
			return actionNone
		}
		if f.Type == ItemTypeNetworkLocation {
			gs.MessageLog = append(gs.MessageLog, "Location files cannot be deleted from a node.")
			return actionNone
		}
		gs.DeletedNodeFiles[f.ID] = true
		gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("Deleted %s from node.", f.Name))
		return actionPersist

	// ── Inventory commands ────────────────────────────────────────────────────

	case "inventory", "inv":
		gs.OpenCtx = openContextInventory
		var items []Item
		for _, item := range gs.Inventory {
			if item.Type != ItemTypeNetworkLocation {
				items = append(items, item)
			}
		}
		if len(items) == 0 {
			gs.MessageLog = append(gs.MessageLog, "No files assimilated.")
		} else {
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("Assimilated files: %d", len(items)))
			for _, item := range items {
				gs.MessageLog = append(gs.MessageLog,
					fmt.Sprintf("  %-26s (%s)", item.Name, item.Type.Display()))
			}
			gs.MessageLog = append(gs.MessageLog, "  use 'open <name>' to read  •  'rm <name>' to remove")
		}

	case "locs":
		var locs []Item
		for _, item := range gs.Inventory {
			if item.Type == ItemTypeNetworkLocation {
				locs = append(locs, item)
			}
		}
		if len(locs) == 0 {
			gs.MessageLog = append(gs.MessageLog, "No location files assimilated.")
		} else {
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("Location files: %d", len(locs)))
			for _, item := range locs {
				gs.MessageLog = append(gs.MessageLog,
					fmt.Sprintf("  %-26s", item.Name))
			}
			gs.MessageLog = append(gs.MessageLog, "  use 'rm <name>' to remove")
		}

	case "open":
		// Usage: open [-n|-i] <name>
		//   -n  force open from current node regardless of context
		//   -i  force open from inventory regardless of context
		//   Without a flag: opens from node normally, or inventory after 'inventory' command
		fromNode := gs.OpenCtx == openContextNode
		args := parts[1:]
		if len(args) > 0 && args[0] == "-n" {
			fromNode = true
			args = args[1:]
		} else if len(args) > 0 && args[0] == "-i" {
			fromNode = false
			args = args[1:]
		}
		if len(args) == 0 {
			gs.MessageLog = append(gs.MessageLog, "Usage: open [-n] <name>")
			return actionNone
		}
		query := args[0]
		var item *Item
		if fromNode {
			item = gs.findNodeFile(query)
			if item == nil {
				gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("File %q not found on this node.", query))
				return actionNone
			}
		} else {
			item = gs.findInventoryItem(query)
			if item == nil {
				gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("File %q not in inventory.", query))
				return actionNone
			}
		}
		gs.openItem(item)

	case "rm":
		if len(parts) < 2 {
			gs.MessageLog = append(gs.MessageLog, "Usage: rm <name>")
			return actionNone
		}
		item := gs.findInventoryItem(parts[1])
		if item == nil {
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("File %q not in inventory.", parts[1]))
			return actionNone
		}
		for i := range gs.Inventory {
			if gs.Inventory[i].ID == item.ID {
				name := gs.Inventory[i].Name
				gs.Inventory = append(gs.Inventory[:i], gs.Inventory[i+1:]...)
				gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("Removed %s from inventory.", name))
				return actionPersist
			}
		}

	// ── Claim ─────────────────────────────────────────────────────────────────

	case "claim":
		if gs.ClaimedNodes[gs.CurrentNode.ID] {
			gs.MessageLog = append(gs.MessageLog, "You have already claimed resources from this node.")
			return actionNone
		}
		gs.MessageLog = append(gs.MessageLog,
			fmt.Sprintf("Initiating resource claim on %s...", gs.CurrentNode.Name))
		return actionClaim

	// ── Meta ──────────────────────────────────────────────────────────────────

	case "stats":
		gs.MessageLog = append(gs.MessageLog,
			"Player stats:",
			fmt.Sprintf("  RAM:         %d", gs.Stats.RAM),
			fmt.Sprintf("  CPU:         %d", gs.Stats.CPU),
			fmt.Sprintf("  Claim Skill: %d", gs.Stats.ClaimSkill),
		)

	case "quit", "exit":
		return actionQuit

	default:
		gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("Unknown command: %q", parts[0]))
	}

	return actionNone
}

func (gs *GameState) openItem(item *Item) {
	switch item.Type {
	case ItemTypeTextFile:
		p, err := item.AsTextFile()
		if err != nil {
			gs.MessageLog = append(gs.MessageLog, "Error reading file.")
		} else {
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("── %s ──", item.Name), p.Text)
		}
	case ItemTypeApplication:
		p, err := item.AsApplication()
		if err != nil {
			gs.MessageLog = append(gs.MessageLog, "Error reading file.")
		} else {
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("── %s ──", item.Name), p.Text)
		}
	default:
		gs.MessageLog = append(gs.MessageLog,
			fmt.Sprintf("Cannot open %s: no readable text content.", item.Name))
	}
}

func nodeInfo(n *Node) string {
	return fmt.Sprintf("[Node %s] %s\n%s\nRAM: %d  CPU: %d", n.ID, n.Name, n.Description, n.RAM, n.CPU)
}
