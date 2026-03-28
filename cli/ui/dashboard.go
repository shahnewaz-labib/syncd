package ui

import (
	"fmt"
	"strings"
	"time"

	"syncd/announcement"
	"syncd/events"
	"syncd/transfer"
	"syncd/utils"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Pane int

const (
	PaneDevices Pane = iota
	PaneTransfers
)

type TransferItem struct {
	ID          string
	FileName    string
	PeerName    string
	PeerIP      string
	TotalBytes  int64
	Transferred int64
	IsSending   bool
	Status      string
	StartTime   time.Time
}

type LogEntry struct {
	Time    time.Time
	Message string
	Type    string // "info", "success", "error", "warning"
}

type DashboardModel struct {
	width  int
	height int

	activePane    Pane
	deviceCursor  int
	devices       []*announcement.Device
	myInfo        *DeviceInfo
	transfers     []TransferItem
	transferCursor int
	logs          []LogEntry

	// Sub-views for modals
	showFilePicker   bool
	filePicker       FilePickerModel
	showTransferPrompt bool
	transferPrompt   TransferPromptModel
	showSaveLocation bool
	saveLocation     SaveLocationModel

	// Progress bars for transfers
	progressBars map[string]progress.Model

	selectedDevice *announcement.Device
	pendingRequest *events.TransferRequestPayload

	statusMsg string
	quitting  bool
}

type DeviceInfo struct {
	Username string
	DeviceID string
	IP       string
}

type tickMsg time.Time

func NewDashboardModel() DashboardModel {
	deviceInfo, _ := utils.GetDeviceInfo()
	myInfo := &DeviceInfo{
		Username: utils.GetUsername(),
		DeviceID: deviceInfo.UniqueDeviceID[:16] + "...",
		IP:       utils.GetLocalIP(),
	}

	return DashboardModel{
		width:        80,
		height:       24,
		activePane:   PaneDevices,
		devices:      announcement.GetOnlineDevices(),
		myInfo:       myInfo,
		transfers:    []TransferItem{},
		logs:         []LogEntry{{Time: time.Now(), Message: "syncd started", Type: "info"}},
		progressBars: make(map[string]progress.Model),
		statusMsg:    "Ready",
	}
}

func (m *DashboardModel) addLog(message, logType string) {
	entry := LogEntry{
		Time:    time.Now(),
		Message: message,
		Type:    logType,
	}
	m.logs = append(m.logs, entry)
	// Keep only last 50 logs
	if len(m.logs) > 50 {
		m.logs = m.logs[len(m.logs)-50:]
	}
}

func (m DashboardModel) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		listenEvents(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func listenEvents() tea.Cmd {
	return func() tea.Msg {
		select {
		case event := <-events.GetEventChannel():
			return event
		case <-time.After(100 * time.Millisecond):
			return nil
		}
	}
}

func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle modals first
	if m.showFilePicker {
		return m.updateFilePicker(msg)
	}
	if m.showTransferPrompt {
		return m.updateTransferPrompt(msg)
	}
	if m.showSaveLocation {
		return m.updateSaveLocation(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tickMsg:
		oldCount := len(m.devices)
		m.devices = announcement.GetOnlineDevices()
		newCount := len(m.devices)
		if newCount > oldCount {
			m.addLog(fmt.Sprintf("Device discovered (%d online)", newCount), "info")
		} else if newCount < oldCount {
			m.addLog(fmt.Sprintf("Device went offline (%d online)", newCount), "warning")
		}
		return m, tea.Batch(tickCmd(), listenEvents())

	case events.Event:
		return m.handleEvent(msg)

	case nil:
		return m, listenEvents()
	}

	return m, nil
}

func (m DashboardModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, dashKeys.Quit):
		m.quitting = true
		return m, tea.Quit

	case key.Matches(msg, dashKeys.Tab):
		if m.activePane == PaneDevices {
			m.activePane = PaneTransfers
		} else {
			m.activePane = PaneDevices
		}

	case key.Matches(msg, dashKeys.Up):
		if m.activePane == PaneDevices && m.deviceCursor > 0 {
			m.deviceCursor--
		} else if m.activePane == PaneTransfers && m.transferCursor > 0 {
			m.transferCursor--
		}

	case key.Matches(msg, dashKeys.Down):
		if m.activePane == PaneDevices && m.deviceCursor < len(m.devices)-1 {
			m.deviceCursor++
		} else if m.activePane == PaneTransfers && m.transferCursor < len(m.transfers)-1 {
			m.transferCursor++
		}

	case key.Matches(msg, dashKeys.Enter):
		if m.activePane == PaneDevices && len(m.devices) > 0 {
			m.selectedDevice = m.devices[m.deviceCursor]
			m.showFilePicker = true
			m.filePicker = NewFilePickerModel()
			return m, m.filePicker.Init()
		}

	case key.Matches(msg, dashKeys.Refresh):
		m.devices = announcement.GetOnlineDevices()
		m.statusMsg = "Refreshed"
	}

	return m, nil
}

