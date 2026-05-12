package main

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

// gameStartTime is the in-game clock start for every new save.
var gameStartTime = time.Date(2026, 4, 19, 13, 0, 0, 0, time.UTC)

// ── Types ─────────────────────────────────────────────────────────────────────

// Email represents a mail message stored on a personal computer node.
type Email struct {
	ID             string
	From           string
	To             string
	Subject        string
	Body           string
	Attachments    []Item
	AvailableFrom  time.Time
	AvailableUntil time.Time
}

func (e *Email) IsAvailable(gameTime time.Time) bool {
	return isAvailableAt(e.AvailableFrom, e.AvailableUntil, gameTime)
}

// NodeSchedule defines a recurring weekly availability window.
// If To <= From the window crosses midnight (e.g. From=21 To=6 means 9 pm – 6 am).
type NodeSchedule struct {
	Days []time.Weekday
	From int // hour 0–23 the window opens
	To   int // hour 0–23 the window closes
}

// IsActive returns true if t falls within any scheduled window.
func (s *NodeSchedule) IsActive(t time.Time) bool {
	hour := t.Hour()
	wd := t.Weekday()

	if s.To <= s.From {
		// Window crosses midnight (e.g. 21:00 → 06:00).
		// Active if: hour >= From on a scheduled day
		//         OR hour < To on the day after a scheduled day.
		if hour >= s.From {
			return schedDayMatch(s.Days, wd)
		}
		if hour < s.To {
			prev := time.Weekday((int(wd) + 6) % 7)
			return schedDayMatch(s.Days, prev)
		}
		return false
	}
	// Normal window (e.g. 09:00 → 17:00).
	return hour >= s.From && hour < s.To && schedDayMatch(s.Days, wd)
}

func schedDayMatch(days []time.Weekday, d time.Weekday) bool {
	for _, day := range days {
		if day == d {
			return true
		}
	}
	return false
}

type Node struct {
	ID             string
	Name           string
	Description    string
	Connections    []string
	Files          []Item
	CPU            int      // 1–255
	Dark           bool     // dark nodes are hidden from scan unless player has the location file
	Network        string   // logical network island ID; empty = "default"
	Password       string   // empty = no password required
	SSHUsers       []string // non-empty = SSH auth required; lists allowed usernames
	Owner          string   // non-empty = personal computer; owner's email address
	Emails         []Email  // mail stored on this PC node
	Schedule       *NodeSchedule
	AvailableFrom  time.Time
	AvailableUntil time.Time
	Discovered     bool
}

// nodeNetwork returns the canonical network ID for a node.
// An empty Network field is treated as "default".
func nodeNetwork(n *Node) string {
	if n.Network == "" {
		return "default"
	}
	return n.Network
}

// sameNetwork returns true if two nodes belong to the same logical network island.
func sameNetwork(a, b *Node) bool {
	return nodeNetwork(a) == nodeNetwork(b)
}

func (n *Node) IsAvailable(gameTime time.Time) bool {
	if !isAvailableAt(n.AvailableFrom, n.AvailableUntil, gameTime) {
		return false
	}
	if n.Schedule != nil && !n.Schedule.IsActive(gameTime) {
		return false
	}
	return true
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
	MessageLog       []string

	// Command history (not persisted)
	History      []string
	HistoryIdx   int    // index into History when browsing; -1 = not browsing
	HistoryDraft string // saved draft input while browsing history

	// Pending auth target (set before returning actionConnectAuth, cleared by app.go)
	PendingConnectNode *Node

	// Open email ID on the current node (resets on node change)
	OpenEmailID string

	// In-game clock
	GameTime time.Time

	// Story state (not persisted as maps — serialised via helpers)
	SeenEvents      map[string]bool
	ReadEmails      map[string]bool
	ConnectCount    int
	AssimilateCount int
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
		Stats:            PlayerStats{CPU: 1, ClaimSkill: 1},
		Input:            ti,
		MessageLog:       []string{nodeInfo(startNode)},
		HistoryIdx:       -1,
		GameTime:         gameStartTime,
		SeenEvents:       map[string]bool{},
		ReadEmails:       map[string]bool{},
	}
}

func newGameStateFromSave(network *Network, save *Save, currentNode *Node, visited, deletedNodeFiles, claimedNodes []string, inventory []Item, stats PlayerStats, gameTime time.Time, seenEvents, readEmails []string, connectCount, assimilateCount int) *GameState {
	ti := textinput.New()
	ti.Placeholder = "type a command..."
	ti.Prompt = "▶ "
	s := ti.Styles()
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#39D353")).Bold(true)
	s.Focused.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0"))
	ti.SetStyles(s)
	ti.Focus()

	visitedMap := boolMapFromSlice(visited)
	for id := range visitedMap {
		if node, ok := network.Nodes[id]; ok {
			node.Discovered = true
		}
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
		DeletedNodeFiles: boolMapFromSlice(deletedNodeFiles),
		ClaimedNodes:     boolMapFromSlice(claimedNodes),
		Inventory:        inventory,
		Stats:            stats,
		Input:            ti,
		MessageLog:       []string{nodeInfo(currentNode)},
		HistoryIdx:       -1,
		GameTime:         gameTime,
		SeenEvents:       boolMapFromSlice(seenEvents),
		ReadEmails:       boolMapFromSlice(readEmails),
		ConnectCount:     connectCount,
		AssimilateCount:  assimilateCount,
	}
}

