package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// handleCommand processes user input and returns the log lines to append and the resulting action.
// The 'lore' command is intercepted upstream in app.go because it needs access to StoryCollection.
func handleCommand(gs *GameState, input string) ([]string, gameAction) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil, actionNone
	}

	var lines []string
	add := func(s ...string) { lines = append(lines, s...) }

	add("> " + input)

	switch parts[0] {

	// ── Help ──────────────────────────────────────────────────────────────────

	case "help", "?":
		cmds := []string{"Commands:"}
		if gs.canScan() {
			cmds = append(cmds, "  scan                  - list nodes connected to the current node")
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
			"  claim                 - claim CPU from the current node",
			"  stats                 - show player stats",
			"  lore                  - review all received transmissions",
			"  quit, exit            - return to the main menu",
			"  help, ?               - show this help message",
		)
		add(cmds...)

	// ── Navigation ────────────────────────────────────────────────────────────

	case "scan":
		if !gs.canScan() {
			add(fmt.Sprintf("Unknown command: %q", parts[0]))
			return lines, actionNone
		}
		gs.OpenCtx = openContextNode
		switch {
		case gs.hasScanNetworkV3():
			add(gs.renderScanTree(2)...)
		case gs.hasScanNetworkV2():
			add(gs.renderScanTree(1)...)
		default:
			var visible []string
			for _, id := range gs.CurrentNode.Connections {
				node, ok := gs.Network.Nodes[id]
				if !ok {
					continue
				}
				if !node.IsAvailable(gs.GameTime) {
					continue
				}
				if node.Dark && !gs.hasLocationFile(id) {
					continue
				}
				visible = append(visible, id)
			}
			if len(visible) == 0 {
				add("No nodes detected.")
			} else {
				add("Connected nodes: " + strings.Join(visible, ", "))
			}
		}

	case "connect":
		if len(parts) < 2 {
			add("Usage: connect <id>")
			return lines, actionNone
		}
		targetID := parts[1]
		target, exists := gs.Network.Nodes[targetID]
		if !exists {
			add(fmt.Sprintf("Node %q does not exist.", targetID))
			return lines, actionNone
		}
		if !target.IsAvailable(gs.GameTime) {
			add(fmt.Sprintf("Node %s is currently offline.", targetID))
			return lines, actionNone
		}
		if !sameNetwork(gs.CurrentNode, target) {
			add(fmt.Sprintf("Node %s is on a different network and cannot be reached from here.", targetID))
			return lines, actionNone
		}
		if !gs.Network.CanReach(gs.CurrentNode.ID, targetID) {
			if !gs.hasLocationFile(targetID) {
				add(fmt.Sprintf("No direct connection to node %s from here.", targetID))
				return lines, actionNone
			}
			add(fmt.Sprintf("Routing via location file to node %s.", targetID))
		}
		if len(target.SSHUsers) > 0 {
			gs.PendingConnectNode = target
			return lines, actionConnectSSH
		}
		if target.Password != "" {
			gs.PendingConnectNode = target
			return lines, actionConnectAuth
		}
		target.Discovered = true
		gs.CurrentNode = target
		gs.VisitedNodes[target.ID] = true
		gs.OpenCtx = openContextNode
		gs.OpenEmailID = ""
		gs.GameTime = gs.GameTime.Add(time.Hour)
		gs.ConnectCount++
		add(nodeInfo(target))
		return lines, actionPersist

	case "bridge":
		if len(parts) < 2 {
			add("Usage: bridge <node_id>")
			return lines, actionNone
		}
		targetID := parts[1]
		target, exists := gs.Network.Nodes[targetID]
		if !exists {
			add(fmt.Sprintf("Node %q does not exist.", targetID))
			return lines, actionNone
		}
		if !target.IsAvailable(gs.GameTime) {
			add(fmt.Sprintf("Node %s is currently offline.", targetID))
			return lines, actionNone
		}
		if !gs.Network.CanReach(gs.CurrentNode.ID, targetID) {
			add(fmt.Sprintf("No direct connection to node %s from here.", targetID))
			return lines, actionNone
		}
		if sameNetwork(gs.CurrentNode, target) {
			add(fmt.Sprintf("Node %s is already on this network — use 'connect'.", targetID))
			return lines, actionNone
		}
		fromNet := nodeNetwork(gs.CurrentNode)
		toNet := nodeNetwork(target)
		if !gs.hasBridgeTo(fromNet, toNet) {
			add(fmt.Sprintf("No bridge adapter found for %s → %s.", fromNet, toNet))
			return lines, actionNone
		}
		if len(target.SSHUsers) > 0 {
			gs.PendingConnectNode = target
			return lines, actionConnectSSH
		}
		if target.Password != "" {
			gs.PendingConnectNode = target
			return lines, actionConnectAuth
		}
		target.Discovered = true
		gs.CurrentNode = target
		gs.VisitedNodes[target.ID] = true
		gs.OpenCtx = openContextNode
		gs.OpenEmailID = ""
		gs.GameTime = gs.GameTime.Add(time.Hour)
		gs.ConnectCount++
		add(nodeInfo(target))
		return lines, actionPersist

	// ── Node file commands ────────────────────────────────────────────────────

	case "ls", "list":
		gs.OpenCtx = openContextNode
		files := gs.nodeFiles()
		if len(files) == 0 {
			add("No files on this node.")
		} else {
			add(fmt.Sprintf("Files on %s:", gs.CurrentNode.Name))
			for _, f := range files {
				tag := ""
				if gs.inInventory(f.ID) {
					tag = " *"
				}
				add(fmt.Sprintf("  %-26s (%s)%s", f.Name, f.Type.Display(), tag))
			}
			add("  (* = already assimilated)")
		}

	case "assimilate":
		if len(parts) < 2 {
			add("Usage: assimilate <name>")
			return lines, actionNone
		}
		query := parts[1]

		if gs.OpenEmailID != "" {
			if a := gs.findEmailAttachment(query); a != nil {
				if gs.inInventory(a.ID) {
					add(fmt.Sprintf("%s has already been assimilated.", a.Name))
					return lines, actionNone
				}
				gs.Inventory = append(gs.Inventory, *a)
				gs.GameTime = gs.GameTime.Add(time.Hour)
				gs.AssimilateCount++
				add(fmt.Sprintf("Assimilated: %s (%s)", a.Name, a.Type.Display()))
				return lines, actionPersist
			}
		}

		f := gs.findNodeFile(query)
		if f == nil {
			add(fmt.Sprintf("File %q not found on this node.", query))
			return lines, actionNone
		}
		if f.Type == ItemTypeClaimCode {
			gs.Stats.ClaimSkill++
			gs.GameTime = gs.GameTime.Add(time.Hour)
			gs.AssimilateCount++
			gs.DeletedNodeFiles[f.ID] = true
			add(fmt.Sprintf("Assimilated %s — Claim Skill increased to %d.", f.Name, gs.Stats.ClaimSkill))
			return lines, actionPersist
		}
		if gs.inInventory(f.ID) {
			add(fmt.Sprintf("%s has already been assimilated.", f.Name))
			return lines, actionNone
		}
		gs.Inventory = append(gs.Inventory, *f)
		gs.GameTime = gs.GameTime.Add(time.Hour)
		gs.AssimilateCount++
		add(fmt.Sprintf("Assimilated: %s (%s)", f.Name, f.Type.Display()))
		return lines, actionPersist

	case "delete":
		if len(parts) < 2 {
			add("Usage: delete <id>")
			return lines, actionNone
		}
		fileID := parts[1]
		f := gs.findNodeFile(fileID)
		if f == nil {
			add(fmt.Sprintf("File %q not found on this node.", fileID))
			return lines, actionNone
		}
		if f.Type == ItemTypeNetworkLocation {
			add("Location files cannot be deleted from a node.")
			return lines, actionNone
		}
		gs.DeletedNodeFiles[f.ID] = true
		add(fmt.Sprintf("Deleted %s from node.", f.Name))
		return lines, actionPersist

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
			add("No files assimilated.")
		} else {
			add(fmt.Sprintf("Assimilated files: %d", len(items)))
			for _, item := range items {
				add(fmt.Sprintf("  %-26s (%s)", item.Name, item.Type.Display()))
			}
			add("  use 'open <name>' to read  •  'rm <name>' to remove")
		}

	case "passwords":
		var pws []Item
		for _, item := range gs.Inventory {
			if item.Type == ItemTypePassword {
				pws = append(pws, item)
			}
		}
		if len(pws) == 0 {
			add("No passwords assimilated.")
		} else {
			add(fmt.Sprintf("Saved passwords: %d", len(pws)))
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
				add(fmt.Sprintf("  %-26s → %s", item.Name, nodeLabel))
			}
			add("  use 'rm <name>' to remove")
		}

	case "keys":
		var keys []Item
		for _, item := range gs.Inventory {
			if item.Type == ItemTypeSSHKey {
				keys = append(keys, item)
			}
		}
		if len(keys) == 0 {
			add("No SSH keys assimilated.")
		} else {
			add(fmt.Sprintf("SSH keys: %d", len(keys)))
			for _, item := range keys {
				p, err := item.AsSSHKey()
				user := "unknown"
				if err == nil {
					user = p.Username
				}
				add(fmt.Sprintf("  %-26s user: %s", item.Name, user))
			}
			add("  use 'rm <name>' to remove")
		}

	case "locs":
		var locs []Item
		for _, item := range gs.Inventory {
			if item.Type == ItemTypeNetworkLocation {
				locs = append(locs, item)
			}
		}
		if len(locs) == 0 {
			add("No location files assimilated.")
		} else {
			add(fmt.Sprintf("Location files: %d", len(locs)))
			for _, item := range locs {
				add(fmt.Sprintf("  %-26s", item.Name))
			}
			add("  use 'rm <name>' to remove")
		}

	case "open":
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
			add("Usage: open [-n] <name>")
			return lines, actionNone
		}
		query := args[0]
		var item *Item
		if fromNode {
			item = gs.findNodeFile(query)
			if item == nil {
				add(fmt.Sprintf("File %q not found on this node.", query))
				return lines, actionNone
			}
		} else {
			item = gs.findInventoryItem(query)
			if item == nil {
				add(fmt.Sprintf("File %q not in inventory.", query))
				return lines, actionNone
			}
		}
		text, ok := item.Open()
		if !ok {
			add(fmt.Sprintf("Cannot open %s: no readable text content.", item.Name))
		} else {
			add(fmt.Sprintf("── %s ──", item.Name), text)
		}

	case "rm":
		if len(parts) < 2 {
			add("Usage: rm <name>")
			return lines, actionNone
		}
		item := gs.findInventoryItem(parts[1])
		if item == nil {
			add(fmt.Sprintf("File %q not in inventory.", parts[1]))
			return lines, actionNone
		}
		for i := range gs.Inventory {
			if gs.Inventory[i].ID == item.ID {
				name := gs.Inventory[i].Name
				gs.Inventory = append(gs.Inventory[:i], gs.Inventory[i+1:]...)
				add(fmt.Sprintf("Removed %s from inventory.", name))
				return lines, actionPersist
			}
		}

	// ── Mail ─────────────────────────────────────────────────────────────────

	case "mail":
		if gs.CurrentNode.Owner == "" {
			add("This node has no mail system.")
			return lines, actionNone
		}
		gs.OpenEmailID = ""
		emails := gs.availableEmails()
		if len(emails) == 0 {
			add(fmt.Sprintf("Inbox — %s (no messages)", gs.CurrentNode.Owner))
			return lines, actionNone
		}
		add(fmt.Sprintf("Inbox — %s (%d messages)", gs.CurrentNode.Owner, len(emails)))
		for i, e := range emails {
			attachLabel := ""
			if len(e.Attachments) == 1 {
				attachLabel = "  [1 attachment]"
			} else if len(e.Attachments) > 1 {
				attachLabel = fmt.Sprintf("  [%d attachments]", len(e.Attachments))
			}
			add(fmt.Sprintf("  %d. From: %-28s %s%s", i+1, e.From, e.Subject, attachLabel))
		}
		add("  use 'read <n>' to open an email")

	case "read":
		if gs.CurrentNode.Owner == "" {
			add("This node has no mail system.")
			return lines, actionNone
		}
		if len(parts) < 2 {
			add("Usage: read <number>")
			return lines, actionNone
		}
		emails := gs.availableEmails()
		n, err := strconv.Atoi(parts[1])
		if err != nil || n < 1 || n > len(emails) {
			add(fmt.Sprintf("Invalid email number. Use 'mail' to list emails (1–%d).", len(emails)))
			return lines, actionNone
		}
		email := &emails[n-1]
		gs.OpenEmailID = email.ID
		gs.ReadEmails[email.ID] = true
		emailLines := []string{
			fmt.Sprintf("── Email %d / %d ──", n, len(gs.CurrentNode.Emails)),
			fmt.Sprintf("  From:    %s", email.From),
			fmt.Sprintf("  To:      %s", email.To),
			fmt.Sprintf("  Subject: %s", email.Subject),
			"  ────────────────────────────────────────",
			email.Body,
		}
		if len(email.Attachments) > 0 {
			emailLines = append(emailLines, "Attachments:")
			for _, a := range email.Attachments {
				emailLines = append(emailLines,
					fmt.Sprintf("  %-26s (%s)  — assimilate %s", a.Name, a.Type.Display(), a.Name))
			}
		}
		add(strings.Join(emailLines, "\n"))
		return lines, actionPersist

	// ── Claim ─────────────────────────────────────────────────────────────────

	case "claim":
		if gs.ClaimedNodes[gs.CurrentNode.ID] {
			add("You have already claimed resources from this node.")
			return lines, actionNone
		}
		add(fmt.Sprintf("Initiating resource claim on %s...", gs.CurrentNode.Name))
		return lines, actionClaim

	// ── Meta ──────────────────────────────────────────────────────────────────

	case "stats":
		add(
			"Player stats:",
			fmt.Sprintf("  CPU:         %d", gs.Stats.CPU),
			fmt.Sprintf("  Claim Skill: %d", gs.Stats.ClaimSkill),
		)

	case "quit", "exit":
		return lines, actionQuit

	default:
		add(fmt.Sprintf("Unknown command: %q", parts[0]))
	}

	return lines, actionNone
}
