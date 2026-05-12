package main

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
)

// ── Messages ──────────────────────────────────────────────────────────────────

type dbOpenedMsg struct {
	db  *Database
	err error
}

type networkLoadedMsg struct {
	network *Network
	err     error
}

type syncDoneMsg struct {
	story *StoryCollection
	err   error
}

type readyMsg struct{}

// ── Commands ──────────────────────────────────────────────────────────────────

func openDBCmd() tea.Cmd {
	return func() tea.Msg {
		db, err := OpenDatabase("saves.db")
		return dbOpenedMsg{db: db, err: err}
	}
}

func loadNetworkCmd() tea.Cmd {
	return func() tea.Msg {
		var (
			network *Network
			err     error
		)
		if len(embeddedNetwork) > 0 {
			network, err = loadNetworkFromBytes(embeddedNetwork)
		} else {
			network, err = loadNetwork("network.json")
		}
		return networkLoadedMsg{network: network, err: err}
	}
}

func syncItemsCmd(db *Database, network *Network) tea.Cmd {
	return func() tea.Msg {
		if err := syncWorldItems(db, network); err != nil {
			return syncDoneMsg{err: err}
		}
		var (
			story *StoryCollection
			err   error
		)
		if len(embeddedStory) > 0 {
			story, err = loadStoryFromBytes(embeddedStory)
		} else {
			story, err = loadStory("story.json")
		}
		return syncDoneMsg{story: story, err: err}
	}
}

func readyCmd() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(350 * time.Millisecond) // let bar finish animating to 100%
		return readyMsg{}
	}
}

// ── Model ─────────────────────────────────────────────────────────────────────

type LoadingModel struct {
	bar     progress.Model
	status  string
	db      *Database
	network *Network
	story   *StoryCollection
	err     error
}

func NewLoadingModel() LoadingModel {
	return LoadingModel{
		bar:    progress.New(progress.WithDefaultBlend(), progress.WithWidth(50)),
		status: "Opening database...",
	}
}

func (m LoadingModel) Init() tea.Cmd {
	return tea.Batch(openDBCmd(), m.bar.SetPercent(0.05))
}

func (m LoadingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case progress.FrameMsg:
		bar, cmd := m.bar.Update(msg)
		m.bar = bar
		return m, cmd

	case dbOpenedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.db = msg.db
		m.status = "Loading network..."
		return m, tea.Batch(m.bar.SetPercent(0.33), loadNetworkCmd())

	case networkLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.network = msg.network
		m.status = "Syncing world items..."
		return m, tea.Batch(m.bar.SetPercent(0.66), syncItemsCmd(m.db, m.network))

	case syncDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.story = msg.story
		m.status = "Ready."
		return m, tea.Batch(m.bar.SetPercent(1.0), readyCmd())

	case readyMsg:
		return NewAppModel(m.db, m.network, m.story), nil

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m LoadingModel) View() tea.View {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styleNodeHeader.Render("  ▲ AI ESCAPE") + "\n")
	b.WriteString(styleDetail.Render("  ─────────────────────────────────────────────────────") + "\n\n")
	if m.err != nil {
		b.WriteString(styleWarn.Render("  Error: "+m.err.Error()) + "\n")
	} else {
		b.WriteString(styleDetail.Render("  "+m.status) + "\n\n")
		b.WriteString("  " + m.bar.View() + "\n")
	}
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}
