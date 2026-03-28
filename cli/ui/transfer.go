package ui

import (
	"fmt"
	"strings"

	"syncd/events"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Transfer request prompt (receiver side)
type TransferPromptModel struct {
	request    events.TransferRequestPayload
	accepted   *bool
	quitting   bool
}

type TransferDecisionMsg struct {
	TransferID string
	Accepted   bool
}

func NewTransferPromptModel(request events.TransferRequestPayload) TransferPromptModel {
	return TransferPromptModel{
		request: request,
	}
}

func (m TransferPromptModel) Init() tea.Cmd {
	return nil
}

func (m TransferPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y", "enter":
			accepted := true
			m.accepted = &accepted
			return m, func() tea.Msg {
				return TransferDecisionMsg{TransferID: m.request.TransferID, Accepted: true}
			}
		case "n", "N", "esc":
			accepted := false
			m.accepted = &accepted
			return m, func() tea.Msg {
				return TransferDecisionMsg{TransferID: m.request.TransferID, Accepted: false}
			}
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m TransferPromptModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	s.WriteString(TitleStyle.Render("Incoming Transfer Request"))
	s.WriteString("\n\n")

	box := BoxStyle.Render(fmt.Sprintf(
		"From: %s (%s)\nFile: %s\nSize: %s",
		m.request.SenderName,
		m.request.SenderIP,
		m.request.FileName,
		FormatSize(m.request.FileSize),
	))
	s.WriteString(box)
	s.WriteString("\n\n")

	s.WriteString(NormalStyle.Render("Accept this transfer? "))
	s.WriteString(SelectedStyle.Render("[Y/n]"))
	s.WriteString("\n")

	return s.String()
}

// Save location input (receiver side)
type SaveLocationModel struct {
	textInput  textinput.Model
	transferID string
	fileName   string
	savePath   string
	confirmed  bool
	quitting   bool
}

type SaveLocationConfirmedMsg struct {
	TransferID string
	SavePath   string
}

func NewSaveLocationModel(transferID, fileName string) SaveLocationModel {
	ti := textinput.New()
	home := getHomeDir()
	defaultPath := fmt.Sprintf("%s/Downloads/%s", home, fileName)
	ti.SetValue(defaultPath)
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	return SaveLocationModel{
		textInput:  ti,
		transferID: transferID,
		fileName:   fileName,
	}
}

func getHomeDir() string {
	if home := getEnvHome(); home != "" {
		return home
	}
	return "/tmp"
}

func getEnvHome() string {
	// Will be set via os.UserHomeDir() in cli.go
	return homeDir
}

var homeDir string

func SetHomeDir(dir string) {
	homeDir = dir
}

func (m SaveLocationModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m SaveLocationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.savePath = m.textInput.Value()
			m.confirmed = true
			return m, func() tea.Msg {
				return SaveLocationConfirmedMsg{
					TransferID: m.transferID,
					SavePath:   m.savePath,
				}
			}
		case "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m SaveLocationModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	s.WriteString(TitleStyle.Render("Save Location"))
	s.WriteString("\n\n")
	s.WriteString(NormalStyle.Render("Where to save the file?\n\n"))
	s.WriteString(m.textInput.View())
	s.WriteString("\n")
	s.WriteString(HelpStyle.Render("\n  enter: confirm | esc: cancel"))

	return s.String()
}

// Transfer progress (both sides)
type TransferProgressModel struct {
	transferID       string
	fileName         string
	totalBytes       int64
	bytesTransferred int64
	progress         progress.Model
	status           string
	completed        bool
	failed           bool
	errorMsg         string
}

func NewTransferProgressModel(transferID, fileName string, totalBytes int64, isSending bool) TransferProgressModel {
	p := progress.New(progress.WithDefaultGradient())

	status := "Receiving..."
	if isSending {
		status = "Sending..."
	}

	return TransferProgressModel{
		transferID: transferID,
		fileName:   fileName,
		totalBytes: totalBytes,
		progress:   p,
		status:     status,
	}
}

func (m TransferProgressModel) Init() tea.Cmd {
	return nil
}

func (m TransferProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))) {
			return m, tea.Quit
		}
		if m.completed || m.failed {
			if key.Matches(msg, key.NewBinding(key.WithKeys("enter"))) {
				return m, tea.Quit
			}
		}

	case events.TransferProgressPayload:
		if msg.TransferID == m.transferID {
			m.bytesTransferred = msg.BytesTransferred
		}

	case TransferCompletedMsg:
		if msg.TransferID == m.transferID {
			m.completed = true
			m.bytesTransferred = m.totalBytes
			m.status = "Completed!"
		}

	case TransferFailedMsg:
		if msg.TransferID == m.transferID {
			m.failed = true
			m.errorMsg = msg.Error
			m.status = "Failed"
		}

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd
	}

	return m, nil
}

func (m TransferProgressModel) View() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("File Transfer"))
	s.WriteString("\n\n")

	s.WriteString(NormalStyle.Render(fmt.Sprintf("File: %s\n", m.fileName)))
	s.WriteString(NormalStyle.Render(fmt.Sprintf("Size: %s\n\n", FormatSize(m.totalBytes))))

	percent := 0.0
	if m.totalBytes > 0 {
		percent = float64(m.bytesTransferred) / float64(m.totalBytes)
	}

	s.WriteString(m.progress.ViewAs(percent))
	s.WriteString("\n\n")

	s.WriteString(fmt.Sprintf("%s / %s", FormatSize(m.bytesTransferred), FormatSize(m.totalBytes)))
	s.WriteString("\n\n")

	if m.completed {
		s.WriteString(SuccessStyle.Render(m.status))
		s.WriteString(HelpStyle.Render("\n\n  Press enter to continue"))
	} else if m.failed {
		s.WriteString(ErrorStyle.Render(m.status + ": " + m.errorMsg))
		s.WriteString(HelpStyle.Render("\n\n  Press enter to continue"))
	} else {
		s.WriteString(DimStyle.Render(m.status))
	}

	return s.String()
}

type TransferCompletedMsg struct {
	TransferID string
}

type TransferFailedMsg struct {
	TransferID string
	Error      string
}
