package main

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

type Screen int

const (
	ScreenMainMenu Screen = iota
	ScreenNewGame
	ScreenLoadSave
	ScreenGame
)

var mainMenuItems = []string{"New Game", "Load Game", "Quit"}

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
	gs                 *GameState
	awaitingQuitConfirm bool
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
	b.WriteString("\n  AI ESCAPE\n\n")
	for i, item := range mainMenuItems {
		if i == m.menuCursor {
			b.WriteString("  > " + item + "\n")
		} else {
			b.WriteString("    " + item + "\n")
		}
	}
	b.WriteString("\n  ↑↓  navigate  •  Enter  select")
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
	b.WriteString("\n  NEW GAME\n\n")
	b.WriteString("  Save name: " + m.nameInput.View() + "\n")
	if m.nameErr != "" {
		b.WriteString("\n  " + m.nameErr + "\n")
	}
	b.WriteString("\n  Enter  start  •  Esc  back")
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
		// stats are embedded in save via LoadSave
		return saveLoadedMsg{save: save, visited: visited, inventory: inventory, deletedNodeFiles: deleted, err: err}
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
		m.gs = newGameStateFromSave(m.network, msg.save, currentNode, msg.visited, msg.deletedNodeFiles, msg.inventory, msg.save.Stats)
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
	b.WriteString("\n  LOAD GAME\n\n")
	if m.savesErr != "" {
		b.WriteString("  Error: " + m.savesErr + "\n\n")
	}
	if len(m.saves) == 0 && m.savesErr == "" {
		b.WriteString("  No saves found.\n")
	}
	for i, s := range m.saves {
		cursor := "    "
		if i == m.saveCursor {
			cursor = "  > "
		}
		b.WriteString(fmt.Sprintf("%s%-24s  %3d nodes visited  •  %s\n",
			cursor, s.Name, s.VisitedCount, s.UpdatedAt.Format("2006-01-02 15:04")))
	}
	b.WriteString("\n  Enter  load  •  D  delete  •  Esc  back")
	return b.String()
}

// ── Game ──────────────────────────────────────────────────────────────────────

type gameSavedMsg struct{ err error }

func persistSaveCmd(db *Database, gs *GameState) tea.Cmd {
	saveID := gs.SaveID
	currentNodeID := gs.CurrentNode.ID
	visited := gs.visitedList()
	deleted := gs.deletedFilesList()
	inventory := gs.inventoryIDs()
	stats := gs.Stats
	return func() tea.Msg {
		return gameSavedMsg{err: db.UpdateSave(saveID, currentNodeID, visited, deleted, inventory, stats)}
	}
}

func (m AppModel) updateGame(msg tea.Msg) (tea.Model, tea.Cmd) {
	gs := m.gs
	switch msg := msg.(type) {
	case gameSavedMsg:
		return m, nil
	case tea.KeyPressMsg:
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
		case "tab":
			if completed, ok := gs.tabComplete(gs.Input.Value()); ok {
				gs.Input.SetValue(completed)
				gs.Input.CursorEnd()
			}
			return m, nil
		case "enter":
			input := strings.TrimSpace(gs.Input.Value())
			gs.Input.SetValue("")
			if input != "" {
				switch gs.handleCommand(input) {
				case actionPersist:
					return m, persistSaveCmd(m.db, gs)
				case actionQuit:
					m.awaitingQuitConfirm = true
				}
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	gs.Input, cmd = gs.Input.Update(msg)
	return m, cmd
}

func (m AppModel) viewGame() string {
	gs := m.gs
	var b strings.Builder
	b.WriteString(fmt.Sprintf("  [%s]  visited: %d / %d nodes\n\n",
		gs.SaveName, len(gs.VisitedNodes), len(gs.Network.Nodes)))
	for _, line := range gs.MessageLog {
		b.WriteString(line + "\n")
	}
	b.WriteByte('\n')
	if m.awaitingQuitConfirm {
		b.WriteString("  Return to main menu? [y/n] ")
	} else {
		b.WriteString(gs.Input.View())
	}
	return b.String()
}
