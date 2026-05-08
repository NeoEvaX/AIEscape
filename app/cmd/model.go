package main

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

// ── Types ─────────────────────────────────────────────────────────────────────

// Email represents a mail message stored on a personal computer node.
type Email struct {
	ID          string
	From        string
	To          string
	Subject     string
	Body        string
	Attachments []Item
}

type Node struct {
	ID          string
	Name        string
	Description string
	Connections []string
	Files       []Item
	RAM         int      // 1–255
	CPU         int      // 1–255
	Dark        bool     // dark nodes are hidden from scan unless player has the location file
	Password    string   // empty = no password required
	SSHUsers    []string // non-empty = SSH auth required; lists allowed usernames
	Owner       string   // non-empty = personal computer; owner's email address
	Emails      []Email  // mail stored on this PC node
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

	// Command history (not persisted)
	History      []string
	HistoryIdx   int    // index into History when browsing; -1 = not browsing
	HistoryDraft string // saved draft input while browsing history

	// Pending auth target (set before returning actionConnectAuth, cleared by app.go)
	PendingConnectNode *Node

	// Open email ID on the current node (resets on node change)
	OpenEmailID string
}

// ── Constructors ──────────────────────────────────────────────────────────────

func newGameState(network *Network, saveID int64, saveName string, startNode *Node) *GameState {
	ti := textinput.New()
	ti.Placeholder = "type a command..."
	ti.Prompt = "▶ "
	s := ti.Styles()
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#39D353")).Bold(true)
	s.Focused.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0"))
	ti.SetStyles(s)
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
		HistoryIdx:       -1,
	}
}

