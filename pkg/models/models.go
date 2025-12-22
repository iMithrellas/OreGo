package models

import "time"

type CaptureMetadata struct {
	Ts       time.Time `json:"ts"`
	Timezone string    `json:"timezone"`
	Hostname string    `json:"hostname"`
	User     string    `json:"user"`
	Command  string    `json:"command"`
	Version  string    `json:"version"`
}

type WindowState struct {
	Floating   bool `json:"floating"`
	Fullscreen int  `json:"fullscreen"`
	Xwayland   bool `json:"xwayland"`
	Pinned     bool `json:"pinned"`
}

type ActiveWindow struct {
	Address string      `json:"address"`
	Class   string      `json:"class"`
	Title   string      `json:"title"`
	Pid     int         `json:"pid"`
	State   WindowState `json:"state"`
}

type Workspace struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Monitor         string `json:"monitor"`
	Windows         int    `json:"windows"`
	HasFullscreen   bool   `json:"has_fullscreen"`
	LastWindowTitle string `json:"last_window_title"`
}

type Client struct {
	Address     string `json:"address"`
	Class       string `json:"class"`
	Title       string `json:"title"`
	Pid         int    `json:"pid"`
	WorkspaceID int    `json:"workspace"`
}

// Screenshot represents the aggregate data for a single capture.
type Screenshot struct {
	ID           int64           `json:"id"` // Database ID
	FilePath     string          `json:"file_path"`
	Capture      CaptureMetadata `json:"capture"`
	ActiveWindow ActiveWindow    `json:"active_window"`
	Workspace    Workspace       `json:"workspace"`
	Clients      []Client        `json:"clients"`
}
