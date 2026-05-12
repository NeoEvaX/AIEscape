package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ── Test helpers ──────────────────────────────────────────────────────────────

// mkNode builds a Node with sensible defaults. Apply option funcs to customise.
func mkNode(id string, opts ...func(*Node)) *Node {
	n := &Node{
		ID:    id,
		Name:  "Node " + id,
		CPU:   10,
		Files: []Item{},
	}
	for _, o := range opts {
		o(n)
	}
	return n
}

func withConns(ids ...string) func(*Node) {
	return func(n *Node) { n.Connections = ids }
}
func withNet(net string) func(*Node) {
	return func(n *Node) { n.Network = net }
}
func withDark(n *Node)                         { n.Dark = true }
func withPW(pw string) func(*Node)             { return func(n *Node) { n.Password = pw } }
func withSSH(users ...string) func(*Node)      { return func(n *Node) { n.SSHUsers = users } }
func withNodeFiles(files ...Item) func(*Node)  { return func(n *Node) { n.Files = files } }
func withAvailFrom(t time.Time) func(*Node)    { return func(n *Node) { n.AvailableFrom = t } }

// mkNet assembles a Network whose StartNodeID is the first node's ID.
func mkNet(nodes ...*Node) *Network {
	m := make(map[string]*Node, len(nodes))
	for _, n := range nodes {
		m[n.ID] = n
	}
	start := ""
	if len(nodes) > 0 {
		start = nodes[0].ID
	}
	return &Network{Nodes: m, StartNodeID: start}
}

// mkItem creates an Item with a JSON-marshalled payload.
func mkItem(id, name string, typ ItemType, payload interface{}) Item {
	raw, _ := json.Marshal(payload)
	return Item{ID: id, Name: name, Type: typ, Payload: raw}
}

// mkState builds a minimal GameState. Apply option funcs to customise.
func mkState(net *Network, cur *Node, opts ...func(*GameState)) *GameState {
	gs := &GameState{
		Network:          net,
		CurrentNode:      cur,
		VisitedNodes:     map[string]bool{cur.ID: true},
		DeletedNodeFiles: map[string]bool{},
		ClaimedNodes:     map[string]bool{},
		Inventory:        []Item{},
		MessageLog:       []string{},
		HistoryIdx:       -1,
		GameTime:         gameStartTime,
		SeenEvents:       map[string]bool{},
		ReadEmails:       map[string]bool{},
		Stats:            PlayerStats{CPU: 1, ClaimSkill: 1},
	}
	for _, o := range opts {
		o(gs)
	}
	return gs
}

func withInv(items ...Item) func(*GameState) {
	return func(gs *GameState) { gs.Inventory = items }
}
func withTime(t time.Time) func(*GameState) {
	return func(gs *GameState) { gs.GameTime = t }
}

// scanApp returns an application Item that unlocks the scan command.
func scanApp() Item {
	return mkItem("app-scan", "scan.app", ItemTypeApplication,
		ApplicationPayload{Text: "scan", Action: "scan_network"})
}

// bridgeItem returns a network_bridge Item for the given network pair.
func bridgeItem(from, to string) Item {
	return mkItem("bridge-"+from+"-"+to, "bridge.bin", ItemTypeNetworkBridge,
		NetworkBridgePayload{FromNetwork: from, ToNetwork: to})
}

// logContains returns true if any MessageLog entry contains substr.
func logContains(gs *GameState, substr string) bool {
	for _, line := range gs.MessageLog {
		if strings.Contains(line, substr) {
			return true
		}
	}
	return false
}

// runCmd calls handleCommand, appends returned lines to gs.MessageLog, and returns the action.
func runCmd(gs *GameState, input string) gameAction {
	lines, action := handleCommand(gs, input)
	gs.MessageLog = append(gs.MessageLog, lines...)
	return action
}

// ── isAvailableAt ─────────────────────────────────────────────────────────────

