package hyprland

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"time"

	"orego/pkg/models"
)

// HyprMonitor represents the JSON output from 'hyprctl monitors -j'
type HyprMonitor struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Focused         bool   `json:"focused"`
	ActiveWorkspace struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"activeWorkspace"`
}

// HyprWindow represents the JSON output from 'hyprctl activewindow -j' and 'hyprctl clients -j'
type HyprWindow struct {
	Address   string `json:"address"`
	Class     string `json:"class"`
	Title     string `json:"title"`
	Pid       int    `json:"pid"`
	Workspace struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"workspace"`
	Floating   bool `json:"floating"`
	Fullscreen int  `json:"fullscreen"` // 0: no, 1: maximize, 2: fullscreen
	Xwayland   bool `json:"xwayland"`
	Pinned     bool `json:"pinned"`
}

func runHyprctl(args ...string) ([]byte, error) {
	cmd := exec.Command("hyprctl", args...)
	return cmd.Output()
}

func GetScreenshotData(captureAll bool) (*models.Screenshot, error) {
	rawActive, err := runHyprctl("activewindow", "-j")
	if err != nil {
		return nil, fmt.Errorf("failed to get active window: %w", err)
	}
	var activeWin HyprWindow
	if len(rawActive) > 0 && string(rawActive) != "{}" {
		// Ignore error here as empty active window is possible (e.g. desktop focused)
		_ = json.Unmarshal(rawActive, &activeWin)
	}

	rawMonitors, err := runHyprctl("monitors", "-j")
	if err != nil {
		return nil, fmt.Errorf("failed to get monitors: %w", err)
	}
	var monitors []HyprMonitor
	if err := json.Unmarshal(rawMonitors, &monitors); err != nil {
		return nil, fmt.Errorf("failed to parse monitors: %w", err)
	}

	var activeMon HyprMonitor
	visibleWorkspaceIDs := make(map[int]bool)
	foundMon := false
	
	for _, m := range monitors {
		visibleWorkspaceIDs[m.ActiveWorkspace.ID] = true
		if m.Focused {
			activeMon = m
			foundMon = true
		}
	}
	if !foundMon && len(monitors) > 0 {
		activeMon = monitors[0] // Fallback
	}

	rawClients, err := runHyprctl("clients", "-j")
	if err != nil {
		return nil, fmt.Errorf("failed to get clients: %w", err)
	}
	var allClients []HyprWindow
	if err := json.Unmarshal(rawClients, &allClients); err != nil {
		return nil, fmt.Errorf("failed to parse clients: %w", err)
	}

	var workspaceClients []models.Client
	var windowCount int
	var lastWindowTitle string

	targetWorkspaceID := activeMon.ActiveWorkspace.ID

	for _, c := range allClients {
		shouldInclude := false
		if captureAll {
			if visibleWorkspaceIDs[c.Workspace.ID] {
				shouldInclude = true
			}
		} else {
			if c.Workspace.ID == targetWorkspaceID {
				shouldInclude = true
			}
		}

		if shouldInclude {
			windowCount++
			lastWindowTitle = c.Title

			workspaceClients = append(workspaceClients, models.Client{
				Address:     c.Address,
				Class:       c.Class,
				Title:       c.Title,
				Pid:         c.Pid,
				WorkspaceID: c.Workspace.ID,
			})
		}
	}

	hostname, _ := os.Hostname()
	currentUser, _ := user.Current()
	username := "unknown"
	if currentUser != nil {
		username = currentUser.Username
	}
	
	monitorName := activeMon.Name
	if captureAll {
		monitorName = "all-visible"
	}

	data := &models.Screenshot{
		Capture: models.CaptureMetadata{
			Ts:       time.Now(),
			Timezone: "UTC", // Will be updated below
			Hostname: hostname,
			User:     username,
			Command:  "orego capture",
			Version:  "0.1.0",
		},
		ActiveWindow: models.ActiveWindow{
			Address: activeWin.Address,
			Class:   activeWin.Class,
			Title:   activeWin.Title,
			Pid:     activeWin.Pid,
			State: models.WindowState{
				Floating:   activeWin.Floating,
				Fullscreen: activeWin.Fullscreen,
				Xwayland:   activeWin.Xwayland,
				Pinned:     activeWin.Pinned,
			},
		},
		Workspace: models.Workspace{
			ID:              activeMon.ActiveWorkspace.ID,
			Name:            activeMon.ActiveWorkspace.Name,
			Monitor:         monitorName,
			Windows:         windowCount,
			HasFullscreen:   activeWin.Fullscreen > 0 && activeWin.Workspace.ID == activeMon.ActiveWorkspace.ID,
			LastWindowTitle: lastWindowTitle,
		},
		Clients: workspaceClients,
	}

	tzName, _ := time.Now().Zone()
	data.Capture.Timezone = tzName

	return data, nil
}
