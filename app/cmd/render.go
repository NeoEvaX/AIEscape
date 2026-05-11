package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// ── Palette ───────────────────────────────────────────────────────────────────

var (
	styleCmd        = lipgloss.NewStyle().Foreground(lipgloss.Color("#39D353")).Bold(true)
	styleNodeHeader = lipgloss.NewStyle().Foreground(lipgloss.Color("#22D3EE")).Bold(true)
	styleFileHeader = lipgloss.NewStyle().Foreground(lipgloss.Color("#FCD34D")).Bold(true)
	styleSection    = lipgloss.NewStyle().Foreground(lipgloss.Color("#A78BFA"))
	styleDetail     = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	styleSuccess    = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399"))
	styleWarn       = lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171"))
	styleNormal     = lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1"))
	styleMeta       = lipgloss.NewStyle().Foreground(lipgloss.Color("#475569"))
	styleDivider    = lipgloss.NewStyle().Foreground(lipgloss.Color("#1E3A4A"))

	styleSSHLocked   = lipgloss.NewStyle().Foreground(lipgloss.Color("#39D353")).Bold(true)
	styleSSHCycling  = lipgloss.NewStyle().Foreground(lipgloss.Color("#1E3A2A"))

	styleStatusLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("#475569"))
	styleStatusValue = lipgloss.NewStyle().Foreground(lipgloss.Color("#22D3EE")).Bold(true)
	styleStatusSep   = lipgloss.NewStyle().Foreground(lipgloss.Color("#1E3A4A"))
)

var divider = "  " + strings.Repeat("─", 60)

// ── Line classification ───────────────────────────────────────────────────────

func isSectionLine(line string) bool {
	for _, prefix := range []string{
		"Files on ", "Connected nodes", "Commands:", "Player stats:",
		"Assimilated files", "Location files", "No files", "No nodes detected",
		"No saves", "No location", "No files assimilated",
		"Inbox — ", "Attachments:",
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func isWarnLine(line string) bool {
	for _, kw := range []string{
		"not found", "does not exist", "No direct connection",
		"cannot be deleted", "Cannot open", "Unknown command",
		"Usage:", "already been assimilated", "already claimed",
		"cannot open", "not in inventory",
	} {
		if strings.Contains(strings.ToLower(line), strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func isSuccessLine(line string) bool {
	for _, prefix := range []string{
		"Assimilated", "Deleted ", "Removed ", "Claim complete",
		"Initiating resource", "Routing via",
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func classifyAndRender(line string) string {
	switch {
	case strings.HasPrefix(line, "> "):
		return styleCmd.Render(line)
	case strings.HasPrefix(line, "[Node "):
		return styleNodeHeader.Render(line)
	case strings.HasPrefix(line, "── "):
		return styleFileHeader.Render(line)
	case strings.HasPrefix(line, "────"):
		return styleDivider.Render(line)
	case strings.HasPrefix(line, "RAM: ") || strings.HasPrefix(line, "  RAM:") ||
		strings.HasPrefix(line, "  CPU:") || strings.HasPrefix(line, "  Claim"):
		return styleDetail.Render(line)
	case strings.HasPrefix(line, "  [Personal Computer"):
		return styleSection.Render(line)
	case isSectionLine(line):
		return styleSection.Render(line)
	case strings.HasPrefix(line, "  "):
		return styleDetail.Render(line)
	case isWarnLine(line):
		return styleWarn.Render(line)
	case isSuccessLine(line):
		return styleSuccess.Render(line)
	default:
		return styleNormal.Render(line)
	}
}

// renderLog renders the message log with color and a subtle divider before
// each command block.
func renderLog(entries []string) string {
	var b strings.Builder
	firstCmd := true
	for _, entry := range entries {
		// Entries may contain embedded newlines (e.g. nodeInfo).
		lines := strings.Split(entry, "\n")
		isCmd := strings.HasPrefix(lines[0], "> ")
		if isCmd {
			if !firstCmd {
				b.WriteString(styleDivider.Render(divider) + "\n")
			}
			firstCmd = false
		}
		for _, line := range lines {
			b.WriteString(classifyAndRender(line) + "\n")
		}
	}
	return b.String()
}

// renderStatusBar renders the top status bar for the game screen.
func renderStatusBar(gs *GameState) string {
	sep := styleStatusSep.Render("  │  ")
	parts := []string{
		styleStatusValue.Render(gs.SaveName),
		styleStatusLabel.Render("node") + " " + styleStatusValue.Render(gs.CurrentNode.ID+": "+gs.CurrentNode.Name),
		styleStatusLabel.Render("visited") + " " + styleStatusValue.Render(
			sprint(len(gs.VisitedNodes))+"/"+sprint(len(gs.Network.Nodes))),
		styleStatusLabel.Render("ram") + " " + styleStatusValue.Render(sprint(gs.Stats.RAM)) +
			"  " + styleStatusLabel.Render("cpu") + " " + styleStatusValue.Render(sprint(gs.Stats.CPU)) +
			"  " + styleStatusLabel.Render("cs") + " " + styleStatusValue.Render(sprint(gs.Stats.ClaimSkill)),
		styleStatusLabel.Render("time") + " " + styleStatusValue.Render(gs.GameTime.Format("Jan 02 2006  15:04")),
	}
	bar := "  " + strings.Join(parts, sep)
	rule := styleDivider.Render("  " + strings.Repeat("─", 70))
	return bar + "\n" + rule + "\n"
}

func sprint(n int) string {
	return fmt.Sprintf("%d", n)
}
