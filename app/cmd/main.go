package main

import (
	"log"

	tea "charm.land/bubbletea/v2"
)

func main() {
	db, err := OpenDatabase("saves.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	network, err := loadNetwork("network.json")
	if err != nil {
		log.Fatal(err)
	}

	p := tea.NewProgram(NewAppModel(db, network))
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
