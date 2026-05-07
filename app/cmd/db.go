package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Save struct {
	ID            int64
	Name          string
	CurrentNodeID string
	VisitedCount  int
	UpdatedAt     time.Time
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
	_, err := d.conn.Exec(`
		CREATE TABLE IF NOT EXISTS saves (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			name           TEXT    UNIQUE NOT NULL,
			current_node_id TEXT   NOT NULL,
			updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS visited_nodes (
			save_id INTEGER NOT NULL REFERENCES saves(id) ON DELETE CASCADE,
			node_id TEXT    NOT NULL,
			PRIMARY KEY (save_id, node_id)
		);
		PRAGMA foreign_keys = ON;
	`)
	return err
}

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

func (d *Database) CreateSave(name, currentNodeID string, visitedNodeIDs []string) (int64, error) {
	tx, err := d.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO saves (name, current_node_id, updated_at) VALUES (?, ?, ?)`,
		name, currentNodeID, time.Now(),
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
	err := d.conn.QueryRow(
		`SELECT id, name, current_node_id, updated_at FROM saves WHERE id = ?`, id,
	).Scan(&s.ID, &s.Name, &s.CurrentNodeID, &s.UpdatedAt)
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

func (d *Database) UpdateSave(id int64, currentNodeID string, visitedNodeIDs []string) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE saves SET current_node_id = ?, updated_at = ? WHERE id = ?`,
		currentNodeID, time.Now(), id,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM visited_nodes WHERE save_id = ?`, id); err != nil {
		return err
	}
	for _, nodeID := range visitedNodeIDs {
		if _, err := tx.Exec(`INSERT INTO visited_nodes (save_id, node_id) VALUES (?, ?)`, id, nodeID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *Database) DeleteSave(id int64) error {
	_, err := d.conn.Exec(`DELETE FROM saves WHERE id = ?`, id)
	return err
}

func (d *Database) Close() error {
	return d.conn.Close()
}