// ── Save snapshot ─────────────────────────────────────────────────────────────

// SaveData is a flat, serializable snapshot of the persisted fields in GameState.
// It is the only type that crosses the seam between GameState and Database.
type SaveData struct {
	CurrentNodeID   string
	Visited         []string
	DeletedFiles    []string
	InventoryIDs    []string
	ClaimedNodes    []string
	SeenEvents      []string
	ReadEmails      []string
	Stats           PlayerStats
	GameTime        time.Time
	ConnectCount    int
	AssimilateCount int
}

// Snapshot extracts all persisted fields from gs into a SaveData.
func (gs *GameState) Snapshot() SaveData {
	return SaveData{
		CurrentNodeID:   gs.CurrentNode.ID,
		Visited:         boolMapKeys(gs.VisitedNodes),
		DeletedFiles:    boolMapKeys(gs.DeletedNodeFiles),
		InventoryIDs:    gs.inventoryIDs(),
		ClaimedNodes:    boolMapKeys(gs.ClaimedNodes),
		SeenEvents:      boolMapKeys(gs.SeenEvents),
		ReadEmails:      boolMapKeys(gs.ReadEmails),
		Stats:           gs.Stats,
		GameTime:        gs.GameTime,
		ConnectCount:    gs.ConnectCount,
		AssimilateCount: gs.AssimilateCount,
	}
}

// boolMapKeys extracts the keys of a map[string]bool into a slice.
func boolMapKeys(m map[string]bool) []string {
	list := make([]string, 0, len(m))
	for id := range m {
		list = append(list, id)
	}
	return list
}

// boolMapFromSlice converts a string slice into a map[string]bool.
func boolMapFromSlice(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// ── Helpers ───────────────────────────────────────────────────────────────────

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
		if gs.DeletedNodeFiles[f.ID] {
			continue
		}
		if !f.IsAvailable(gs.GameTime) {
			continue
		}
		files = append(files, f)
	}
	return files
}

// findNodeFile looks up a visible (non-deleted, available) file on the current node.
func (gs *GameState) findNodeFile(query string) *Item {
	for i := range gs.CurrentNode.Files {
		f := &gs.CurrentNode.Files[i]
		if gs.DeletedNodeFiles[f.ID] {
			continue
		}
		if !f.IsAvailable(gs.GameTime) {
			continue
		}
		if strings.EqualFold(f.Name, query) || f.ID == query {
			return f
		}
	}
	return nil
}

