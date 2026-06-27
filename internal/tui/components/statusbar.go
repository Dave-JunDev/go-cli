package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/dave/kube-tui/internal/tui/theme"
)

type StatusBar struct {
	cluster   string
	namespace string
	resource  string
	mode      string
	items     int
	error     string
}

func NewStatusBar() *StatusBar {
	return &StatusBar{}
}

func (s *StatusBar) SetCluster(c string)  { s.cluster = c }
func (s *StatusBar) SetNamespace(n string) { s.namespace = n }
func (s *StatusBar) SetResource(r string)  { s.resource = r }
func (s *StatusBar) SetMode(m string)      { s.mode = m }
func (s *StatusBar) SetItems(n int)        { s.items = n }
func (s *StatusBar) SetError(e string)     { s.error = e }
func (s *StatusBar) ClearError()           { s.error = "" }

func (s *StatusBar) View(width int) string {
	clusterInfo := lipgloss.NewStyle().
		Foreground(theme.HotPink).
		Bold(true).
		Render("☸ " + theme.Truncate(s.cluster, 30))

	nsInfo := lipgloss.NewStyle().
		Foreground(theme.SkyBlue).
		Render("■ " + s.namespace)

	modeInfo := lipgloss.NewStyle().
		Foreground(theme.Lime).
		Render("◆ " + s.mode)

	countInfo := lipgloss.NewStyle().
		Foreground(theme.Orange).
		Render(fmt.Sprintf("◆ %d items", s.items))

	left := lipgloss.JoinHorizontal(lipgloss.Center,
		clusterInfo, "  ", nsInfo, "  ", modeInfo, "  ", countInfo,
	)

	if s.error != "" {
		errInfo := theme.ErrorStyle.Render("✗ " + s.error)
		return lipgloss.JoinHorizontal(lipgloss.Center,
			left,
			"  ",
			errInfo,
		)
	}

	helpInfo := lipgloss.NewStyle().
		Foreground(theme.MutedText).
		Italic(true).
		Render("↑↓ nav • / filter • q back • ? help")

	fillWidth := width - lipgloss.Width(left) - lipgloss.Width(helpInfo) - 4
	if fillWidth < 0 {
		fillWidth = 1
	}
	fill := lipgloss.NewStyle().Width(fillWidth).Render("")

	return lipgloss.NewStyle().
		Background(lipgloss.Color("#0D0D1A")).
		Width(width).
		Padding(0, 1).
		Render(lipgloss.JoinHorizontal(lipgloss.Center, left, fill, helpInfo))
}