func TestIsAvailableAt(t *testing.T) {
	now := time.Date(2026, 4, 19, 13, 0, 0, 0, time.UTC)
	before := now.Add(-time.Hour)
	after := now.Add(time.Hour)
	zero := time.Time{}

	cases := []struct {
		from, until time.Time
		want        bool
	}{
		{zero, zero, true},          // no bounds — always available
		{before, zero, true},        // from in past, no until
		{after, zero, false},        // from in future
		{zero, after, true},         // no from, until in future
		{zero, before, false},       // no from, until in past
		{before, after, true},       // within range
		{after, after.Add(time.Hour), false}, // entirely in future
	}
	for _, c := range cases {
		got := isAvailableAt(c.from, c.until, now)
		if got != c.want {
			t.Errorf("isAvailableAt(from=%v, until=%v) = %v; want %v",
				c.from, c.until, got, c.want)
		}
	}
}

// ── NodeSchedule.IsActive ─────────────────────────────────────────────────────

// gameStartTime is 2026-04-19 13:00 UTC, which is a Sunday.
// We use offsets from there for schedule tests.

func TestNodeSchedule_NormalWindow_Inside(t *testing.T) {
	sched := &NodeSchedule{
		Days: []time.Weekday{time.Monday, time.Wednesday},
		From: 9,
		To:   17,
	}
	// Monday 12:00
	monday := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	if !sched.IsActive(monday) {
		t.Error("expected active on Monday at 12:00; got inactive")
	}
}

func TestNodeSchedule_NormalWindow_Outside_Hour(t *testing.T) {
	sched := &NodeSchedule{
		Days: []time.Weekday{time.Monday},
		From: 9,
		To:   17,
	}
	// Monday 20:00 — outside 09-17
	monday20 := time.Date(2026, 4, 20, 20, 0, 0, 0, time.UTC)
	if sched.IsActive(monday20) {
		t.Error("expected inactive on Monday at 20:00; got active")
	}
}

func TestNodeSchedule_NormalWindow_Outside_Day(t *testing.T) {
	sched := &NodeSchedule{
		Days: []time.Weekday{time.Monday},
		From: 9,
		To:   17,
	}
	// Tuesday 12:00 — wrong day
	tuesday := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	if sched.IsActive(tuesday) {
		t.Error("expected inactive on Tuesday; got active")
	}
}

func TestNodeSchedule_MidnightCrossing_Inside_BeforeMidnight(t *testing.T) {
	// 21:00 – 06:00 window, Mon/Wed/Sat
	sched := &NodeSchedule{
		Days: []time.Weekday{time.Monday, time.Wednesday, time.Saturday},
		From: 21,
		To:   6,
	}
	// Monday 22:00 — inside
	t1 := time.Date(2026, 4, 20, 22, 0, 0, 0, time.UTC)
	if !sched.IsActive(t1) {
		t.Error("expected active Mon 22:00 in midnight-crossing window; got inactive")
	}
}

func TestNodeSchedule_MidnightCrossing_Inside_AfterMidnight(t *testing.T) {
	sched := &NodeSchedule{
		Days: []time.Weekday{time.Monday, time.Wednesday, time.Saturday},
		From: 21,
		To:   6,
	}
	// Tuesday 03:00 — inside (Monday window hasn't closed yet)
	t2 := time.Date(2026, 4, 21, 3, 0, 0, 0, time.UTC)
	if !sched.IsActive(t2) {
		t.Error("expected active Tue 03:00 in Monday's midnight-crossing window; got inactive")
	}
}

func TestNodeSchedule_MidnightCrossing_Outside(t *testing.T) {
	sched := &NodeSchedule{
		Days: []time.Weekday{time.Monday, time.Wednesday, time.Saturday},
		From: 21,
		To:   6,
	}
	// Tuesday 10:00 — outside (window closed at 06:00)
	t3 := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	if sched.IsActive(t3) {
		t.Error("expected inactive Tue 10:00; got active")
	}
}

// ── nodeNetwork / sameNetwork ─────────────────────────────────────────────────

