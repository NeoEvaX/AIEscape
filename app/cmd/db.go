package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Save struct {
	ID             int64
	Name           string
	CurrentNodeID  string
	VisitedCount   int
	UpdatedAt      time.Time
	GameTime       time.Time
	Stats          PlayerStats
	ConnectCount   int
	AssimilateCount int
}

type Database struct {
	conn *sql.DB
}

func OpenDatabase(path string) (*Database, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	d := &Database{conn: conn}
	if err := d.migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	return d, nil
}

func (d *Database) migrate() error {
	stmts := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS saves (
			id               INTEGER  PRIMARY KEY AUTOINCREMENT,
			name             TEXT     UNIQUE NOT NULL,
			current_node_id  TEXT     NOT NULL,
			updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS visited_nodes (
			save_id  INTEGER NOT NULL REFERENCES saves(id) ON DELETE CASCADE,
			node_id  TEXT    NOT NULL,
			PRIMARY KEY (save_id, node_id)
		)`,
		// World-level item definitions synced from network.json on startup.
		`CREATE TABLE IF NOT EXISTS items (
			id       TEXT PRIMARY KEY,
			name     TEXT NOT NULL,
			type     TEXT NOT NULL,
			payload  TEXT NOT NULL
		)`,
		// Files the player has assimilated into their inventory.
		`CREATE TABLE IF NOT EXISTS save_items (
			save_id  INTEGER NOT NULL REFERENCES saves(id) ON DELETE CASCADE,
			item_id  TEXT    NOT NULL,
			PRIMARY KEY (save_id, item_id)
		)`,
		// Files the player has deleted from nodes (per-save).
		`CREATE TABLE IF NOT EXISTS save_deleted_node_files (
			save_id  INTEGER NOT NULL REFERENCES saves(id) ON DELETE CASCADE,
			item_id  TEXT    NOT NULL,
			PRIMARY KEY (save_id, item_id)
		)`,
		// Nodes the player has already claimed resources from (per-save).
		`CREATE TABLE IF NOT EXISTS save_claimed_nodes (
			save_id  INTEGER NOT NULL REFERENCES saves(id) ON DELETE CASCADE,
			node_id  TEXT    NOT NULL,
			PRIMARY KEY (save_id, node_id)
		)`,
		// Story events the player has already seen (per-save).
		`CREATE TABLE IF NOT EXISTS save_seen_events (
			save_id  INTEGER NOT NULL REFERENCES saves(id) ON DELETE CASCADE,
			event_id TEXT    NOT NULL,
			PRIMARY KEY (save_id, event_id)
		)`,
		// Emails the player has read (per-save).
		`CREATE TABLE IF NOT EXISTS save_read_emails (
			save_id  INTEGER NOT NULL REFERENCES saves(id) ON DELETE CASCADE,
			email_id TEXT    NOT NULL,
			PRIMARY KEY (save_id, email_id)
		)`,
	}
	for _, s := range stmts {
		if _, err := d.conn.Exec(s); err != nil {
			return fmt.Errorf("migration failed (%s...): %w", s[:min(40, len(s))], err)
		}
	}

	// Additive migrations — ignored if the column already exists.
	for _, s := range []string{
		`ALTER TABLE saves ADD COLUMN cpu              INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE saves ADD COLUMN claim_skill      INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE saves ADD COLUMN game_time        INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE saves ADD COLUMN connect_count    INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE saves ADD COLUMN assimilate_count INTEGER NOT NULL DEFAULT 0`,
	} {
		d.conn.Exec(s) // intentionally ignore "duplicate column" errors
	}

	return nil
}

// ── Items ─────────────────────────────────────────────────────────────────────

func (d *Database) UpsertItem(item Item) error {
	_, err := d.conn.Exec(
		`INSERT OR REPLACE INTO items (id, name, type, payload) VALUES (?, ?, ?, ?)`,
		item.ID, item.Name, string(item.Type), string(item.Payload),
	)
	return err
}

// UpsertItems upserts a batch of items in a single transaction.
func (d *Database) UpsertItems(items []Item) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO items (id, name, type, payload) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, item := range items {
		if _, err := stmt.Exec(item.ID, item.Name, string(item.Type), string(item.Payload)); err != nil {
			return fmt.Errorf("upserting item %q: %w", item.ID, err)
		}
	}
	return tx.Commit()
}

func (d *Database) GetInventory(saveID int64) ([]Item, error) {
	rows, err := d.conn.Query(`
		SELECT i.id, i.name, i.type, i.payload
		FROM items i
		JOIN save_items si ON si.item_id = i.id
		WHERE si.save_id = ?
		ORDER BY i.name
	`, saveID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		var payload string
		if err := rows.Scan(&item.ID, &item.Name, &item.Type, &payload); err != nil {
			return nil, err
		}
		item.Payload = []byte(payload)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *Database) GetDeletedNodeFiles(saveID int64) ([]string, error) {
	rows, err := d.conn.Query(
		`SELECT item_id FROM save_deleted_node_files WHERE save_id = ?`, saveID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (d *Database) GetClaimedNodes(saveID int64) ([]string, error) {
	rows, err := d.conn.Query(
		`SELECT node_id FROM save_claimed_nodes WHERE save_id = ?`, saveID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ── Saves ─────────────────────────────────────────────────────────────────────

func (d *Database) ListSaves() ([]Save, error) {
	rows, err := d.conn.Query(`
		SELECT s.id, s.name, s.current_node_id, s.updated_at, COUNT(v.node_id)
		FROM saves s
		LEFT JOIN visited_nodes v ON v.save_id = s.id
		GROUP BY s.id
		ORDER BY s.updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var saves []Save
	for rows.Next() {
		var s Save
		if err := rows.Scan(&s.ID, &s.Name, &s.CurrentNodeID, &s.UpdatedAt, &s.VisitedCount); err != nil {
			return nil, err
		}
		saves = append(saves, s)
	}
	return saves, rows.Err()
}

func (d *Database) CreateSave(name, currentNodeID string, visitedNodeIDs []string, gameTime time.Time) (int64, error) {
	tx, err := d.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO saves (name, current_node_id, updated_at, game_time) VALUES (?, ?, ?, ?)`,
		name, currentNodeID, time.Now(), gameTime.Unix(),
	)
	if err != nil {
		return 0, fmt.Errorf("creating save %q: %w", name, err)
	}
	id, _ := res.LastInsertId()

	for _, nodeID := range visitedNodeIDs {
		if _, err := tx.Exec(`INSERT INTO visited_nodes (save_id, node_id) VALUES (?, ?)`, id, nodeID); err != nil {
			return 0, err
		}
	}

	return id, tx.Commit()
}

func (d *Database) LoadSave(id int64) (*Save, []string, error) {
	var s Save
	var gameTimeUnix int64
	err := d.conn.QueryRow(
		`SELECT id, name, current_node_id, updated_at, cpu, claim_skill, game_time, connect_count, assimilate_count FROM saves WHERE id = ?`, id,
	).Scan(&s.ID, &s.Name, &s.CurrentNodeID, &s.UpdatedAt, &s.Stats.CPU, &s.Stats.ClaimSkill, &gameTimeUnix, &s.ConnectCount, &s.AssimilateCount)
	if gameTimeUnix == 0 {
		s.GameTime = gameStartTime
	} else {
		s.GameTime = time.Unix(gameTimeUnix, 0).UTC()
	}
	if err != nil {
		return nil, nil, fmt.Errorf("loading save: %w", err)
	}

	rows, err := d.conn.Query(`SELECT node_id FROM visited_nodes WHERE save_id = ?`, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var visited []string
	for rows.Next() {
		var nodeID string
		if err := rows.Scan(&nodeID); err != nil {
			return nil, nil, err
		}
		visited = append(visited, nodeID)
	}
	return &s, visited, rows.Err()
}

// UpdateSave persists all mutable save state in a single transaction.
func (d *Database) UpdateSave(saveID int64, data SaveData) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE saves SET current_node_id = ?, updated_at = ?, cpu = ?, claim_skill = ?, game_time = ?, connect_count = ?, assimilate_count = ? WHERE id = ?`,
		data.CurrentNodeID, time.Now(), data.Stats.CPU, data.Stats.ClaimSkill, data.GameTime.Unix(), data.ConnectCount, data.AssimilateCount, saveID,
	); err != nil {
		return err
	}

	replaceList(tx, `visited_nodes`, `node_id`, saveID, data.Visited)
	replaceList(tx, `save_deleted_node_files`, `item_id`, saveID, data.DeletedFiles)
	replaceList(tx, `save_items`, `item_id`, saveID, data.InventoryIDs)
	replaceList(tx, `save_claimed_nodes`, `node_id`, saveID, data.ClaimedNodes)
	replaceList(tx, `save_seen_events`, `event_id`, saveID, data.SeenEvents)
	replaceList(tx, `save_read_emails`, `email_id`, saveID, data.ReadEmails)

	return tx.Commit()
}

func (d *Database) GetSeenEvents(saveID int64) ([]string, error) {
	rows, err := d.conn.Query(
		`SELECT event_id FROM save_seen_events WHERE save_id = ?`, saveID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (d *Database) GetReadEmails(saveID int64) ([]string, error) {
	rows, err := d.conn.Query(
		`SELECT email_id FROM save_read_emails WHERE save_id = ?`, saveID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// replaceList deletes all rows for saveID in table and reinserts them.
func replaceList(tx *sql.Tx, table, col string, saveID int64, values []string) {
	tx.Exec(`DELETE FROM `+table+` WHERE save_id = ?`, saveID)
	for _, v := range values {
		tx.Exec(`INSERT OR IGNORE INTO `+table+` (save_id, `+col+`) VALUES (?, ?)`, saveID, v)
	}
}

func (d *Database) DeleteSave(id int64) error {
	_, err := d.conn.Exec(`DELETE FROM saves WHERE id = ?`, id)
	return err
}

func (d *Database) Close() error {
	return d.conn.Close()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
