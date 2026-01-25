package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	lipglossv2 "github.com/charmbracelet/lipgloss/v2"

	"orego/internal/db"
	"orego/pkg/models"
)

func RenderTable(store *db.Store) error {
	// Fetch initial data
	entries, err := store.ListScreenshots(0, "", "")
	if err != nil {
		return err
	}

	m := model{
		store:     store,
		entries:   entries,
		showIdx:   -1,
		deleteIdx: -1,
		keys:      newKeyMap(),
		help:      help.New(),
	}
	m.help.ShowAll = true
	m.initTable()

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

type model struct {
	store     *db.Store
	table     table.Model
	entries   []models.Screenshot
	showIdx   int
	deleteIdx int
	width     int
	height    int
	status    string
	keys      keyMap
	help      help.Model
	showHelp  bool
}

type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	Open       key.Binding
	CopyImage  key.Binding
	OpenFolder key.Binding
	CopyFolder key.Binding
	Delete     key.Binding
	Help       key.Binding
	Quit       key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "move down"),
		),
		Open: key.NewBinding(
			key.WithKeys("enter", "i"),
			key.WithHelp("enter/i", "open"),
		),
		CopyImage: key.NewBinding(
			key.WithKeys("c", "y"),
			key.WithHelp("c/y", "copy image"),
		),
		OpenFolder: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "open folder"),
		),
		CopyFolder: key.NewBinding(
			key.WithKeys("C", "Y"),
			key.WithHelp("C/Y", "copy path"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q/esc", "quit"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Open, k.CopyImage},
		{k.OpenFolder, k.CopyFolder, k.Delete},
		{k.Help, k.Quit},
	}
}

func (m *model) initTable() {
	cols := []table.Column{
		{Title: "ID", Width: 4},
		{Title: "Time", Width: 16},
		{Title: "App", Width: 20},
		{Title: "Title", Width: 40},
	}
	m.table = table.New(table.WithColumns(cols), table.WithFocused(true))
	m.updateRows()
	m.applyStyles()
}

