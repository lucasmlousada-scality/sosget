package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasmlousada-scality/sosget/internal/sftp"
)

var (
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	headerStyle   = lipgloss.NewStyle().Bold(true).Underline(true)
)

type model struct {
	files    []sftp.FileEntry
	cursor   int
	selected map[int]bool
	done     bool
	quit     bool
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.files)-1 {
				m.cursor++
			}
		case " ":
			m.selected[m.cursor] = !m.selected[m.cursor]
		case "a":
			// Select / deselect all
			if len(m.selected) == len(m.files) {
				m.selected = map[int]bool{}
			} else {
				for i := range m.files {
					m.selected[i] = true
				}
			}
		case "enter":
			m.done = true
			return m, tea.Quit
		case "q", "ctrl+c", "esc":
			m.quit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Select files to download"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓ navigate  space select  a select-all  enter download  q quit"))
	b.WriteString("\n\n")

	for i, f := range m.files {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("▶ ")
		}

		check := "[ ]"
		name := f.Name
		if m.selected[i] {
			check = selectedStyle.Render("[✓]")
			name = selectedStyle.Render(name)
		}

		size := formatSize(f.Size)
		age := formatAge(f.ModTime)
		meta := dimStyle.Render(fmt.Sprintf("  %8s  %s", size, age))

		b.WriteString(fmt.Sprintf("%s%s %s%s\n", cursor, check, name, meta))
	}

	b.WriteString(dimStyle.Render(fmt.Sprintf("\n%d file(s), %d selected", len(m.files), len(m.selected))))
	return b.String()
}

// FilePicker renders an interactive file picker and returns the selected files.
func FilePicker(files []sftp.FileEntry) ([]sftp.FileEntry, error) {
	m := model{
		files:    files,
		selected: make(map[int]bool),
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	fm := finalModel.(model)
	if fm.quit {
		return nil, nil
	}

	var result []sftp.FileEntry
	for i, f := range files {
		if fm.selected[i] {
			result = append(result, f)
		}
	}
	return result, nil
}

func formatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return t.Format("2006-01-02 15:04")
	}
}