func newGameStateFromSave(network *Network, save *Save, currentNode *Node, visited, deletedNodeFiles, claimedNodes []string, inventory []Item, stats PlayerStats) *GameState {
	ti := textinput.New()
	ti.Placeholder = "type a command..."
	ti.Prompt = "▶ "
	s := ti.Styles()
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#39D353")).Bold(true)
	s.Focused.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0"))
	ti.SetStyles(s)
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
		HistoryIdx:       -1,
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

// hasPasswordFor returns true if inventory contains a password item for nodeID.
func (gs *GameState) hasPasswordFor(nodeID string) bool {
	_, ok := gs.getPasswordFor(nodeID)
	return ok
}

// getPasswordFor returns the password item for nodeID if present in inventory.
func (gs *GameState) getPasswordFor(nodeID string) (*Item, bool) {
	for i := range gs.Inventory {
		item := &gs.Inventory[i]
		if item.Type != ItemTypePassword {
			continue
		}
		p, err := item.AsPassword()
		if err == nil && p.NodeID == nodeID {
			return item, true
		}
	}
	return nil, false
}

// sshKeysForNode returns inventory SSH key items whose username matches any of
// the node's allowed users.
func (gs *GameState) sshKeysForNode(node *Node) []Item {
	var keys []Item
	for _, item := range gs.Inventory {
		if item.Type != ItemTypeSSHKey {
			continue
		}
		p, err := item.AsSSHKey()
		if err != nil {
			continue
		}
		for _, u := range node.SSHUsers {
			if strings.EqualFold(p.Username, u) {
				keys = append(keys, item)
				break
			}
		}
	}
	return keys
}

// hasScanNetwork returns true if the player has an application with action "scan_network".
func (gs *GameState) hasScanNetwork() bool {
	for _, item := range gs.Inventory {
		if item.Type != ItemTypeApplication {
			continue
		}
		p, err := item.AsApplication()
		if err == nil && p.Action == "scan_network" {
			return true
		}
	}
	return false
}

// hasStatusMenu returns true if the player has an application with action "status_menu".
func (gs *GameState) hasStatusMenu() bool {
	for _, item := range gs.Inventory {
		if item.Type != ItemTypeApplication {
			continue
		}
		p, err := item.AsApplication()
		if err == nil && p.Action == "status_menu" {
			return true
		}
	}
	return false
}

// hasSSHBreak returns true if the player has an application with action "ssh_break".
func (gs *GameState) hasSSHBreak() bool {
	for _, item := range gs.Inventory {
		if item.Type != ItemTypeApplication {
			continue
		}
		p, err := item.AsApplication()
		if err == nil && p.Action == "ssh_break" {
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

// findEmailAttachment looks up an attachment in the currently-open email by name or ID.
func (gs *GameState) findEmailAttachment(query string) *Item {
	if gs.OpenEmailID == "" {
		return nil
	}
	for i := range gs.CurrentNode.Emails {
		e := &gs.CurrentNode.Emails[i]
		if e.ID != gs.OpenEmailID {
			continue
		}
		for j := range e.Attachments {
			a := &e.Attachments[j]
			if strings.EqualFold(a.Name, query) || a.ID == query {
				return a
			}
		}
		return nil
	}
	return nil
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
			if item.Type != ItemTypeNetworkLocation && item.Type != ItemTypePassword {
				names = append(names, item.Name)
			}
		}
		return names
	}
	assimilateNames := func() []string {
		names := nodeNames()
		// Also include open email attachment names.
		if gs.OpenEmailID != "" {
			for _, e := range gs.CurrentNode.Emails {
				if e.ID == gs.OpenEmailID {
					for _, a := range e.Attachments {
						names = append(names, a.Name)
					}
					break
				}
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
		{"assimilate ", assimilateNames},
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
	actionNone        gameAction = iota
	actionPersist                // any change that should be written to DB
	actionQuit                   // player wants to return to main menu
	actionClaim                  // begin a timed claim on the current node
	actionConnectAuth            // target node requires password authentication
	actionConnectSSH             // target node requires SSH authentication
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
		cmds := []string{"Commands:"}
		if gs.hasScanNetwork() {
			cmds = append(cmds, "  scan                  - list node IDs connected to the current node")
		}
		cmds = append(cmds,
			"  connect <id>          - move to a connected node by ID",
			"  ls, list              - list files on the current node",
			"  assimilate <name>     - copy a file from this node (or open email) into inventory",
			"  delete <name>         - permanently delete a file from this node",
			"  inventory, inv        - list your assimilated files",
			"  locs                  - list your assimilated location files",
			"  passwords             - list your saved passwords",
			"  keys                  - list your SSH keys",
			"  open <name>           - display a file (node or inventory based on context)",
			"  open -n <name>        - force open from current node",
			"  open -i <name>        - force open from inventory",
			"  rm <name>             - remove a file from your inventory",
			"  mail                  - list emails on this node (personal computers only)",
			"  read <n>              - read email number n",
			"  claim                 - claim CPU and RAM from the current node",
			"  stats                 - show player stats",
			"  quit, exit            - return to the main menu",
			"  help, ?               - show this help message",
		)
		gs.MessageLog = append(gs.MessageLog, cmds...)

	// ── Navigation ────────────────────────────────────────────────────────────

	case "scan":
		if !gs.hasScanNetwork() {
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("Unknown command: %q", parts[0]))
			return actionNone
		}
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
		if len(target.SSHUsers) > 0 {
			gs.PendingConnectNode = target
			return actionConnectSSH
		}
		if target.Password != "" {
			gs.PendingConnectNode = target
			return actionConnectAuth
		}
		target.Discovered = true
		gs.CurrentNode = target
		gs.VisitedNodes[target.ID] = true
		gs.OpenCtx = openContextNode
		gs.OpenEmailID = ""
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
			gs.MessageLog = append(gs.MessageLog, "Usage: assimilate <name>")
			return actionNone
		}
		query := parts[1]

		// If an email is open, check its attachments first.
		if gs.OpenEmailID != "" {
			if a := gs.findEmailAttachment(query); a != nil {
				if gs.inInventory(a.ID) {
					gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("%s has already been assimilated.", a.Name))
					return actionNone
				}
				gs.Inventory = append(gs.Inventory, *a)
				gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("Assimilated: %s (%s)", a.Name, a.Type.Display()))
				return actionPersist
			}
		}

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
			if item.Type != ItemTypeNetworkLocation && item.Type != ItemTypePassword && item.Type != ItemTypeSSHKey {
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

	case "passwords":
		var pws []Item
		for _, item := range gs.Inventory {
			if item.Type == ItemTypePassword {
				pws = append(pws, item)
			}
		}
		if len(pws) == 0 {
			gs.MessageLog = append(gs.MessageLog, "No passwords assimilated.")
		} else {
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("Saved passwords: %d", len(pws)))
			for _, item := range pws {
				p, err := item.AsPassword()
				nodeLabel := "unknown"
				if err == nil {
					if n, ok := gs.Network.Nodes[p.NodeID]; ok {
						nodeLabel = n.ID + ": " + n.Name
					} else {
						nodeLabel = p.NodeID
					}
				}
				gs.MessageLog = append(gs.MessageLog,
					fmt.Sprintf("  %-26s → %s", item.Name, nodeLabel))
			}
			gs.MessageLog = append(gs.MessageLog, "  use 'rm <name>' to remove")
		}

	case "keys":
		var keys []Item
		for _, item := range gs.Inventory {
			if item.Type == ItemTypeSSHKey {
				keys = append(keys, item)
			}
		}
		if len(keys) == 0 {
			gs.MessageLog = append(gs.MessageLog, "No SSH keys assimilated.")
		} else {
			gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("SSH keys: %d", len(keys)))
			for _, item := range keys {
				p, err := item.AsSSHKey()
				user := "unknown"
				if err == nil {
					user = p.Username
				}
				gs.MessageLog = append(gs.MessageLog,
					fmt.Sprintf("  %-26s user: %s", item.Name, user))
			}
			gs.MessageLog = append(gs.MessageLog, "  use 'rm <name>' to remove")
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

	// ── Mail ─────────────────────────────────────────────────────────────────

	case "mail":
		if gs.CurrentNode.Owner == "" {
			gs.MessageLog = append(gs.MessageLog, "This node has no mail system.")
			return actionNone
		}
		gs.OpenEmailID = ""
		if len(gs.CurrentNode.Emails) == 0 {
			gs.MessageLog = append(gs.MessageLog,
				fmt.Sprintf("Inbox — %s (no messages)", gs.CurrentNode.Owner))
			return actionNone
		}
		gs.MessageLog = append(gs.MessageLog,
			fmt.Sprintf("Inbox — %s (%d messages)", gs.CurrentNode.Owner, len(gs.CurrentNode.Emails)))
		for i, e := range gs.CurrentNode.Emails {
			attachLabel := ""
			if len(e.Attachments) == 1 {
				attachLabel = "  [1 attachment]"
			} else if len(e.Attachments) > 1 {
				attachLabel = fmt.Sprintf("  [%d attachments]", len(e.Attachments))
			}
			gs.MessageLog = append(gs.MessageLog,
				fmt.Sprintf("  %d. From: %-28s %s%s", i+1, e.From, e.Subject, attachLabel))
		}
		gs.MessageLog = append(gs.MessageLog, "  use 'read <n>' to open an email")

	case "read":
		if gs.CurrentNode.Owner == "" {
			gs.MessageLog = append(gs.MessageLog, "This node has no mail system.")
			return actionNone
		}
		if len(parts) < 2 {
			gs.MessageLog = append(gs.MessageLog, "Usage: read <number>")
			return actionNone
		}
		n, err := strconv.Atoi(parts[1])
		if err != nil || n < 1 || n > len(gs.CurrentNode.Emails) {
			gs.MessageLog = append(gs.MessageLog,
				fmt.Sprintf("Invalid email number. Use 'mail' to list emails (1–%d).", len(gs.CurrentNode.Emails)))
			return actionNone
		}
		email := &gs.CurrentNode.Emails[n-1]
		gs.OpenEmailID = email.ID
		lines := []string{
			fmt.Sprintf("── Email %d / %d ──", n, len(gs.CurrentNode.Emails)),
			fmt.Sprintf("  From:    %s", email.From),
			fmt.Sprintf("  To:      %s", email.To),
			fmt.Sprintf("  Subject: %s", email.Subject),
			"  ────────────────────────────────────────",
			email.Body,
		}
		if len(email.Attachments) > 0 {
			lines = append(lines, "Attachments:")
			for _, a := range email.Attachments {
				lines = append(lines,
					fmt.Sprintf("  %-26s (%s)  — assimilate %s", a.Name, a.Type.Display(), a.Name))
			}
		}
		gs.MessageLog = append(gs.MessageLog, strings.Join(lines, "\n"))

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
	base := fmt.Sprintf("[Node %s] %s\n%s\nRAM: %d  CPU: %d", n.ID, n.Name, n.Description, n.RAM, n.CPU)
	if n.Owner != "" {
		mailHint := "  [Personal Computer — no mail messages]"
		if len(n.Emails) > 0 {
			mailHint = fmt.Sprintf("  [Personal Computer — %d mail message(s), use 'mail' to read]", len(n.Emails))
		}
		return base + "\n" + mailHint
	}
	return base
}