func (m *model) updateRows() {
	rows := make([]table.Row, 0, len(m.entries))
	for _, e := range m.entries {
		ts := e.Capture.Ts.Local().Format("2006-01-02 15:04")
		rows = append(rows, table.Row{
			fmt.Sprintf("%d", e.ID),
			ts,
			e.ActiveWindow.Class,
			e.ActiveWindow.Title,
		})
	}
	m.table.SetRows(rows)
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applyLayout()
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp
			return m, nil
		case key.Matches(msg, m.keys.Open):
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.entries) {
				sel := m.entries[idx]
				_ = exec.Command("xdg-open", sel.FilePath).Start()
				m.status = fmt.Sprintf("Opened %s", sel.FilePath)
			}
			return m, nil
		case key.Matches(msg, m.keys.CopyImage):
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.entries) {
				sel := m.entries[idx]
				file, err := os.Open(sel.FilePath)
				if err != nil {
					if os.IsNotExist(err) {
						m.status = fmt.Sprintf("Missing file: %s", sel.FilePath)
					} else {
						m.status = fmt.Sprintf("Open failed: %v", err)
					}
					return m, nil
				}
				defer file.Close()

				copyCmd := exec.Command("wl-copy", "--type", "image/png")
				copyCmd.Stdin = file
				if err := copyCmd.Run(); err != nil {
					m.status = fmt.Sprintf("Copy failed: %v", err)
					return m, nil
				}
				m.status = "Copied to clipboard"
			}
			return m, nil
		case key.Matches(msg, m.keys.OpenFolder):
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.entries) {
				sel := m.entries[idx]
				if _, err := os.Stat(sel.FilePath); err != nil {
					if os.IsNotExist(err) {
						m.status = fmt.Sprintf("Missing file: %s", sel.FilePath)
					} else {
						m.status = fmt.Sprintf("Stat failed: %v", err)
					}
					return m, nil
				}
				folder := filepath.Dir(sel.FilePath)
				if err := exec.Command("xdg-open", folder).Start(); err != nil {
					m.status = fmt.Sprintf("Open folder failed: %v", err)
					return m, nil
				}
				m.status = fmt.Sprintf("Opened %s", folder)
			}
			return m, nil
		case key.Matches(msg, m.keys.CopyFolder):
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.entries) {
				sel := m.entries[idx]
				if _, err := os.Stat(sel.FilePath); err != nil {
					if os.IsNotExist(err) {
						m.status = fmt.Sprintf("Missing file: %s", sel.FilePath)
					} else {
						m.status = fmt.Sprintf("Stat failed: %v", err)
					}
					return m, nil
				}
				copyCmd := exec.Command("wl-copy")
				copyCmd.Stdin = strings.NewReader(sel.FilePath)
				if err := copyCmd.Run(); err != nil {
					m.status = fmt.Sprintf("Copy path failed: %v", err)
					return m, nil
				}
				m.status = "Copied path to clipboard"
			}
			return m, nil
		case key.Matches(msg, m.keys.Delete):
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.entries) {
				sel := m.entries[idx]
				if err := m.store.DeleteScreenshot(sel.ID); err == nil {
					// Remove from slice
					m.entries = append(m.entries[:idx], m.entries[idx+1:]...)
					m.updateRows()
					// Adjust cursor
					if idx >= len(m.entries) {
						m.table.SetCursor(len(m.entries) - 1)
					}
					m.status = fmt.Sprintf("Deleted ID %d", sel.ID)
				} else {
					m.status = fmt.Sprintf("Error deleting: %v", err)
				}
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	base := m.table.View() + "\n" + m.renderFooter()
	if m.showHelp {
		helpView, w, h := m.helpModalView()
		return m.renderOverlay(base, helpView, w, h)
	}
	return base
}

func (m model) renderFooter() string {
	left := "? for help"
	right := fmt.Sprintf("%d items", len(m.entries))
	if m.status != "" {
		right = m.status + " • " + right
	}

	width := m.width
	if width == 0 {
		width = 80
	}
	space := width - lipgloss.Width(left) - lipgloss.Width(right)
	if space < 1 {
		space = 1
	}
	return left + strings.Repeat(" ", space) + right
}

func (m model) helpModalView() (string, int, int) {
	content := m.help.View(m.keys)
	box := lipgloss.NewStyle().
		Padding(1, 2).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63"))
	view := box.Render(content)
	return view, lipgloss.Width(view), lipgloss.Height(view)
}

func (m model) renderOverlay(base, overlay string, overlayW, overlayH int) string {
	termW, termH := m.width, m.height
	if termW <= 0 {
		termW = 80
	}
	if termH <= 0 {
		termH = 24
	}

	x := (termW - overlayW) / 2
	y := (termH - overlayH) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	dimBase := lipglossv2.NewStyle().Faint(true).Render(base)
	baseLayer := lipglossv2.NewLayer(dimBase).
		Width(termW).
		Height(termH)
	overlayLayer := lipglossv2.NewLayer(overlay).
		Width(overlayW).
		Height(overlayH).
		X(x).
		Y(y)

	return lipglossv2.NewCanvas(baseLayer, overlayLayer).Render()
}

func (m *model) applyLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	h := m.height - 2 // Footer space
	if h < 5 {
		h = 5
	}
	m.table.SetHeight(h)
	m.table.SetWidth(m.width)

	// Dynamic column width
	avail := m.width - 4 - 24 // approximate fixed widths for ID and Time
	if avail > 20 {
		appW := avail / 3
		titleW := avail - appW
		cols := m.table.Columns()
		cols[2].Width = appW
		cols[3].Width = titleW
		m.table.SetColumns(cols)
	}
}

func (m *model) applyStyles() {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	m.table.SetStyles(s)
}
