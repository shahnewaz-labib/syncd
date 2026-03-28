package ui

import "github.com/charmbracelet/lipgloss"

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("170")).
			Bold(true)

	NormalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Bold(true)

	ProgressBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205"))

	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)
)

func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return lipgloss.NewStyle().Render(
			lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(
				formatFloat(float64(bytes)/float64(GB)) + " GB",
			),
		)
	case bytes >= MB:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(
			formatFloat(float64(bytes)/float64(MB)) + " MB",
		)
	case bytes >= KB:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(
			formatFloat(float64(bytes)/float64(KB)) + " KB",
		)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(
			formatInt(bytes) + " B",
		)
	}
}

func formatFloat(f float64) string {
	if f == float64(int(f)) {
		return formatInt(int64(f))
	}
	return lipgloss.NewStyle().Render(
		func() string {
			s := ""
			for i := 0; i < 2; i++ {
				f *= 10
				s += string('0' + byte(int(f)%10))
			}
			return formatInt(int64(f/100)) + "." + s
		}(),
	)
}

func formatInt(n int64) string {
	if n < 0 {
		return "-" + formatInt(-n)
	}
	if n < 10 {
		return string('0' + byte(n))
	}
	return formatInt(n/10) + string('0'+byte(n%10))
}