func TestNodeNetwork(t *testing.T) {
	if got := nodeNetwork(&Node{Network: ""}); got != "default" {
		t.Errorf("nodeNetwork(empty) = %q; want %q", got, "default")
	}
	if got := nodeNetwork(&Node{Network: "dmz"}); got != "dmz" {
		t.Errorf("nodeNetwork(dmz) = %q; want %q", got, "dmz")
	}
}

func TestSameNetwork(t *testing.T) {
	a := &Node{Network: ""}
	b := &Node{Network: ""}
	c := &Node{Network: "dmz"}

	if !sameNetwork(a, b) {
		t.Error("expected sameNetwork(default, default) = true; got false")
	}
	if sameNetwork(a, c) {
		t.Error("expected sameNetwork(default, dmz) = false; got true")
	}
	if sameNetwork(c, a) {
		t.Error("expected sameNetwork(dmz, default) = false; got true")
	}
}

// ── Network.CanReach ──────────────────────────────────────────────────────────

func TestCanReach(t *testing.T) {
	a := mkNode("a", withConns("b"))
	b := mkNode("b", withConns("a"))
	c := mkNode("c")
	net := mkNet(a, b, c)

	if !net.CanReach("a", "b") {
		t.Error("expected CanReach(a,b) = true; got false")
	}
	if !net.CanReach("b", "a") {
		t.Error("expected CanReach(b,a) = true; got false")
	}
	if net.CanReach("a", "c") {
		t.Error("expected CanReach(a,c) = false; got true")
	}
}

// ── visibleConnections ────────────────────────────────────────────────────────

func TestVisibleConnections_Normal(t *testing.T) {
	a := mkNode("a", withConns("b", "c"))
	b := mkNode("b")
	c := mkNode("c")
	net := mkNet(a, b, c)
	gs := mkState(net, a)

	vis := gs.visibleConnections(a)
	if len(vis) != 2 {
		t.Errorf("len(visibleConnections) = %d; want 2", len(vis))
	}
}

func TestVisibleConnections_Dark_Hidden(t *testing.T) {
	a := mkNode("a", withConns("b"))
	b := mkNode("b", withDark)
	net := mkNet(a, b)
	gs := mkState(net, a)

	vis := gs.visibleConnections(a)
	if len(vis) != 0 {
		t.Errorf("dark node should be hidden; got %d visible", len(vis))
	}
}

func TestVisibleConnections_Dark_RevealedByLocFile(t *testing.T) {
	a := mkNode("a", withConns("b"))
	b := mkNode("b", withDark)
	net := mkNet(a, b)
	locFile := mkItem("loc-b", "b.loc", ItemTypeNetworkLocation,
		NetworkLocationPayload{NodeID: "b"})
	gs := mkState(net, a, withInv(locFile))

	vis := gs.visibleConnections(a)
	if len(vis) != 1 {
		t.Errorf("dark node with loc file should be visible; got %d", len(vis))
	}
}

func TestVisibleConnections_CrossNetwork_Visible(t *testing.T) {
	// Cross-network nodes in connections ARE visible (they just can't be entered with connect).
	a := mkNode("a", withConns("b"))
	b := mkNode("b", withNet("dmz"))
	net := mkNet(a, b)
	gs := mkState(net, a)

	vis := gs.visibleConnections(a)
	if len(vis) != 1 {
		t.Errorf("cross-network node should be visible in scan; got %d visible", len(vis))
	}
}

func TestVisibleConnections_Unavailable_Hidden(t *testing.T) {
	future := gameStartTime.Add(24 * time.Hour)
	a := mkNode("a", withConns("b"))
	b := mkNode("b", withAvailFrom(future))
	net := mkNet(a, b)
	gs := mkState(net, a)

	vis := gs.visibleConnections(a)
	if len(vis) != 0 {
		t.Errorf("unavailable node should be hidden; got %d visible", len(vis))
	}
}