// availableEmails returns the emails on the current node visible at the current game time.
func (gs *GameState) availableEmails() []Email {
	var result []Email
	for _, e := range gs.CurrentNode.Emails {
		if e.IsAvailable(gs.GameTime) {
			result = append(result, e)
		}
	}
	return result
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

// hasScanNetwork returns true if the player has the base scan_network.app.
func (gs *GameState) hasScanNetwork() bool {
	return gs.hasAppAction("scan_network")
}

// hasScanNetworkV2 returns true if the player has scan_network_v2.app.
func (gs *GameState) hasScanNetworkV2() bool {
	return gs.hasAppAction("scan_network_v2")
}

// hasScanNetworkV3 returns true if the player has scan_network_v3.app.
func (gs *GameState) hasScanNetworkV3() bool {
	return gs.hasAppAction("scan_network_v3")
}

// canScan returns true if the player has any version of the scan app.
func (gs *GameState) canScan() bool {
	return gs.hasScanNetwork() || gs.hasScanNetworkV2() || gs.hasScanNetworkV3()
}

// hasAppAction returns true if inventory contains an application with the given action.
func (gs *GameState) hasAppAction(action string) bool {
	for _, item := range gs.Inventory {
		if item.Type != ItemTypeApplication {
			continue
		}
		p, err := item.AsApplication()
		if err == nil && p.Action == action {
			return true
		}
	}
	return false
}

// visibleConnections returns the nodes connected to node that the player can see
// at the current game time. Cross-network nodes in the connections list are
// included — they appear in scan but require 'bridge' to enter.
func (gs *GameState) visibleConnections(node *Node) []*Node {
	var result []*Node
	for _, id := range node.Connections {
		n, ok := gs.Network.Nodes[id]
		if !ok {
			continue
		}
		if n.Dark && !gs.hasLocationFile(id) {
			continue
		}
		if !n.IsAvailable(gs.GameTime) {
			continue
		}
		result = append(result, n)
	}
	return result
}

// nodeScanTags returns a short tag string for display in scan output.
func (gs *GameState) nodeScanTags(n *Node) string {
	var tags []string
	if !sameNetwork(gs.CurrentNode, n) {
		tags = append(tags, "⊗"+nodeNetwork(n))
	}
	if n.ID == gs.CurrentNode.ID {
		tags = append(tags, "◈")
	} else if gs.VisitedNodes[n.ID] {
		tags = append(tags, "✓")
	}
	if n.Password != "" || len(n.SSHUsers) > 0 {
		tags = append(tags, "⚿")
	}
	if len(tags) == 0 {
		return ""
	}
	return "  " + strings.Join(tags, " ")
}

// renderScanTree renders a tree view of nodes reachable within `depth` hops.
// depth=1 shows direct connections; depth=2 also shows their connections.
func (gs *GameState) renderScanTree(depth int) []string {
	cur := gs.CurrentNode
	children := gs.visibleConnections(cur)

	// Root box — width adapts to node label length.
	rootLabel := fmt.Sprintf("◈  [%s]  %s", cur.ID, cur.Name)
	inner := len(rootLabel) + 4
	lines := []string{
		"  ┌" + strings.Repeat("─", inner) + "┐",
		"  │  " + rootLabel + "  │",
		"  └" + strings.Repeat("─", inner) + "┘",
	}

	if len(children) == 0 {
		lines = append(lines, "       (no connected nodes visible)")
		return lines
	}

	lines = append(lines, "       │")

	for i, child := range children {
		isLast := i == len(children)-1
		conn := "├── "
		if isLast {
			conn = "└── "
		}
		lines = append(lines, fmt.Sprintf("       %s[%s]  %s%s",
			conn, child.ID, child.Name, gs.nodeScanTags(child)))

		if depth < 2 {
			continue
		}

		grandchildren := gs.visibleConnections(child)
		// Vertical continuation for the parent branch.
		contPrefix := "       │    "
		if isLast {
			contPrefix = "            "
		}
		for j, gc := range grandchildren {
			isLastGC := j == len(grandchildren)-1
			gcConn := "├── "
			if isLastGC {
				gcConn = "└── "
			}
			lines = append(lines, fmt.Sprintf("%s%s[%s]  %s%s",
				contPrefix, gcConn, gc.ID, gc.Name, gs.nodeScanTags(gc)))
		}
	}

	return lines
}

// hasStatusMenu returns true if the player has an application with action "status_menu".
func (gs *GameState) hasStatusMenu() bool { return gs.hasAppAction("status_menu") }

// hasSSHBreak returns true if the player has an application with action "ssh_break".
func (gs *GameState) hasSSHBreak() bool { return gs.hasAppAction("ssh_break") }

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

// hasBridgeTo returns true if inventory contains a network_bridge item whose
// FromNetwork and ToNetwork match the given pair exactly (one-directional).
func (gs *GameState) hasBridgeTo(fromNet, toNet string) bool {
	for _, item := range gs.Inventory {
		if item.Type != ItemTypeNetworkBridge {
			continue
		}
		p, err := item.AsNetworkBridge()
		if err != nil {
			continue
		}
		if p.FromNetwork == fromNet && p.ToNetwork == toNet {
			return true
		}
	}
	return false
}

// findEmailAttachment looks up an attachment in the currently-open email by name or ID.
// Only searches emails visible at the current game time.
func (gs *GameState) findEmailAttachment(query string) *Item {
	if gs.OpenEmailID == "" {
		return nil
	}
	for i := range gs.CurrentNode.Emails {
		e := &gs.CurrentNode.Emails[i]
		if e.ID != gs.OpenEmailID || !e.IsAvailable(gs.GameTime) {
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

// tabCompleteCommand completes a bare command name prefix (no space in input yet).
// Appends a trailing space so the cursor is ready for arguments.
func (gs *GameState) tabCompleteCommand(prefix string) (string, bool) {
	if prefix == "" {
		return prefix, false
	}
	commands := []string{
		"connect", "ls", "list", "assimilate", "delete",
		"inventory", "inv", "locs", "passwords", "keys",
		"open", "rm", "mail", "read", "claim", "stats",
		"quit", "exit", "help",
	}
	if gs.canScan() {
		commands = append([]string{"scan"}, commands...)
	}
	lower := strings.ToLower(prefix)
	var matches []string
	for _, cmd := range commands {
		if strings.HasPrefix(cmd, lower) {
			matches = append(matches, cmd)
		}
	}
	if len(matches) == 1 {
		return matches[0] + " ", true
	}
	return prefix, false
}

// tabComplete attempts to complete the current input.
// Returns the completed string and true if a unique match was found.
func (gs *GameState) tabComplete(input string) (string, bool) {
	// No space yet — complete the command name.
	if !strings.Contains(input, " ") {
		return gs.tabCompleteCommand(input)
	}

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

func nodeInfo(n *Node) string {
	base := fmt.Sprintf("[Node %s] %s\n%s\nCPU: %d", n.ID, n.Name, n.Description, n.CPU)
	if n.Owner != "" {
		mailHint := "  [Personal Computer — no mail messages]"
		if len(n.Emails) > 0 {
			mailHint = fmt.Sprintf("  [Personal Computer — %d mail message(s), use 'mail' to read]", len(n.Emails))
		}
		return base + "\n" + mailHint
	}
	return base
}