func (m DashboardModel) handleEvent(event events.Event) (tea.Model, tea.Cmd) {
	switch event.Type {
	case events.TransferRequestReceived:
		payload := event.Payload.(events.TransferRequestPayload)
		m.pendingRequest = &payload
		m.transferPrompt = NewTransferPromptModel(payload)
		m.showTransferPrompt = true
		m.statusMsg = "Incoming transfer request"
		m.addLog(fmt.Sprintf("Transfer request from %s (%s)", payload.SenderName, payload.FileName), "info")
		return m, m.transferPrompt.Init()

	case events.TransferProgress:
		payload := event.Payload.(events.TransferProgressPayload)
		for i := range m.transfers {
			if m.transfers[i].ID == payload.TransferID {
				m.transfers[i].Transferred = payload.BytesTransferred
				m.transfers[i].Status = "transferring"
				break
			}
		}

	case events.TransferCompleted:
		transferID := event.Payload.(string)
		var fileName string
		for i := range m.transfers {
			if m.transfers[i].ID == transferID {
				m.transfers[i].Status = "completed"
				m.transfers[i].Transferred = m.transfers[i].TotalBytes
				fileName = m.transfers[i].FileName
				break
			}
		}
		m.statusMsg = "Transfer completed"
		m.addLog(fmt.Sprintf("Transfer completed: %s", fileName), "success")

	case events.TransferFailed:
		payload := event.Payload.(map[string]any)
		transferID := payload["transfer_id"].(string)
		errMsg := ""
		if e, ok := payload["error"].(string); ok {
			errMsg = e
		}
		for i := range m.transfers {
			if m.transfers[i].ID == transferID {
				m.transfers[i].Status = "failed"
				break
			}
		}
		m.statusMsg = "Transfer failed"
		m.addLog(fmt.Sprintf("Transfer failed: %s", errMsg), "error")

	case events.TransferResponseReceived:
		payload := event.Payload.(events.TransferResponsePayload)
		if payload.Accepted {
			m.statusMsg = "Transfer accepted, sending..."
			m.addLog("Transfer accepted by peer", "success")
		} else {
			m.statusMsg = "Transfer rejected"
			m.addLog("Transfer rejected by peer", "warning")
			for i := range m.transfers {
				if m.transfers[i].ID == payload.TransferID {
					m.transfers[i].Status = "rejected"
					break
				}
			}
		}
	}

	return m, listenEvents()
}

func (m DashboardModel) updateFilePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.showFilePicker = false
			return m, nil
		}

	case FileSelectedMsg:
		m.showFilePicker = false
		m.addLog(fmt.Sprintf("Sending %s to %s", msg.Path, m.selectedDevice.Username), "info")
		// Start transfer
		return m, m.startTransfer(msg.Path, msg.Size)
	}

	var cmd tea.Cmd
	updated, cmd := m.filePicker.Update(msg)
	m.filePicker = updated.(FilePickerModel)
	return m, cmd
}

func (m DashboardModel) updateTransferPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TransferDecisionMsg:
		m.showTransferPrompt = false
		if msg.Accepted {
			m.saveLocation = NewSaveLocationModel(msg.TransferID, m.pendingRequest.FileName)
			m.showSaveLocation = true
			return m, m.saveLocation.Init()
		} else {
			m.rejectTransfer(msg.TransferID)
			m.statusMsg = "Transfer rejected"
		}
		return m, nil
	}

	var cmd tea.Cmd
	updated, cmd := m.transferPrompt.Update(msg)
	m.transferPrompt = updated.(TransferPromptModel)
	return m, cmd
}

