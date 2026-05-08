package main

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Screen int

const (
	ScreenMainMenu Screen = iota
	ScreenNewGame
	ScreenLoadSave
	ScreenGame
)

var mainMenuItems = []string{"New Game", "Load Game", "Quit"}

type claimState struct {
	active  bool
	nodeID  string
	elapsed time.Duration
	total   time.Duration
	bar     progress.Model
}

type authPhase int

const (
	authPhaseMenu   authPhase = iota // showing the 3-option menu
	authPhaseTyping                  // player is typing a password
	authPhaseBrute                   // brute force in progress
)

type connectAuthState struct {
	active   bool
	node     *Node
	phase    authPhase
	menuSel  int // 0=type pw, 1=use saved, 2=brute force
	hasSaved bool
	pwInput  textinput.Model
	elapsed  time.Duration
	total    time.Duration
	bar      progress.Model
}

type AppModel struct {
	screen  Screen
	db      *Database
	network *Network

	// Main menu
	menuCursor int

	// New game
	nameInput textinput.Model
	nameErr   string

	// Load/delete saves
	saves      []Save
	saveCursor int
	savesErr   string

	// Game
	gs                  *GameState
	awaitingQuitConfirm bool
	claim               claimState
	auth                connectAuthState
}

func NewAppModel(db *Database, network *Network) AppModel {
	return AppModel{
		screen:  ScreenMainMenu,
		db:      db,
		network: network,
	}
}

func (m AppModel) Init() tea.Cmd { return nil }

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case ScreenMainMenu:
		return m.updateMainMenu(msg)
	case ScreenNewGame:
		return m.updateNewGame(msg)
	case ScreenLoadSave:
		return m.updateLoadSave(msg)
	case ScreenGame:
		return m.updateGame(msg)
	}
	return m, nil
}

func (m AppModel) View() tea.View {
	var s string
	switch m.screen {
	case ScreenMainMenu:
		s = m.viewMainMenu()
	case ScreenNewGame:
		s = m.viewNewGame()
	case ScreenLoadSave:
		s = m.viewLoadSave()
	case ScreenGame:
		s = m.viewGame()
	}
	return tea.NewView(s)
}

// ── Main Menu ─────────────────────────────────────────────────────────────────

func (m AppModel) updateMainMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch kp.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < len(mainMenuItems)-1 {
			m.menuCursor++
		}
	case "enter":
		switch mainMenuItems[m.menuCursor] {
		case "New Game":
			ti := textinput.New()
			ti.Placeholder = "my-save"
			ti.Focus()
			m.nameInput = ti
			m.nameErr = ""
			m.screen = ScreenNewGame
		case "Load Game":
			m.saveCursor = 0
			m.savesErr = ""
			m.screen = ScreenLoadSave
			return m, loadSavesCmd(m.db)
		case "Quit":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m AppModel) viewMainMenu() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styleNodeHeader.Render("  ▲ AI ESCAPE") + "\n")
	b.WriteString(styleDetail.Render("  ─────────────────") + "\n\n")
	for i, item := range mainMenuItems {
		if i == m.menuCursor {
			b.WriteString(styleCmd.Render("  ▶ "+item) + "\n")
		} else {
			b.WriteString(styleDetail.Render("    "+item) + "\n")
		}
	}
	b.WriteString("\n" + styleDetail.Render("  ↑↓  navigate  •  Enter  select"))
	return b.String()
}

// ── New Game ──────────────────────────────────────────────────────────────────

type saveCreatedMsg struct {
	id  int64
	err error
}

func createSaveCmd(db *Database, name, startNodeID string) tea.Cmd {
	return func() tea.Msg {
		id, err := db.CreateSave(name, startNodeID, []string{startNodeID})
		return saveCreatedMsg{id: id, err: err}
	}
}

func (m AppModel) updateNewGame(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case saveCreatedMsg:
		if msg.err != nil {
			m.nameErr = "Could not create save: " + msg.err.Error()
			return m, nil
		}
		startNode := m.network.Nodes[m.network.StartNodeID]
		m.gs = newGameState(m.network, msg.id, strings.TrimSpace(m.nameInput.Value()), startNode)
		m.screen = ScreenGame
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.screen = ScreenMainMenu
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.nameInput.Value())
			if name == "" {
				m.nameErr = "Name cannot be empty."
				return m, nil
			}
			m.nameErr = ""
			startNode := m.network.Nodes[m.network.StartNodeID]
			return m, createSaveCmd(m.db, name, startNode.ID)
		}
	}

	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

