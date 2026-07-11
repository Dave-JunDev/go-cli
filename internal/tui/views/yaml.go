package views

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dave/kube-tui/internal/model"
	"github.com/dave/kube-tui/internal/tui/theme"
	"github.com/dave/kube-tui/internal/tui/components"
)

type YamlModel struct {
	yaml      string
	resource  model.K8sResource
	yamlLines []string
	yamlOff   int
	width     int
	height    int
	search    *components.Filter
	searches  []int
	searchIdx int
	mode      string
	saveInput textinput.Model
	saveMode  bool
}

func NewYamlModel() *YamlModel {
	si := textinput.New()
	si.Placeholder = "~/k8s-ui/resource.yaml"
	si.Prompt = "💾 "
	si.Width = 60
	si.CharLimit = 200
	return &YamlModel{
		search:    components.NewFilter("Search...", nil),
		mode:      "YAML",
		saveInput: si,
	}
}

func (m *YamlModel) SetMode(mode string) {
	m.mode = mode
	title := "Search YAML..."
	if mode == "Describe" {
		title = "Search describe..."
	}
	m.search = components.NewFilter(title, nil)
}

func (m *YamlModel) SetYAML(yaml string, r model.K8sResource) {
	m.yaml = yaml
	m.resource = r
	m.yamlLines = strings.Split(yaml, "\n")
	m.yamlOff = 0
}

func (m *YamlModel) Init() tea.Cmd {
	return nil
}

func (m *YamlModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.saveInput.Width = theme.ContentWidth(m.width) - 10
}

func (m *YamlModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.saveInput.Width = theme.ContentWidth(m.width) - 10
		return m, nil

	case tea.KeyMsg:
		if m.saveMode {
			return m.handleSaveKey(msg)
		}
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
	case "up":
		if m.yamlOff > 0 {
			m.yamlOff--
		}

	case "down":
		visible := m.visibleLines()
		if m.yamlOff < len(m.yamlLines)-visible {
			m.yamlOff++
		}

	case "pgup":
		visible := m.visibleLines()
		m.yamlOff -= visible
		if m.yamlOff < 0 {
			m.yamlOff = 0
		}

	case "pgdown":
		visible := m.visibleLines()
		m.yamlOff += visible
		max := len(m.yamlLines) - visible
		if m.yamlOff > max {
			m.yamlOff = max
		}
		if m.yamlOff < 0 {
			m.yamlOff = 0
		}

	case "g":
		m.yamlOff = 0

	case "G":
		visible := m.visibleLines()
		m.yamlOff = len(m.yamlLines) - visible
		if m.yamlOff < 0 {
			m.yamlOff = 0
		}

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

	case "s":
		m.saveMode = true
		m.saveInput.SetValue(m.defaultSavePath())
		return m, m.saveInput.Focus()

	case "backspace", "esc", "q":
		return m, popViewCmd()

	case "ctrl+c":
		return m, tea.Quit
	}

	return m, nil
}

func (m *YamlModel) handleSaveKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		path := m.saveInput.Value()
		if path == "" {
			m.saveMode = false
			return m, nil
		}
		m.saveMode = false
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return m, tea.Println(fmt.Sprintf("Save error: creating directory: %v", err))
		}
		if err := os.WriteFile(path, []byte(m.yaml), 0644); err != nil {
			return m, tea.Println(fmt.Sprintf("Save error: writing file: %v", err))
		}
		return m, tea.Println(fmt.Sprintf("Saved to %s", path))

	case "esc":
		m.saveMode = false
		m.saveInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.saveInput, cmd = m.saveInput.Update(msg)
	return m, cmd
}

func (m *YamlModel) defaultSavePath() string {
	home, _ := os.UserHomeDir()
	ts := time.Now().Format("20060102_150405")
	name := fmt.Sprintf("%s-%s-%s.%s", strings.ToLower(m.mode), m.resource.Namespace, m.resource.Name, "yaml")
	if m.mode == "Describe" {
		name = fmt.Sprintf("describe-%s-%s.txt", m.resource.Namespace, m.resource.Name)
	}
	return filepath.Join(home, "k8s-ui", ts+"-"+name)
}

func (m *YamlModel) visibleLines() int {
	extra := 0
	if m.search.Active() {
		extra++
	}
	if len(m.searches) > 0 {
		extra++
	}
	if m.saveMode {
		extra++
	}
	// Matches maxRows in View(): content = m.height - 8 - extra
	h := m.height - 8 - extra
	if h < 3 {
		h = 3
	}
	return h
}

func (m *YamlModel) runSearch() {
	query := strings.ToLower(m.search.Value())
	m.searches = nil
	m.searchIdx = 0

	if query == "" {
		return
	}

	for i, line := range m.yamlLines {
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
	visible := m.visibleLines()
	m.yamlOff = lineNum - visible/2
	if m.yamlOff < 0 {
		m.yamlOff = 0
	}
}

func (m *YamlModel) View() string {
	cw := theme.ContentWidth(m.width)

	query := strings.ToLower(m.search.Value())
	hasSearch := query != "" && len(m.searches) > 0

	// cw includes the 2-char TitleStyle padding, so content needs full width
	contentW := cw
	if contentW > m.width {
		contentW = m.width
	}
	extra := 0
	if m.search.Active() {
		extra++
	}
	if len(m.searches) > 0 {
		extra++
	}
	if m.saveMode {
		extra++
	}
	maxRows := m.height - 8 - extra
	if maxRows < 3 {
		maxRows = 3
	}
	start, end := visibleWindow(m.yamlOff, len(m.yamlLines), maxRows)

	styled := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		line := m.yamlLines[i]
		display := line
		if len(display) > contentW {
			display = display[:contentW-1] + "\u2026"
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
			styled = append(styled, lipgloss.NewStyle().
				Foreground(theme.Gold).
				Background(theme.Purple).
				Bold(true).
				Render(display))
		} else if isSearchResult {
			styled = append(styled, lipgloss.NewStyle().
				Foreground(theme.Orange).
				Render(display))
		} else {
			styled = append(styled, lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E0E0E0")).
				Render(display))
		}
	}

	content := strings.Join(styled, "\n")

	title := theme.TitleStyle.Copy().Width(cw).Render(fmt.Sprintf("☰  %s — %s/%s", m.mode, m.resource.Type, m.resource.Name))
	subtitle := theme.SubtitleStyle.Copy().Width(cw).Render(fmt.Sprintf(" %s / %s", m.resource.Namespace, m.resource.Name))

	filterView := m.search.View()

	var searchInfo string
	if hasSearch {
		searchInfo = theme.ResourceCountStyle.Render(fmt.Sprintf(" %d/%d matches", m.searchIdx+1, len(m.searches)))
	}

	help := theme.HelpStyle.Render(" \u2191\u2193 scroll \u2022 / search \u2022 n/N next match \u2022 s save \u2022 g top \u2022 G bottom \u2022 q back")

	out := title + subtitle + "\n" + content
	if filterView != "" {
		out += "\n" + filterView
	}
	if searchInfo != "" {
		out += "\n" + searchInfo
	}
	if m.saveMode {
		out += "\n" + m.saveInput.View()
	}
	out += "\n" + help
	return out
}