func (m DashboardModel) updateSaveLocation(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.showSaveLocation = false
			m.rejectTransfer(m.pendingRequest.TransferID)
			return m, nil
		}

	case SaveLocationConfirmedMsg:
		m.showSaveLocation = false
		m.acceptTransfer(msg.TransferID, msg.SavePath)

		// Add to transfers list
		m.transfers = append(m.transfers, TransferItem{
			ID:         msg.TransferID,
			FileName:   m.pendingRequest.FileName,
			PeerName:   m.pendingRequest.SenderName,
			PeerIP:     m.pendingRequest.SenderIP,
			TotalBytes: m.pendingRequest.FileSize,
			IsSending:  false,
			Status:     "receiving",
			StartTime:  time.Now(),
		})
		m.statusMsg = "Receiving file..."
		return m, listenEvents()
	}

	var cmd tea.Cmd
	updated, cmd := m.saveLocation.Update(msg)
	m.saveLocation = updated.(SaveLocationModel)
	return m, cmd
}

func (m *DashboardModel) startTransfer(filePath string, fileSize int64) tea.Cmd {
	return func() tea.Msg {
		deviceInfo, err := utils.GetDeviceInfo()
		if err != nil {
			return events.Event{Type: events.TransferFailed, Payload: map[string]any{"error": err.Error()}}
		}

		sender := transfer.GetSender()
		t, err := sender.InitiateTransfer(
			m.selectedDevice.IP,
			filePath,
			deviceInfo.UniqueDeviceID,
			utils.GetUsername(),
			utils.GetLocalIP(),
		)
		if err != nil {
			return events.Event{Type: events.TransferFailed, Payload: map[string]any{"error": err.Error()}}
		}

		sender.SetFilePath(t.ID, filePath)

		// Add to transfers
		m.transfers = append(m.transfers, TransferItem{
			ID:         t.ID,
			FileName:   t.Request.FileName,
			PeerName:   m.selectedDevice.Username,
			PeerIP:     m.selectedDevice.IP,
			TotalBytes: fileSize,
			IsSending:  true,
			Status:     "pending",
			StartTime:  time.Now(),
		})
		m.statusMsg = "Transfer request sent"

		return nil
	}
}

func (m *DashboardModel) acceptTransfer(transferID, savePath string) {
	receiver := transfer.GetReceiver()
	receiver.AcceptTransfer(transferID, savePath)

	response := transfer.TransferResponse{
		TransferID:   transferID,
		Accepted:     true,
		SavePath:     savePath,
		ReceiverIP:   utils.GetLocalIP(),
		ReceiverPort: 10001,
	}

	if m.pendingRequest != nil {
		go utils.PostJSON(m.pendingRequest.SenderIP, "/transfer/response", response)
	}
}

func (m *DashboardModel) rejectTransfer(transferID string) {
	receiver := transfer.GetReceiver()
	receiver.RejectTransfer(transferID)

	response := transfer.TransferResponse{
		TransferID: transferID,
		Accepted:   false,
	}

	if m.pendingRequest != nil {
		go utils.PostJSON(m.pendingRequest.SenderIP, "/transfer/response", response)
	}
}

func (m DashboardModel) View() string {
	if m.quitting {
		return ""
	}

	// Handle modals
	if m.showFilePicker {
		return m.filePicker.View()
	}
	if m.showTransferPrompt {
		return m.transferPrompt.View()
	}
	if m.showSaveLocation {
		return m.saveLocation.View()
	}

	// Build dashboard
	return m.renderDashboard()
}

func (m DashboardModel) renderDashboard() string {
	// Calculate pane dimensions
	leftWidth := m.width / 3
	rightWidth := m.width - leftWidth - 3

	// Header
	header := m.renderHeader()

	// Left column: My Info + Devices
	myInfoPane := m.renderMyInfo(leftWidth)
	devicesPane := m.renderDevices(leftWidth)
	leftColumn := lipgloss.JoinVertical(lipgloss.Left, myInfoPane, devicesPane)

	// Right column: Transfers
	transfersPane := m.renderTransfers(rightWidth)

	// Join columns
	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, "  ", transfersPane)

	// Logs pane (full width)
	logsPane := m.renderLogs(m.width - 2)

	// Footer
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, "", mainContent, "", logsPane, "", footer)
}

func (m DashboardModel) renderHeader() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("syncd")

	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render(" - P2P File Transfer")

	return title + subtitle
}