func (m AppModel) viewNewGame() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styleNodeHeader.Render("  ▲ NEW GAME") + "\n")
	b.WriteString(styleDetail.Render("  ─────────────────") + "\n\n")
	b.WriteString(styleDetail.Render("  Save name: ") + m.nameInput.View() + "\n")
	if m.nameErr != "" {
		b.WriteString("\n" + styleWarn.Render("  "+m.nameErr) + "\n")
	}
	b.WriteString("\n" + styleDetail.Render("  Enter  start  •  Esc  back"))
	return b.String()
}

// ── Load / Delete Save ────────────────────────────────────────────────────────

type savesLoadedMsg struct {
	saves []Save
	err   error
}

func loadSavesCmd(db *Database) tea.Cmd {
	return func() tea.Msg {
		saves, err := db.ListSaves()
		return savesLoadedMsg{saves: saves, err: err}
	}
}

type saveDeletedMsg struct{ err error }

func deleteSaveCmd(db *Database, id int64) tea.Cmd {
	return func() tea.Msg {
		return saveDeletedMsg{err: db.DeleteSave(id)}
	}
}

type saveLoadedMsg struct {
	save             *Save
	visited          []string
	inventory        []Item
	deletedNodeFiles []string
	claimedNodes     []string
	err              error
}

func loadSaveCmd(db *Database, id int64) tea.Cmd {
	return func() tea.Msg {
		save, visited, err := db.LoadSave(id)
		if err != nil {
			return saveLoadedMsg{err: err}
		}
		inventory, err := db.GetInventory(id)
		if err != nil {
			return saveLoadedMsg{err: err}
		}
		deleted, err := db.GetDeletedNodeFiles(id)
		if err != nil {
			return saveLoadedMsg{err: err}
		}
		claimed, err := db.GetClaimedNodes(id)
		// stats are embedded in save via LoadSave
		return saveLoadedMsg{save: save, visited: visited, inventory: inventory, deletedNodeFiles: deleted, claimedNodes: claimed, err: err}
	}
}

func (m AppModel) updateLoadSave(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case savesLoadedMsg:
		if msg.err != nil {
			m.savesErr = msg.err.Error()
		} else {
			m.saves = msg.saves
			m.savesErr = ""
		}
		return m, nil

	case saveDeletedMsg:
		if msg.err != nil {
			m.savesErr = msg.err.Error()
			return m, nil
		}
		return m, loadSavesCmd(m.db)

	case saveLoadedMsg:
		if msg.err != nil {
			m.savesErr = msg.err.Error()
			return m, nil
		}
		currentNode := m.network.Nodes[msg.save.CurrentNodeID]
		m.gs = newGameStateFromSave(m.network, msg.save, currentNode, msg.visited, msg.deletedNodeFiles, msg.claimedNodes, msg.inventory, msg.save.Stats)
		m.screen = ScreenGame
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.screen = ScreenMainMenu
			return m, nil
		case "up", "k":
			if m.saveCursor > 0 {
				m.saveCursor--
			}
		case "down", "j":
			if m.saveCursor < len(m.saves)-1 {
				m.saveCursor++
			}
		case "enter":
			if len(m.saves) > 0 {
				return m, loadSaveCmd(m.db, m.saves[m.saveCursor].ID)
			}
		case "d":
			if len(m.saves) > 0 {
				id := m.saves[m.saveCursor].ID
				if m.saveCursor > 0 {
					m.saveCursor--
				}
				return m, deleteSaveCmd(m.db, id)
			}
		}
	}
	return m, nil
}