// ── nodeScanTags ──────────────────────────────────────────────────────────────

func TestNodeScanTags_SameNetwork(t *testing.T) {
	a := mkNode("a")
	b := mkNode("b")
	gs := mkState(mkNet(a, b), a)

	tags := gs.nodeScanTags(b)
	if strings.Contains(tags, "⊗") {
		t.Errorf("same-network node should not have ⊗ tag; got %q", tags)
	}
}

func TestNodeScanTags_CrossNetwork(t *testing.T) {
	a := mkNode("a")
	b := mkNode("b", withNet("dmz"))
	gs := mkState(mkNet(a, b), a)

	tags := gs.nodeScanTags(b)
	if !strings.Contains(tags, "⊗dmz") {
		t.Errorf("cross-network node should have ⊗dmz tag; got %q", tags)
	}
}

// ── hasBridgeTo ───────────────────────────────────────────────────────────────

func TestHasBridgeTo(t *testing.T) {
	a := mkNode("a")
	gs := mkState(mkNet(a), a, withInv(bridgeItem("corp", "dmz")))

	if !gs.hasBridgeTo("corp", "dmz") {
		t.Error("expected hasBridgeTo(corp, dmz) = true; got false")
	}
	// Reverse direction should NOT work.
	if gs.hasBridgeTo("dmz", "corp") {
		t.Error("expected hasBridgeTo(dmz, corp) = false; got true")
	}
	// Different networks.
	if gs.hasBridgeTo("corp", "internet") {
		t.Error("expected hasBridgeTo(corp, internet) = false; got true")
	}
}

// ── handleCommand: connect ────────────────────────────────────────────────────

func TestConnect_Success(t *testing.T) {
	a := mkNode("a", withConns("b"))
	b := mkNode("b")
	net := mkNet(a, b)
	gs := mkState(net, a)

	action := runCmd(gs,"connect b")

	if action != actionPersist {
		t.Errorf("action = %v; want actionPersist", action)
	}
	if gs.CurrentNode.ID != "b" {
		t.Errorf("CurrentNode = %q; want %q", gs.CurrentNode.ID, "b")
	}
	if !gs.VisitedNodes["b"] {
		t.Error("node 'b' should be in VisitedNodes")
	}
	if gs.ConnectCount != 1 {
		t.Errorf("ConnectCount = %d; want 1", gs.ConnectCount)
	}
	if gs.GameTime == gameStartTime {
		t.Error("GameTime should have advanced after connect")
	}
}

func TestConnect_NodeNotExist(t *testing.T) {
	a := mkNode("a")
	gs := mkState(mkNet(a), a)

	action := runCmd(gs,"connect z")

	if action != actionNone {
		t.Errorf("action = %v; want actionNone", action)
	}
	if gs.CurrentNode.ID != "a" {
		t.Error("CurrentNode should not change when connect fails")
	}
}

func TestConnect_NoPath(t *testing.T) {
	a := mkNode("a")
	b := mkNode("b")
	gs := mkState(mkNet(a, b), a)

	action := runCmd(gs,"connect b")

	if action != actionNone {
		t.Errorf("action = %v; want actionNone", action)
	}
	if gs.CurrentNode.ID != "a" {
		t.Error("CurrentNode should not change")
	}
}

func TestConnect_CrossNetwork_Blocked(t *testing.T) {
	a := mkNode("a", withConns("b"))
	b := mkNode("b", withNet("dmz"))
	gs := mkState(mkNet(a, b), a)

	action := runCmd(gs,"connect b")

	if action != actionNone {
		t.Errorf("action = %v; want actionNone", action)
	}
	if gs.CurrentNode.ID != "a" {
		t.Error("CurrentNode should not change on cross-network connect")
	}
	if !logContains(gs, "different network") {
		t.Error("expected 'different network' message in log")
	}
}

