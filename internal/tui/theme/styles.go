package theme

import "github.com/charmbracelet/lipgloss"

var (
	HotPink    = lipgloss.Color("#FF1493")
	Purple     = lipgloss.Color("#BF00FF")
	Lime       = lipgloss.Color("#39FF14")
	Gold       = lipgloss.Color("#FFD700")
	Orange     = lipgloss.Color("#FF6B35")
	BrightRed  = lipgloss.Color("#FF0040")
	SkyBlue    = lipgloss.Color("#00BFFF")
	Coral      = lipgloss.Color("#FF6F61")
	ElectricBlue = lipgloss.Color("#00FFFF")
	DarkBg     = lipgloss.Color("#1A1A2E")
	DarkCard   = lipgloss.Color("#16213E")
	MutedText  = lipgloss.Color("#8A8A9A")
)

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(HotPink).
			Background(lipgloss.Color("#2D0A3E")).
			Padding(0, 2).
			Margin(0, 0, 1, 0)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ElectricBlue).
			Bold(true).
			Padding(0, 1)

	SelectedRowStyle = lipgloss.NewStyle().
				Background(Purple).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true)

	NormalRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E0E0E0"))

	FilterStyle = lipgloss.NewStyle().
			Foreground(Lime).
			Bold(true)

	StatusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#0D0D1A")).
			Foreground(Gold).
			Padding(0, 1).
			Width(80)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(BrightRed).
			Bold(true).
			Background(lipgloss.Color("#2A0000")).
			Padding(0, 1)

	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666680")).
			Italic(true)

	ClusterCardStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(Purple).
				Padding(0, 2).
				Margin(0, 1, 1, 1).
				Width(50)

	ClusterCardSelected = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(Gold).
				Background(lipgloss.Color("#2A1040")).
				Padding(0, 2).
				Margin(0, 1, 1, 1).
				Width(50)

	ResourceCountStyle = lipgloss.NewStyle().
				Foreground(Orange).
				Bold(true)

	NamespaceStyle = lipgloss.NewStyle().
			Foreground(SkyBlue).
			Bold(true)

	HeaderStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#0D0D1A")).
			Foreground(Gold).
			Bold(true).
			Padding(0, 1).
			Margin(0, 0, 1, 0)

	SpinnerStyle = lipgloss.NewStyle().
			Foreground(HotPink)

	StatusDotReady = lipgloss.NewStyle().
			Foreground(Lime).
			SetString("●")

	StatusDotError = lipgloss.NewStyle().
			Foreground(BrightRed).
			SetString("●")

	StatusDotWarn = lipgloss.NewStyle().
			Foreground(Orange).
			SetString("●")
)

func ColoredStatus(status string) string {
	s := lipgloss.NewStyle()
	switch status {
	case "Running", "Ready", "Active", "Succeeded":
		s = s.Foreground(Lime).Bold(true)
	case "Pending", "ContainerCreating", "PodInitializing":
		s = s.Foreground(Gold).Bold(true)
	case "Failed", "Error", "CrashLoopBackOff", "ImagePullBackOff", "NotReady":
		s = s.Foreground(BrightRed).Bold(true)
	case "Terminating":
		s = s.Foreground(Orange)
	default:
		s = s.Foreground(lipgloss.Color("#AAAAAA"))
	}
	return s.Render(status)
}

func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}