func (m AppModel) viewLoadSave() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styleNodeHeader.Render("  ▲ LOAD GAME") + "\n")
	b.WriteString(styleDetail.Render("  ─────────────────") + "\n\n")
	if m.savesErr != "" {
		b.WriteString(styleWarn.Render("  Error: "+m.savesErr) + "\n\n")
	}
	if len(m.saves) == 0 && m.savesErr == "" {
		b.WriteString(styleDetail.Render("  No saves found.") + "\n")
	}
	for i, s := range m.saves {
		if i == m.saveCursor {
			name := styleCmd.Render(fmt.Sprintf("  ▶ %-22s", s.Name))
			meta := styleDetail.Render(fmt.Sprintf("  %3d nodes visited  ·  %s", s.VisitedCount, s.UpdatedAt.Format("2006-01-02 15:04")))
			b.WriteString(name + meta + "\n")
		} else {
			line := styleDetail.Render(fmt.Sprintf("    %-22s  %3d nodes visited  ·  %s", s.Name, s.VisitedCount, s.UpdatedAt.Format("2006-01-02 15:04")))
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("\n" + styleDetail.Render("  Enter  load  •  D  delete  •  Esc  back"))
	return b.String()
}

// ── Game ──────────────────────────────────────────────────────────────────────

type gameSavedMsg struct{ err error }

type claimTickMsg struct{}
type bruteTickMsg struct{}

func claimTickCmd() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(100 * time.Millisecond)
		return claimTickMsg{}
	}
}

func bruteTickCmd() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(100 * time.Millisecond)
		return bruteTickMsg{}
	}
}

// completeConnect performs the node connection after auth succeeds.
func completeConnect(gs *GameState, node *Node) {
	node.Discovered = true
	gs.CurrentNode = node
	gs.VisitedNodes[node.ID] = true
	gs.OpenCtx = openContextNode
	gs.MessageLog = append(gs.MessageLog, nodeInfo(node))
}

func persistSaveCmd(db *Database, gs *GameState) tea.Cmd {
	saveID := gs.SaveID
	currentNodeID := gs.CurrentNode.ID
	visited := gs.visitedList()
	deleted := gs.deletedFilesList()
	inventory := gs.inventoryIDs()
	claimed := gs.claimedList()
	stats := gs.Stats
	return func() tea.Msg {
		return gameSavedMsg{err: db.UpdateSave(saveID, currentNodeID, visited, deleted, inventory, claimed, stats)}
	}
}

