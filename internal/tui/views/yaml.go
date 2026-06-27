package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dave/kube-tui/internal/model"
	"github.com/dave/kube-tui/internal/tui/theme"
	"github.com/dave/kube-tui/internal/tui/components"
)

type YamlModel struct {
	yaml      string
	resource  model.K8sResource
	viewport  viewport.Model
	width     int
	height    int
	search    *components.Filter
	searches  []int
	searchIdx int
}

func NewYamlModel() *YamlModel {
	vp := viewport.New(80, 20)
	return &YamlModel{
		viewport: vp,
		search:   components.NewFilter("Search YAML...", nil),
	}
}

func (m *YamlModel) SetYAML(yaml string, r model.K8sResource) {
	m.yaml = yaml
	m.resource = r
}

func (m *YamlModel) Init() tea.Cmd {
	return nil
}

func (m *YamlModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.updateSize()
}

func (m *YamlModel) updateSize() {
	m.viewport.Width = theme.ViewportWidth(m.width)
	m.viewport.Height = theme.ViewportHeight(m.height, 8)
	m.search.SetWidth(theme.ViewportWidth(m.width) - 4)
}

func (m *YamlModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSize()
		return m, nil

	case tea.KeyMsg:
		if m.search.Active() {
			return m.handleSearchKey(msg)
		}
		return m.handleNavigationKey(msg)
	}

	return m, nil
}

func (m *YamlModel) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.search.Blur()
		m.search.SetValue("")
		m.searches = nil
		m.searchIdx = 0
		return m, nil

	case "enter":
		m.search.Blur()
		if len(m.searches) > 0 {
			m.scrollToMatch(m.searchIdx)
		}
		return m, nil
	}

	cmd := m.search.Update(msg)
	m.runSearch()
	return m, cmd
}

func (m *YamlModel) handleNavigationKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "/":
		return m, m.search.Focus()

	case "n":
		if len(m.searches) > 0 {
			m.searchIdx = (m.searchIdx + 1) % len(m.searches)
			m.scrollToMatch(m.searchIdx)
		}

	case "N":
		if len(m.searches) > 0 {
			m.searchIdx = (m.searchIdx - 1 + len(m.searches)) % len(m.searches)
			m.scrollToMatch(m.searchIdx)
		}

	case "backspace", "esc", "q":
		return m, popViewCmd()

	case "ctrl+c":
		return m, tea.Quit

	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *YamlModel) runSearch() {
	query := strings.ToLower(m.search.Value())
	m.searches = nil
	m.searchIdx = 0

	if query == "" {
		return
	}

	lines := strings.Split(m.yaml, "\n")
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			m.searches = append(m.searches, i)
		}
	}
}

func (m *YamlModel) scrollToMatch(idx int) {
	if idx < 0 || idx >= len(m.searches) {
		return
	}
	lineNum := m.searches[idx]
	m.viewport.SetYOffset(lineNum - m.viewport.Height/2)
	if m.viewport.YOffset < 0 {
		m.viewport.YOffset = 0
	}
}

func (m *YamlModel) View() string {
	m.updateSize()
	m.search.SetWidth(theme.ViewportWidth(m.width) - 4)

	cw := theme.ContentWidth(m.width)
	title := theme.TitleStyle.Copy().Width(cw).Render(fmt.Sprintf("☰  YAML — %s/%s", m.resource.Type, m.resource.Name))

	lines := strings.Split(m.yaml, "\n")
	query := strings.ToLower(m.search.Value())
	hasSearch := query != "" && len(m.searches) > 0

	vpW := theme.ViewportWidth(m.width)
	styled := make([]string, len(lines))
	for i, line := range lines {
		display := line
		if len(display) > vpW-2 {
			display = display[:vpW-5] + "…"
		}

		isMatch := hasSearch && m.searchIdx < len(m.searches) && m.searches[m.searchIdx] == i
		isSearchResult := false
		for _, s := range m.searches {
			if s == i {
				isSearchResult = true
				break
			}
		}

		if isMatch {
			styled[i] = lipgloss.NewStyle().
				Foreground(theme.Gold).
				Background(theme.Purple).
				Bold(true).
				Render(display)
		} else if isSearchResult {
			styled[i] = lipgloss.NewStyle().
				Foreground(theme.Orange).
				Render(display)
		} else {
			styled[i] = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E0E0E0")).
				Render(display)
		}
	}

	m.viewport.SetContent(strings.Join(styled, "\n"))

	filterView := m.search.View()

	var searchInfo string
	if hasSearch {
		searchInfo = theme.ResourceCountStyle.Render(fmt.Sprintf(" %d/%d matches", m.searchIdx+1, len(m.searches)))
	}

	help := theme.HelpStyle.Render(" ↑↓ scroll • / search • n/N next match • q back")

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"\n",
		m.viewport.View(),
		"\n",
		filterView,
		searchInfo,
		"\n",
		help,
	)
}
