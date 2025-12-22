package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"orego/pkg/models"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	s := &Store{db: db}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) init() error {
	queryScreenshots := `
	CREATE TABLE IF NOT EXISTS screenshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_path TEXT NOT NULL,
		
		-- Capture Metadata
		capture_ts DATETIME,
		capture_timezone TEXT,
		capture_hostname TEXT,
		capture_user TEXT,
		capture_command TEXT,
		capture_version TEXT,
		
		-- Active Window
		active_window_address TEXT,
		active_window_class TEXT,
		active_window_title TEXT,
		active_window_pid INTEGER,
		active_window_floating BOOLEAN,
		active_window_fullscreen INTEGER,
		active_window_xwayland BOOLEAN,
		active_window_pinned BOOLEAN,
		
		-- Workspace
		workspace_id INTEGER,
		workspace_name TEXT,
		workspace_monitor TEXT,
		workspace_windows INTEGER,
		workspace_has_fullscreen BOOLEAN,
		workspace_last_window_title TEXT
	);`

	// Table for clients (one-to-many)
	queryClients := `
	CREATE TABLE IF NOT EXISTS clients (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		screenshot_id INTEGER,
		address TEXT,
		class TEXT,
		title TEXT,
		pid INTEGER,
		workspace_id INTEGER,
		FOREIGN KEY(screenshot_id) REFERENCES screenshots(id) ON DELETE CASCADE
	);`

	if _, err := s.db.Exec(queryScreenshots); err != nil {
		return fmt.Errorf("failed to create screenshots table: %w", err)
	}
	if _, err := s.db.Exec(queryClients); err != nil {
		return fmt.Errorf("failed to create clients table: %w", err)
	}
	return nil
}

func (s *Store) Save(sc *models.Screenshot) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		INSERT INTO screenshots (
			file_path, capture_ts, capture_timezone, capture_hostname, capture_user, capture_command, capture_version,
			active_window_address, active_window_class, active_window_title, active_window_pid,
			active_window_floating, active_window_fullscreen, active_window_xwayland, active_window_pinned,
			workspace_id, workspace_name, workspace_monitor, workspace_windows, workspace_has_fullscreen, workspace_last_window_title
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sc.FilePath, sc.Capture.Ts, sc.Capture.Timezone, sc.Capture.Hostname, sc.Capture.User, sc.Capture.Command, sc.Capture.Version,
		sc.ActiveWindow.Address, sc.ActiveWindow.Class, sc.ActiveWindow.Title, sc.ActiveWindow.Pid,
		sc.ActiveWindow.State.Floating, sc.ActiveWindow.State.Fullscreen, sc.ActiveWindow.State.Xwayland, sc.ActiveWindow.State.Pinned,
		sc.Workspace.ID, sc.Workspace.Name, sc.Workspace.Monitor, sc.Workspace.Windows, sc.Workspace.HasFullscreen, sc.Workspace.LastWindowTitle,
	)
	if err != nil {
		return fmt.Errorf("failed to insert screenshot: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	sc.ID = id

	for _, client := range sc.Clients {
		_, err := tx.Exec(`
			INSERT INTO clients (screenshot_id, address, class, title, pid, workspace_id)
			VALUES (?, ?, ?, ?, ?, ?)`,
			id, client.Address, client.Class, client.Title, client.Pid, client.WorkspaceID,
		)
		if err != nil {
			return fmt.Errorf("failed to insert client: %w", err)
		}
	}

	return tx.Commit()
}

func (s *Store) ListScreenshots(limit int, filterField, filterValue string) ([]models.Screenshot, error) {
	baseQuery := `
	SELECT 
		id, file_path, 
		capture_ts, capture_timezone, capture_hostname, capture_user, capture_command, capture_version,
		active_window_address, active_window_class, active_window_title, active_window_pid,
		workspace_id, workspace_name, workspace_monitor
	FROM screenshots`

	var args []interface{}
	
	// Whitelist filter fields to prevent injection
	fieldMap := map[string]string{
		"app":   "active_window_class",
		"title": "active_window_title",
	}

	if dbField, ok := fieldMap[filterField]; ok && filterValue != "" {
		baseQuery += fmt.Sprintf(" WHERE %s LIKE ?", dbField)
		args = append(args, "%"+filterValue+"%")
	}

	baseQuery += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query screenshots: %w", err)
	}
	defer rows.Close()

	var results []models.Screenshot
	for rows.Next() {
		var sc models.Screenshot
		var ts time.Time

		err := rows.Scan(
			&sc.ID, &sc.FilePath,
			&ts, &sc.Capture.Timezone, &sc.Capture.Hostname, &sc.Capture.User, &sc.Capture.Command, &sc.Capture.Version,
			&sc.ActiveWindow.Address, &sc.ActiveWindow.Class, &sc.ActiveWindow.Title, &sc.ActiveWindow.Pid,
			&sc.Workspace.ID, &sc.Workspace.Name, &sc.Workspace.Monitor,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan screenshot: %w", err)
		}
		sc.Capture.Ts = ts
		results = append(results, sc)
	}
	return results, nil
}

func (s *Store) ListAllPaths() (map[int64]string, error) {
	rows, err := s.db.Query("SELECT id, file_path FROM screenshots")
	if err != nil {
		return nil, fmt.Errorf("failed to query paths: %w", err)
	}
	defer rows.Close()

	paths := make(map[int64]string)
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			return nil, err
		}
		paths[id] = path
	}
	return paths, nil
}

func (s *Store) DeleteScreenshot(id int64) error {
	_, err := s.db.Exec("DELETE FROM screenshots WHERE id = ?", id)
	return err
}

func (s *Store) GetScreenshotPath(id int64) (string, error) {
	var path string
	err := s.db.QueryRow("SELECT file_path FROM screenshots WHERE id = ?", id).Scan(&path)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("screenshot with ID %d not found", id)
	}
	return path, err
}