func (m AppModel) updateGame(msg tea.Msg) (tea.Model, tea.Cmd) {
	gs := m.gs
	switch msg := msg.(type) {
	case gameSavedMsg:
		return m, nil

	case claimTickMsg:
		if !m.claim.active {
			return m, nil
		}
		m.claim.elapsed += 100 * time.Millisecond
		if m.claim.elapsed >= m.claim.total {
			// Claim complete — apply stats.
			m.claim.active = false
			node := gs.Network.Nodes[m.claim.nodeID]
			claimSkill := gs.Stats.ClaimSkill
			ramGain := max(1, int(float64(node.RAM)*float64(claimSkill)/100.0))
			cpuGain := max(1, int(float64(node.CPU)*float64(claimSkill)/100.0))
			gs.Stats.RAM += ramGain
			gs.Stats.CPU += cpuGain
			gs.ClaimedNodes[m.claim.nodeID] = true
			gs.MessageLog = append(gs.MessageLog,
				fmt.Sprintf("Claim complete. +%d RAM  +%d CPU", ramGain, cpuGain))
			return m, persistSaveCmd(m.db, gs)
		}
		return m, claimTickCmd()

	case bruteTickMsg:
		if !m.auth.active || m.auth.phase != authPhaseBrute {
			return m, nil
		}
		m.auth.elapsed += 100 * time.Millisecond
		if m.auth.elapsed >= m.auth.total {
			// Brute force complete — connect.
			node := m.auth.node
			m.auth = connectAuthState{}
			completeConnect(gs, node)
			return m, persistSaveCmd(m.db, gs)
		}
		return m, bruteTickCmd()

	case tea.KeyPressMsg:
		// Block input while a claim or brute force is in progress.
		if m.claim.active {
			return m, nil
		}
		// Route keypresses to auth screen when active.
		if m.auth.active {
			return m.updateAuth(msg)
		}

		// Intercept confirmation prompt before normal input handling.
		if m.awaitingQuitConfirm {
			switch msg.String() {
			case "y", "Y":
				m.awaitingQuitConfirm = false
				m.gs = nil
				m.screen = ScreenMainMenu
			case "n", "N", "esc":
				m.awaitingQuitConfirm = false
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "up":
			if len(gs.History) == 0 {
				return m, nil
			}
			if gs.HistoryIdx == -1 {
				gs.HistoryDraft = gs.Input.Value()
				gs.HistoryIdx = len(gs.History) - 1
			} else if gs.HistoryIdx > 0 {
				gs.HistoryIdx--
			}
			gs.Input.SetValue(gs.History[gs.HistoryIdx])
			gs.Input.CursorEnd()
			return m, nil
		case "down":
			if gs.HistoryIdx == -1 {
				return m, nil
			}
			gs.HistoryIdx++
			if gs.HistoryIdx >= len(gs.History) {
				gs.HistoryIdx = -1
				gs.Input.SetValue(gs.HistoryDraft)
			} else {
				gs.Input.SetValue(gs.History[gs.HistoryIdx])
			}
			gs.Input.CursorEnd()
			return m, nil
		case "tab":
			if completed, ok := gs.tabComplete(gs.Input.Value()); ok {
				gs.Input.SetValue(completed)
				gs.Input.CursorEnd()
			}
			return m, nil
		case "enter":
			input := strings.TrimSpace(gs.Input.Value())
			gs.Input.SetValue("")
			gs.HistoryIdx = -1
			gs.HistoryDraft = ""
			if input != "" {
				gs.History = append(gs.History, input)
				switch gs.handleCommand(input) {
				case actionPersist:
					return m, persistSaveCmd(m.db, gs)
				case actionQuit:
					m.awaitingQuitConfirm = true
				case actionConnectAuth:
					node := gs.PendingConnectNode
					gs.PendingConnectNode = nil
					pwInput := textinput.New()
					pwInput.Placeholder = "enter password..."
					pwInput.EchoMode = textinput.EchoPassword
					s := pwInput.Styles()
					s.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#39D353")).Bold(true)
					pwInput.SetStyles(s)
					pwInput.Prompt = "▶ "
					m.auth = connectAuthState{
						active:   true,
						node:     node,
						phase:    authPhaseMenu,
						menuSel:  0,
						hasSaved: gs.hasPasswordFor(node.ID),
						pwInput:  pwInput,
					}
					return m, nil
				case actionClaim:
					node := gs.CurrentNode
					playerCPU := gs.Stats.CPU
					if playerCPU < 1 {
						playerCPU = 1
					}
					secs := float64(node.CPU) / float64(playerCPU)
					if secs < 0.5 {
						secs = 0.5
					}
					m.claim = claimState{
						active:  true,
						nodeID:  node.ID,
						elapsed: 0,
						total:   time.Duration(secs * float64(time.Second)),
						bar:     progress.New(progress.WithDefaultBlend(), progress.WithWidth(40)),
					}
					return m, claimTickCmd()
				}
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	gs.Input, cmd = gs.Input.Update(msg)
	return m, cmd
}

// authMenuOpts returns the selectable option indices for the current auth state.
// Options: 0=type password, 1=use saved (if available), 2=brute force.
// Returns a slice of the option indices that are shown.
func (m AppModel) authMenuOpts() []int {
	if m.auth.hasSaved {
		return []int{0, 1, 2}
	}
	return []int{0, 2} // skip "use saved"
}

// updateAuth handles key input while the auth overlay is active.
func (m AppModel) updateAuth(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	gs := m.gs
	switch m.auth.phase {

	case authPhaseMenu:
		opts := m.authMenuOpts()
		switch msg.String() {
		case "esc":
			m.auth = connectAuthState{}
			gs.MessageLog = append(gs.MessageLog, "Connection cancelled.")
		case "up", "k":
			if m.auth.menuSel > 0 {
				m.auth.menuSel--
			}
		case "down", "j":
			if m.auth.menuSel < len(opts)-1 {
				m.auth.menuSel++
			}
		case "enter":
			opt := opts[m.auth.menuSel]
			switch opt {
			case 0: // type password
				m.auth.phase = authPhaseTyping
				m.auth.pwInput.Focus()
			case 1: // use saved password
				node := m.auth.node
				m.auth = connectAuthState{}
				gs.MessageLog = append(gs.MessageLog, "Using saved password...")
				completeConnect(gs, node)
				return m, persistSaveCmd(m.db, gs)
			case 2: // brute force
				return m.startBruteForce()
			}
		}
		return m, nil

	case authPhaseTyping:
		switch msg.String() {
		case "esc":
			m.auth.phase = authPhaseMenu
			return m, nil
		case "enter":
			typed := strings.TrimSpace(m.auth.pwInput.Value())
			m.auth.pwInput.SetValue("")
			if typed == m.auth.node.Password {
				node := m.auth.node
				m.auth = connectAuthState{}
				gs.MessageLog = append(gs.MessageLog, "Password accepted.")
				completeConnect(gs, node)
				return m, persistSaveCmd(m.db, gs)
			}
			gs.MessageLog = append(gs.MessageLog, "Incorrect password.")
			m.auth.phase = authPhaseMenu
			return m, nil
		}
		var cmd tea.Cmd
		m.auth.pwInput, cmd = m.auth.pwInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m AppModel) startBruteForce() (AppModel, tea.Cmd) {
	gs := m.gs
	node := m.auth.node
	secs := bruteEst(gs, node)
	m.auth.phase = authPhaseBrute
	m.auth.elapsed = 0
	m.auth.total = time.Duration(secs * float64(time.Second))
	m.auth.bar = progress.New(progress.WithDefaultBlend(), progress.WithWidth(40))
	gs.MessageLog = append(gs.MessageLog, fmt.Sprintf("Brute forcing %s...", node.Name))
	return m, bruteTickCmd()
}

func (m AppModel) viewAuth() string {
	gs := m.gs
	var b strings.Builder
	node := m.auth.node

	switch m.auth.phase {
	case authPhaseMenu:
		b.WriteString(styleWarn.Render(fmt.Sprintf("  ⚠  %s requires authentication.", node.Name)) + "\n")
		b.WriteString(styleDivider.Render("  "+strings.Repeat("─", 44)) + "\n")

		labels := map[int]string{
			0: "Enter password manually",
			1: "Use saved password",
			2: fmt.Sprintf("Brute force  (est. %.0fs)", bruteEst(gs, node)),
		}
		opts := m.authMenuOpts()
		for i, opt := range opts {
			sel := m.auth.menuSel == i
			var line string
			if sel {
				line = styleCmd.Render(fmt.Sprintf("  ▶ %s", labels[opt]))
			} else {
				line = styleNormal.Render(fmt.Sprintf("    %s", labels[opt]))
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n" + styleDetail.Render("  ↑↓  navigate  •  Enter  select  •  Esc  cancel"))

	case authPhaseTyping:
		b.WriteString(styleSection.Render(fmt.Sprintf("  Password for %s:", node.Name)) + "\n")
		b.WriteString("  " + m.auth.pwInput.View() + "\n")
		b.WriteString(styleDetail.Render("  Enter  submit  •  Esc  back"))

	case authPhaseBrute:
		pct := float64(m.auth.elapsed) / float64(m.auth.total)
		b.WriteString(styleSection.Render(fmt.Sprintf("  Brute forcing %s...", node.Name)) + "\n")
		b.WriteString("  " + m.auth.bar.ViewAs(pct) + "\n")
	}
	return b.String()
}

func bruteEst(gs *GameState, node *Node) float64 {
	combined := float64(node.CPU+node.RAM) / float64(gs.Stats.CPU+gs.Stats.RAM)
	secs := combined * 15.0
	if secs < 2.0 {
		secs = 2.0
	}
	return secs
}

func (m AppModel) viewGame() string {
	gs := m.gs
	var b strings.Builder
	b.WriteString(renderLog(gs.MessageLog))
	b.WriteByte('\n')
	if m.auth.active {
		b.WriteString(m.viewAuth())
	} else if m.claim.active {
		pct := float64(m.claim.elapsed) / float64(m.claim.total)
		b.WriteString(styleSection.Render("  Claiming "+gs.CurrentNode.Name+"...") + "\n")
		b.WriteString("  " + m.claim.bar.ViewAs(pct) + "\n")
	} else if m.awaitingQuitConfirm {
		b.WriteString(styleWarn.Render("  Return to main menu? [y/n]") + "\n")
	} else {
		b.WriteString(gs.Input.View() + "\n")
	}
	b.WriteString(renderStatusBar(gs))
	return b.String()
}