func TestConnect_PasswordRequired(t *testing.T) {
	a := mkNode("a", withConns("b"))
	b := mkNode("b", withPW("secret"))
	gs := mkState(mkNet(a, b), a)

	action := runCmd(gs,"connect b")

	if action != actionConnectAuth {
		t.Errorf("action = %v; want actionConnectAuth", action)
	}
	if gs.PendingConnectNode == nil || gs.PendingConnectNode.ID != "b" {
		t.Error("PendingConnectNode should be set to node b")
	}
}

func TestConnect_SSHRequired(t *testing.T) {
	a := mkNode("a", withConns("b"))
	b := mkNode("b", withSSH("admin"))
	gs := mkState(mkNet(a, b), a)

	action := runCmd(gs,"connect b")

	if action != actionConnectSSH {
		t.Errorf("action = %v; want actionConnectSSH", action)
	}
}

func TestConnect_Unavailable(t *testing.T) {
	future := gameStartTime.Add(24 * time.Hour)
	a := mkNode("a", withConns("b"))
	b := mkNode("b", withAvailFrom(future))
	gs := mkState(mkNet(a, b), a)

	action := runCmd(gs,"connect b")

	if action != actionNone {
		t.Errorf("action = %v; want actionNone", action)
	}
	if !logContains(gs, "offline") {
		t.Error("expected 'offline' in log for unavailable node")
	}
}

func TestConnect_ViaLocFile_SameNetwork(t *testing.T) {
	a := mkNode("a") // no direct connection to b
	b := mkNode("b")
	locFile := mkItem("loc-b", "b.loc", ItemTypeNetworkLocation,
		NetworkLocationPayload{NodeID: "b"})
	gs := mkState(mkNet(a, b), a, withInv(locFile))

	action := runCmd(gs,"connect b")

	if action != actionPersist {
		t.Errorf("action = %v; want actionPersist (loc file should enable connect)", action)
	}
	if gs.CurrentNode.ID != "b" {
		t.Errorf("CurrentNode = %q; want b", gs.CurrentNode.ID)
	}
}

func TestConnect_ViaLocFile_CrossNetwork_Blocked(t *testing.T) {
	// Loc file cannot bridge networks.
	a := mkNode("a")
	b := mkNode("b", withNet("dmz"))
	locFile := mkItem("loc-b", "b.loc", ItemTypeNetworkLocation,
		NetworkLocationPayload{NodeID: "b"})
	gs := mkState(mkNet(a, b), a, withInv(locFile))

	action := runCmd(gs,"connect b")

	if action != actionNone {
		t.Errorf("action = %v; want actionNone (loc file can't bridge networks)", action)
	}
}

// ── handleCommand: bridge ─────────────────────────────────────────────────────

func TestBridge_Success(t *testing.T) {
	a := mkNode("a", withConns("b"))
	b := mkNode("b", withNet("dmz"))
	gs := mkState(mkNet(a, b), a, withInv(bridgeItem("default", "dmz")))

	action := runCmd(gs,"bridge b")

	if action != actionPersist {
		t.Errorf("action = %v; want actionPersist", action)
	}
	if gs.CurrentNode.ID != "b" {
		t.Errorf("CurrentNode = %q; want b", gs.CurrentNode.ID)
	}
}

func TestBridge_NoItem(t *testing.T) {
	a := mkNode("a", withConns("b"))
	b := mkNode("b", withNet("dmz"))
	gs := mkState(mkNet(a, b), a) // no bridge item

	action := runCmd(gs,"bridge b")

	if action != actionNone {
		t.Errorf("action = %v; want actionNone", action)
	}
	if !logContains(gs, "bridge") || !logContains(gs, "dmz") {
		t.Error("expected bridge adapter message in log")
	}
}

func TestBridge_WrongDirection(t *testing.T) {
	// Have dmz→default but need default→dmz
	a := mkNode("a", withConns("b"))
	b := mkNode("b", withNet("dmz"))
	gs := mkState(mkNet(a, b), a, withInv(bridgeItem("dmz", "default")))

	action := runCmd(gs,"bridge b")

	if action != actionNone {
		t.Errorf("action = %v; want actionNone (wrong bridge direction)", action)
	}
}

