# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build    # compile to bin/ai-escape
make run      # build and run
make test     # go test -v ./...
```

## Architecture

AIEscape is a terminal-based game built with the [BubbleTea](https://charm.land/bubbletea) TUI framework (v2), which follows the Elm architecture:

- **`app/cmd/main.go`** — entry point; creates and runs the BubbleTea program
- **`app/cmd/model.go`** — defines `GameModel` and all game types; `GameModel` must implement `Init()`, `Update(tea.Msg) tea.Cmd`, and `View() string` to satisfy the BubbleTea `Model` interface

### Key types

- **`GameModel`** — root model holding all state: navigation (current node in a network), UI components (text input, viewport), and message log
- **`Network`** — a graph of `Node` objects keyed by ID; `CanReach(from, to)` checks direct adjacency
- **`Node`** — a location in the game world with connections to other nodes

### UI components (charm.land/bubbles/v2)

- `textinput.Model` — player command input
- `viewport.Model` — scrollable content area (message log / room description)

The `Update` and `View` methods (not yet implemented) belong in `model.go` alongside the existing `Init`.
