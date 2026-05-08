# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build    # compile to bin/ai-escape
make run      # build and run (expects network.json and saves.db in working directory)
make test     # go test -v ./...
```

## Architecture

AIEscape is a terminal-based hacker game built with the [BubbleTea v2](https://charm.land/bubbletea) TUI framework (Elm architecture). The game loads a network map from `network.json` and persists save state to `saves.db` (SQLite).

### File overview

| File | Responsibility |
|---|---|
| `app/cmd/main.go` | Entry point: open DB, load network, sync items, run program |
| `app/cmd/app.go` | `AppModel` — top-level BubbleTea model, manages screen transitions |
| `app/cmd/model.go` | `GameState`, `Node`, `Network` types + all game command logic |
| `app/cmd/items.go` | `Item`, `ItemType` constants, typed payload structs + accessors |
| `app/cmd/db.go` | `Database` wrapper, all SQLite operations |
| `app/cmd/loader.go` | Parses `network.json`; `syncWorldItems` upserts node files to DB |
| `network.json` | World data: nodes, connections, and files placed on each node |
| `saves.db` | SQLite save file (auto-created on first run) |

### BubbleTea v2 API notes

- `Model.View()` returns `tea.View`, not `string` — use `tea.NewView(s)` to wrap a string.
- Key messages are `tea.KeyPressMsg` (not `tea.KeyMsg` from v1); match with `msg.String()`.
- `tea.Cmd` functions are the correct way to do I/O (DB reads/writes) — return them from `Update`, never call DB directly inside `Update`.

### Screen system (`app.go`)

`AppModel` holds a `screen Screen` field and delegates `Update`/`View` to per-screen methods:

- `ScreenMainMenu` — nav with ↑↓/j/k, Enter selects
- `ScreenNewGame` — text input for save name
- `ScreenLoadSave` — list saves; Enter loads, D deletes
- `ScreenGame` — the main game loop

Screen transitions happen via custom message types (`saveCreatedMsg`, `saveLoadedMsg`, etc.) returned as `tea.Cmd` results.

### Game state (`model.go`)

`GameState` holds all runtime state for an active session:

- `Network` / `CurrentNode` — graph traversal
- `VisitedNodes map[string]bool` — nodes the player has been to
- `DeletedNodeFiles map[string]bool` — item IDs deleted from nodes in this save
- `Inventory []Item` — files the player has assimilated
- `Input textinput.Model` — the command prompt

`handleCommand(input string) gameAction` parses player input and returns `actionNone`, `actionPersist`, or `actionQuit`. Any `actionPersist` triggers `persistSaveCmd`, which atomically writes visited nodes, deleted files, and inventory to SQLite in one transaction.

### World data (`network.json`)

```jsonc
{
  "start": "1",
  "nodes": [
    {
      "id": "1",
      "name": "Entry Point",
      "description": "...",
      "connections": ["2", "3"],
      "files": [
        { "id": "f-1-1", "name": "readme.txt", "type": "text_file", "payload": {"text": "..."} }
      ]
    }
  ]
}
```

File IDs use the convention `f-{nodeId}-{n}`. Changing this file and restarting the app updates the world without rebuilding. `syncWorldItems` upserts all node files into the `items` table on every startup.

### Item types and payloads

| Type | `ItemType` constant | Payload fields |
|---|---|---|
| Text File | `ItemTypeTextFile` | `{"text": "..."}` |
| Application | `ItemTypeApplication` | `{"text": "...", "action": "..."}` |
| Certificate | `ItemTypeCertificate` | `{"code": "..."}` |
| Network Location | `ItemTypeNetworkLocation` | `{"node_id": "..."}` |

Use `item.AsTextFile()`, `item.AsApplication()`, etc. to unmarshal payloads.

### Database schema

| Table | Purpose |
|---|---|
| `saves` | Save metadata (name, current node, timestamp) |
| `visited_nodes` | Nodes visited per save |
| `items` | World item definitions (synced from network.json) |
| `save_items` | Player inventory per save |
| `save_deleted_node_files` | Node files deleted per save |

All child tables use `ON DELETE CASCADE` from `saves`. `UpdateSave` replaces all child rows atomically.

### Game commands

| Command | Action |
|---|---|
| `scan` | List connected node IDs |
| `connect <id>` | Move to an adjacent node |
| `ls` | List files on the current node |
| `assimilate <id>` | Copy a node file into inventory |
| `delete <id>` | Permanently remove a file from this node |
| `inventory` / `inv` | List assimilated files |
| `open <id>` | Display text content of an inventory file |
| `rm <id>` | Remove a file from inventory |
| `quit` / `exit` | Return to main menu (with confirmation) |
| `help` / `?` | Show all commands |