func TestBridge_SameNetwork_Rejected(t *testing.T) {
	a := mkNode("a", withConns("b"))
	b := mkNode("b") // same default network
	gs := mkState(mkNet(a, b), a, withInv(bridgeItem("default", "default")))

	action := runCmd(gs,"bridge b")

	if action != actionNone {
		t.Errorf("action = %v; want actionNone (same network)", action)
	}
	if !logContains(gs, "connect") {
		t.Error("expected suggestion to use 'connect' in log")
	}
}

func TestBridge_NotDirectConnection(t *testing.T) {
	a := mkNode("a") // no connection to b
	b := mkNode("b", withNet("dmz"))
	gs := mkState(mkNet(a, b), a, withInv(bridgeItem("default", "dmz")))

	action := runCmd(gs,"bridge b")

	if action != actionNone {
		t.Errorf("action = %v; want actionNone (no direct connection)", action)
	}
	if !logContains(gs, "direct") {
		t.Error("expected 'direct connection' message in log")
	}
}

func TestBridge_NodeNotExist(t *testing.T) {
	a := mkNode("a")
	gs := mkState(mkNet(a), a, withInv(bridgeItem("default", "dmz")))

	action := runCmd(gs,"bridge z")

	if action != actionNone {
		t.Errorf("action = %v; want actionNone", action)
	}
}

// ── handleCommand: assimilate ─────────────────────────────────────────────────

func TestAssimilate_TextFile(t *testing.T) {
	f := mkItem("f-1", "notes.txt", ItemTypeTextFile, TextFilePayload{Text: "hello"})
	a := mkNode("a", withNodeFiles(f))
	gs := mkState(mkNet(a), a)

	action := runCmd(gs,"assimilate notes.txt")

	if action != actionPersist {
		t.Errorf("action = %v; want actionPersist", action)
	}
	if !gs.inInventory("f-1") {
		t.Error("file should be in inventory after assimilate")
	}
	if gs.AssimilateCount != 1 {
		t.Errorf("AssimilateCount = %d; want 1", gs.AssimilateCount)
	}
}

func TestAssimilate_AlreadyHave(t *testing.T) {
	f := mkItem("f-1", "notes.txt", ItemTypeTextFile, TextFilePayload{Text: "hello"})
	a := mkNode("a", withNodeFiles(f))
	gs := mkState(mkNet(a), a, withInv(f))

	action := runCmd(gs,"assimilate notes.txt")

	if action != actionNone {
		t.Errorf("action = %v; want actionNone (already assimilated)", action)
	}
}

func TestAssimilate_ClaimCode(t *testing.T) {
	f := mkItem("cc-1", "claim.code", ItemTypeClaimCode, map[string]interface{}{})
	a := mkNode("a", withNodeFiles(f))
	gs := mkState(mkNet(a), a)
	initialSkill := gs.Stats.ClaimSkill

	action := runCmd(gs,"assimilate claim.code")

	if action != actionPersist {
		t.Errorf("action = %v; want actionPersist", action)
	}
	// Claim codes are consumed, not stored in inventory.
	if gs.inInventory("cc-1") {
		t.Error("claim code should not be in inventory (it gets consumed)")
	}
	if gs.Stats.ClaimSkill != initialSkill+1 {
		t.Errorf("ClaimSkill = %d; want %d", gs.Stats.ClaimSkill, initialSkill+1)
	}
	if !gs.DeletedNodeFiles["cc-1"] {
		t.Error("claim code should be marked deleted after assimilation")
	}
}

func TestAssimilate_NotFound(t *testing.T) {
	a := mkNode("a")
	gs := mkState(mkNet(a), a)

	action := runCmd(gs,"assimilate missing.txt")

	if action != actionNone {
		t.Errorf("action = %v; want actionNone", action)
	}
	if !logContains(gs, "not found") {
		t.Error("expected 'not found' in log")
	}
}

