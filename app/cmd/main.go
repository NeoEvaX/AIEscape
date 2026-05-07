package main

import (
	"log"

	tea "charm.land/bubbletea/v2"
)

func main() {
	network, startNode, err := loadNetwork("network.json")
	if err != nil {
		log.Fatal(err)
	}

	p := tea.NewProgram(NewModel(network, startNode))

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
