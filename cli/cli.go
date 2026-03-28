package cli

import (
	"fmt"
	"os"

	"syncd/announcement"
	"syncd/cli/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func init() {
	home, _ := os.UserHomeDir()
	ui.SetHomeDir(home)
}

func RunDashboard() error {
	p := tea.NewProgram(
		ui.NewDashboardModel(),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}

func ShowStatus() {
	devices := announcement.GetOnlineDevices()

	fmt.Println(ui.TitleStyle.Render("Online Devices"))
	fmt.Println()

	if len(devices) == 0 {
		fmt.Println(ui.DimStyle.Render("  No devices online"))
		return
	}

	for _, device := range devices {
		fmt.Printf("  %s @ %s\n",
			ui.SelectedStyle.Render(device.Username),
			ui.NormalStyle.Render(device.IP),
		)
	}
}
