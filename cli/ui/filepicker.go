package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type FileEntry struct {
	Name  string
	Path  string
	IsDir bool
	Size  int64
}

type FilePickerModel struct {
	currentDir   string
	entries      []FileEntry
	cursor       int
	selectedFile string
	err          error
	quitting     bool
}

type FileSelectedMsg struct {
	Path string
	Size int64
}

func NewFilePickerModel() FilePickerModel {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/"
	}

	m := FilePickerModel{
		currentDir: home,
	}
	m.loadDir()
	return m
}

func (m *FilePickerModel) loadDir() {
	entries, err := os.ReadDir(m.currentDir)
	if err != nil {
		m.err = err
		return
	}

	m.entries = make([]FileEntry, 0, len(entries)+1)

	// Add parent directory entry if not at root
	if m.currentDir != "/" {
		m.entries = append(m.entries, FileEntry{
			Name:  "..",
			Path:  filepath.Dir(m.currentDir),
			IsDir: true,
		})
	}

	// Add directories first, then files
	var dirs, files []FileEntry
	for _, entry := range entries {
		// Skip hidden files
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fe := FileEntry{
			Name:  entry.Name(),
			Path:  filepath.Join(m.currentDir, entry.Name()),
			IsDir: entry.IsDir(),
			Size:  info.Size(),
		}

		if entry.IsDir() {
			dirs = append(dirs, fe)
		} else {
			files = append(files, fe)
		}
	}

	// Sort alphabetically
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	m.entries = append(m.entries, dirs...)
	m.entries = append(m.entries, files...)
	m.cursor = 0
	m.err = nil
}

func (m FilePickerModel) Init() tea.Cmd {
	return nil
}

func (m FilePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, fileKeys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, fileKeys.Down):
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
		case key.Matches(msg, fileKeys.Enter):
			if len(m.entries) > 0 {
				entry := m.entries[m.cursor]
				if entry.IsDir {
					m.currentDir = entry.Path
					m.loadDir()
				} else {
					m.selectedFile = entry.Path
					return m, func() tea.Msg {
						return FileSelectedMsg{Path: entry.Path, Size: entry.Size}
					}
				}
			}
		case key.Matches(msg, fileKeys.Back):
			if m.currentDir != "/" {
				m.currentDir = filepath.Dir(m.currentDir)
				m.loadDir()
			}
		case key.Matches(msg, fileKeys.Quit):
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m FilePickerModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	s.WriteString(TitleStyle.Render("Select a file to send"))
	s.WriteString("\n")
	s.WriteString(DimStyle.Render("  " + m.currentDir))
	s.WriteString("\n\n")

	if m.err != nil {
		s.WriteString(ErrorStyle.Render("  Error: " + m.err.Error()))
		s.WriteString("\n")
	}

	if len(m.entries) == 0 {
		s.WriteString(DimStyle.Render("  (empty directory)\n"))
	} else {
		// Show max 15 entries with scrolling
		start := 0
		end := len(m.entries)
		maxVisible := 15

		if len(m.entries) > maxVisible {
			start = m.cursor - maxVisible/2
			if start < 0 {
				start = 0
			}
			end = start + maxVisible
			if end > len(m.entries) {
				end = len(m.entries)
				start = end - maxVisible
			}
		}

		for i := start; i < end; i++ {
			entry := m.entries[i]
			cursor := "  "
			style := NormalStyle
			if m.cursor == i {
				cursor = "> "
				style = SelectedStyle
			}

			icon := " "
			if entry.IsDir {
				icon = "/"
			}

			line := fmt.Sprintf("%s%s%s", cursor, entry.Name, icon)
			if !entry.IsDir && entry.Size > 0 {
				line += DimStyle.Render(fmt.Sprintf(" (%s)", FormatSize(entry.Size)))
			}
			s.WriteString(style.Render(line))
			s.WriteString("\n")
		}
	}

	s.WriteString(HelpStyle.Render("\n  up/down: navigate | enter: select/open | backspace: go up | q: quit"))

	return s.String()
}

func (m FilePickerModel) SelectedFile() string {
	return m.selectedFile
}

type fileKeyMap struct {
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
	Back  key.Binding
	Quit  key.Binding
}

var fileKeys = fileKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
	),
	Back: key.NewBinding(
		key.WithKeys("backspace", "left", "h"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c", "esc"),
	),
}