func (m DashboardModel) renderMyInfo(width int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1).
		Width(width)

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170")).Render("My Device")

	content := fmt.Sprintf("%s\n\n  User: %s\n  ID:   %s\n  IP:   %s",
		title,
		lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render(m.myInfo.Username),
		lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(m.myInfo.DeviceID),
		lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(m.myInfo.IP),
	)

	return style.Render(content)
}

func (m DashboardModel) renderDevices(width int) string {
	borderColor := lipgloss.Color("62")
	if m.activePane == PaneDevices {
		borderColor = lipgloss.Color("205")
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(width).
		Height(10)

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170")).Render("Online Devices")
	if m.activePane == PaneDevices {
		title += lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(" (active)")
	}

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "")

	if len(m.devices) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  No devices online"))
	} else {
		for i, device := range m.devices {
			cursor := "  "
			nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
			if i == m.deviceCursor && m.activePane == PaneDevices {
				cursor = "> "
				nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
			}

			line := fmt.Sprintf("%s%s", cursor, nameStyle.Render(device.Username))
			ipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
			line += ipStyle.Render(fmt.Sprintf(" @ %s", device.IP))
			lines = append(lines, line)
		}
	}

	return style.Render(strings.Join(lines, "\n"))
}

func (m DashboardModel) renderTransfers(width int) string {
	borderColor := lipgloss.Color("62")
	if m.activePane == PaneTransfers {
		borderColor = lipgloss.Color("205")
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(width).
		Height(15)

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170")).Render("Transfers")
	if m.activePane == PaneTransfers {
		title += lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(" (active)")
	}

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "")

	if len(m.transfers) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  No active transfers"))
		lines = append(lines, "")
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  Select a device and press Enter to send a file"))
	} else {
		for i, t := range m.transfers {
			cursor := "  "
			if i == m.transferCursor && m.activePane == PaneTransfers {
				cursor = "> "
			}

			direction := lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render("RECV")
			if t.IsSending {
				direction = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("SEND")
			}

			statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
			switch t.Status {
			case "completed":
				statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
			case "failed", "rejected":
				statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
			case "transferring", "receiving":
				statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
			}

			// Progress
			percent := 0.0
			if t.TotalBytes > 0 {
				percent = float64(t.Transferred) / float64(t.TotalBytes) * 100
			}

			line := fmt.Sprintf("%s[%s] %s", cursor, direction, t.FileName)
			lines = append(lines, line)

			progressLine := fmt.Sprintf("      %s - %.0f%% (%s / %s)",
				statusStyle.Render(t.Status),
				percent,
				FormatSize(t.Transferred),
				FormatSize(t.TotalBytes),
			)
			lines = append(lines, progressLine)
			lines = append(lines, "")
		}
	}

	return style.Render(strings.Join(lines, "\n"))
}

func (m DashboardModel) renderLogs(width int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1).
		Width(width).
		Height(6)

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170")).Render("Activity Log")

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "")

	if len(m.logs) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  No activity yet"))
	} else {
		// Show last 3 logs
		start := len(m.logs) - 3
		if start < 0 {
			start = 0
		}
		for _, entry := range m.logs[start:] {
			timeStr := entry.Time.Format("15:04:05")
			timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

			msgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
			switch entry.Type {
			case "success":
				msgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
			case "error":
				msgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
			case "warning":
				msgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
			}

			line := fmt.Sprintf("  %s %s", timeStyle.Render(timeStr), msgStyle.Render(entry.Message))
			lines = append(lines, line)
		}
	}

	return style.Render(strings.Join(lines, "\n"))
}

func (m DashboardModel) renderFooter() string {
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	help := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		"Tab: switch pane | Up/Down: navigate | Enter: send file | r: refresh | q: quit",
	)

	status := statusStyle.Render(fmt.Sprintf("Status: %s", m.statusMsg))

	return lipgloss.JoinVertical(lipgloss.Left, help, status)
}

type dashKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Tab     key.Binding
	Enter   key.Binding
	Refresh key.Binding
	Quit    key.Binding
}

var dashKeys = dashKeyMap{
	Up:      key.NewBinding(key.WithKeys("up", "k")),
	Down:    key.NewBinding(key.WithKeys("down", "j")),
	Tab:     key.NewBinding(key.WithKeys("tab")),
	Enter:   key.NewBinding(key.WithKeys("enter")),
	Refresh: key.NewBinding(key.WithKeys("r")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c")),
}
