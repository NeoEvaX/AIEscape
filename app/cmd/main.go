package main

import (
	"log"

	tea "charm.land/bubbletea/v2"
)

func main() {
	p := tea.NewProgram(NewLoadingModel())
	finalModel, err := p.Run()
	if err != nil {
		log.Fatal(err)
	}
	// If an error occurred during loading, the LoadingModel stays on screen
	// with the error displayed; close any open DB on exit.
	if lm, ok := finalModel.(LoadingModel); ok && lm.db != nil {
		lm.db.Close()
	}
	if am, ok := finalModel.(AppModel); ok && am.db != nil {
		am.db.Close()
	}
}
