package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	}
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
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "enter":
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.entries) {
				sel := m.entries[idx]
				_ = exec.Command("xdg-open", sel.FilePath).Start()
				m.status = fmt.Sprintf("Opened %s", sel.FilePath)
			}
			return m, nil
		case "c":
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
		case "d":
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
	return base
}

func (m model) renderFooter() string {
	left := "↑/↓ to navigate • enter=open • c=copy • d=delete • q=quit"
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