// ── handleCommand: scan ───────────────────────────────────────────────────────

func TestScan_WithoutApp_UnknownCommand(t *testing.T) {
	a := mkNode("a")
	gs := mkState(mkNet(a), a)

	action := runCmd(gs,"scan")

	if action != actionNone {
		t.Errorf("action = %v; want actionNone", action)
	}
	if !logContains(gs, "Unknown command") {
		t.Error("expected 'Unknown command' in log when scan app not present")
	}
}

func TestScan_WithApp_ShowsConnections(t *testing.T) {
	a := mkNode("a", withConns("b"))
	b := mkNode("b")
	gs := mkState(mkNet(a, b), a, withInv(scanApp()))

	action := runCmd(gs,"scan")

	if action != actionNone {
		t.Errorf("action = %v; want actionNone", action)
	}
	if !logContains(gs, "b") {
		t.Error("expected node 'b' to appear in scan output")
	}
}

func TestScan_CrossNetworkNode_Tagged(t *testing.T) {
	// ⊗network tags are only shown by the v2/v3 tree renderer, not the v1 plain list.
	a := mkNode("a", withConns("b"))
	b := mkNode("b", withNet("dmz"))
	scanV3 := mkItem("app-scan3", "scan_v3.app", ItemTypeApplication,
		ApplicationPayload{Text: "scan v3", Action: "scan_network_v3"})
	gs := mkState(mkNet(a, b), a, withInv(scanV3))

	runCmd(gs,"scan")

	if !logContains(gs, "⊗dmz") {
		t.Error("cross-network node should appear with ⊗dmz tag in v3 scan output")
	}
}

// ── handleCommand: ls ─────────────────────────────────────────────────────────

func TestLS_ShowsFiles(t *testing.T) {
	f := mkItem("f-1", "data.txt", ItemTypeTextFile, TextFilePayload{Text: "hi"})
	a := mkNode("a", withNodeFiles(f))
	gs := mkState(mkNet(a), a)

	action := runCmd(gs,"ls")

	if action != actionNone {
		t.Errorf("action = %v; want actionNone", action)
	}
	if !logContains(gs, "data.txt") {
		t.Error("expected file name in ls output")
	}
}

func TestLS_DeletedFilesHidden(t *testing.T) {
	f := mkItem("f-1", "data.txt", ItemTypeTextFile, TextFilePayload{Text: "hi"})
	a := mkNode("a", withNodeFiles(f))
	gs := mkState(mkNet(a), a)
	gs.DeletedNodeFiles["f-1"] = true

	runCmd(gs,"ls")

	if logContains(gs, "data.txt") {
		t.Error("deleted file should not appear in ls output")
	}
}

// ── handleCommand: general ────────────────────────────────────────────────────

func TestUnknownCommand(t *testing.T) {
	a := mkNode("a")
	gs := mkState(mkNet(a), a)

	action := runCmd(gs,"frobulate")

	if action != actionNone {
		t.Errorf("action = %v; want actionNone", action)
	}
	if !logContains(gs, "Unknown command") {
		t.Error("expected 'Unknown command' in log")
	}
}

func TestEmptyInput(t *testing.T) {
	a := mkNode("a")
	gs := mkState(mkNet(a), a)

	action := runCmd(gs,"")

	if action != actionNone {
		t.Errorf("action = %v; want actionNone", action)
	}
	// Empty input should not append to MessageLog.
	if len(gs.MessageLog) > 0 {
		t.Errorf("MessageLog should be empty for empty input; got %v", gs.MessageLog)
	}
}

func TestQuit(t *testing.T) {
	a := mkNode("a")
	gs := mkState(mkNet(a), a)

	if got := runCmd(gs,"quit"); got != actionQuit {
		t.Errorf("quit returned %v; want actionQuit", got)
	}
	if got := runCmd(gs,"exit"); got != actionQuit {
		t.Errorf("exit returned %v; want actionQuit", got)
	}
}
