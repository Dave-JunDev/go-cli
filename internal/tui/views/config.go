package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dave/kube-tui/internal/config"
	"github.com/dave/kube-tui/internal/model"
	"github.com/dave/kube-tui/internal/tui/components"
	"github.com/dave/kube-tui/internal/tui/theme"
)

type ConfigModel struct {
	cfg          *config.UIConfig
	clusters     []model.Cluster
	loading      bool
	width        int
	height       int
	statusBar    *components.StatusBar
	mode         string // "main", "addGroup", "renameGroup", "moveCluster"
	input        textinput.Model
	groupCursor  int
	clusterCursor int
	selectedGroup string
	moveTarget    string
	groups       []string
}

func NewConfigModel() *ConfigModel {
	si := textinput.New()
	si.Prompt = "▸ "
	si.Width = 40
	si.CharLimit = 50
	return &ConfigModel{
		cfg:    &config.UIConfig{Groups: make(map[string][]string)},
		input:  si,
		mode:   "main",
	}
}

func (m *ConfigModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.input.Width = theme.ContentWidth(w) - 10
}

func (m *ConfigModel) SetStatusBar(sb *components.StatusBar) {
	m.statusBar = sb
}

func (m *ConfigModel) SetClusters(clusters []model.Cluster) {
	m.clusters = clusters
}

func (m *ConfigModel) Init() tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		cfg, err := config.LoadUIConfig()
		if err != nil {
			cfg = &config.UIConfig{Groups: make(map[string][]string)}
		}
		return configLoadedMsg{cfg: cfg}
	}
}

type configLoadedMsg struct {
	cfg *config.UIConfig
}

func (m *ConfigModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case configLoadedMsg:
		m.cfg = msg.cfg
		m.loading = false
		m.refreshGroups()
		if m.statusBar != nil {
			m.statusBar.SetMode("config")
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = theme.ContentWidth(m.width) - 10

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *ConfigModel) refreshGroups() {
	m.groups = nil
	for g := range m.cfg.Groups {
		m.groups = append(m.groups, g)
	}
	sort.Strings(m.groups)
}

func (m *ConfigModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case "addGroup":
		return m.handleAddGroup(msg)
	case "renameGroup":
		return m.handleRenameGroup(msg)
	case "moveCluster":
		return m.handleMoveCluster(msg)
	default:
		return m.handleMainKey(msg)
	}
}

func (m *ConfigModel) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.groupCursor > 0 {
			m.groupCursor--
		}
	case "down", "j":
		if m.groupCursor < len(m.groups)-1 {
			m.groupCursor++
		}
	case "enter":
		group := m.groups[m.groupCursor]
		if group == "ungrouped" {
			return m, nil
		}
		m.selectedGroup = group
		m.mode = "moveCluster"
		m.clusterCursor = 0
		return m, nil
	case "a":
		m.mode = "addGroup"
		m.input.SetValue("")
		return m, m.input.Focus()
	case "r":
		group := m.groups[m.groupCursor]
		if group == "ungrouped" {
			return m, nil
		}
		m.selectedGroup = group
		m.mode = "renameGroup"
		m.input.SetValue(group)
		return m, m.input.Focus()
	case "d":
		group := m.groups[m.groupCursor]
		if group == "ungrouped" || group == "" {
			return m, nil
		}
		m.cfg.RemoveGroup(group)
		m.refreshGroups()
		if m.groupCursor >= len(m.groups) {
			m.groupCursor = len(m.groups) - 1
		}
		m.save()
	case "backspace", "esc", "q":
		m.save()
		m.mode = "main"
		return m, popViewCmd()
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m *ConfigModel) handleAddGroup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := strings.TrimSpace(m.input.Value())
		if name != "" {
			m.cfg.Groups[name] = nil
			m.refreshGroups()
			m.groupCursor = 0
			for i, g := range m.groups {
				if g == name {
					m.groupCursor = i
					break
				}
			}
			m.save()
		}
		m.mode = "main"
		m.input.Blur()
		return m, nil
	case "esc":
		m.mode = "main"
		m.input.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *ConfigModel) handleRenameGroup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := strings.TrimSpace(m.input.Value())
		if name != "" && name != m.selectedGroup {
			m.cfg.RenameGroup(m.selectedGroup, name)
			m.refreshGroups()
			m.save()
		}
		m.mode = "main"
		m.input.Blur()
		return m, nil
	case "esc":
		m.mode = "main"
		m.input.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *ConfigModel) handleMoveCluster(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	members := m.cfg.Groups[m.selectedGroup]
	if members == nil {
		members = []string{}
	}

	// Clusters not in ANY group (available for adding)
	assigned := make(map[string]bool)
	for _, mems := range m.cfg.Groups {
		for _, nm := range mems {
			assigned[nm] = true
		}
	}
	var available []string
	for _, c := range m.clusters {
		if !assigned[c.Name] {
			available = append(available, c.Name)
		}
	}

	totalItems := len(members) + len(available)

	switch msg.String() {
	case "up", "k":
		if m.clusterCursor > 0 {
			m.clusterCursor--
		}
	case "down", "j":
		if m.clusterCursor < totalItems-1 {
			m.clusterCursor++
		}
	case "enter":
		if m.clusterCursor < len(members) {
			m.cfg.AssignCluster(members[m.clusterCursor], "")
			m.save()
		} else {
			ai := m.clusterCursor - len(members)
			if ai < len(available) {
				m.cfg.AssignCluster(available[ai], m.selectedGroup)
				m.save()
			}
		}
		return m, nil
	case "backspace", "esc", "q":
		m.mode = "main"
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m *ConfigModel) save() {
	if err := m.cfg.Save(); err != nil {
		if m.statusBar != nil {
			m.statusBar.SetError(fmt.Sprintf("save config: %v", err))
		}
	}
}

func (m *ConfigModel) View() string {
	if m.loading {
		return theme.SpinnerStyle.Render(" Loading config...")
	}

	cw := theme.ContentWidth(m.width)

	title := theme.TitleStyle.Copy().Width(cw).Render("⚙  CONFIGURATION")
	subtitle := theme.SubtitleStyle.Copy().Width(cw).Render(" Manage cluster groups")

	switch m.mode {
	case "addGroup":
		return m.viewAddGroup(cw, title, subtitle)
	case "renameGroup":
		return m.viewRenameGroup(cw, title, subtitle)
	case "moveCluster":
		return m.viewMoveCluster(cw, title, subtitle)
	default:
		return m.viewMain(cw, title, subtitle)
	}
}

func (m *ConfigModel) viewMain(cw int, title, subtitle string) string {
	groupStyle := lipgloss.NewStyle().Foreground(theme.HotPink).Bold(true)
	clusterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0"))
	emptyStyle := lipgloss.NewStyle().Foreground(theme.MutedText).Italic(true)
	selectedStyle := lipgloss.NewStyle().Foreground(theme.Gold).Bold(true).Background(theme.Purple).Padding(0, 1)
	help := theme.HelpStyle.Render(" ↑↓ navigate • a add group • r rename • d delete • enter manage • q back")

	maxRows := m.height - 8
	if maxRows < 3 {
		maxRows = 3
	}

	if len(m.groups) == 0 {
		content := theme.HelpStyle.Render(" No groups yet. Press 'a' to add one.")
		return fmt.Sprintf("%s\n%s\n\n%s\n\n%s", title, subtitle, content, help)
	}

	start, end := visibleWindow(m.groupCursor, len(m.groups), maxRows)

	var lines []string
	for i := start; i < end; i++ {
		g := m.groups[i]
		members := m.cfg.Groups[g]
		if g == "ungrouped" {
			// Find clusters not in any group
			inGroup := make(map[string]bool)
			for _, mems := range m.cfg.Groups {
				for _, m := range mems {
					inGroup[m] = true
				}
			}
			var ungrouped []string
			for _, c := range m.clusters {
				if !inGroup[c.Name] {
					ungrouped = append(ungrouped, c.Name)
				}
			}
			members = ungrouped
		}
		if i == m.groupCursor {
			lines = append(lines, selectedStyle.Render(fmt.Sprintf("▸ %s (%d)", g, len(members))))
		} else {
			lines = append(lines, groupStyle.Render(fmt.Sprintf("  %s (%d)", g, len(members))))
		}
		if i == m.groupCursor {
			// Show cluster names for selected group
			memberNames := members
			if len(memberNames) > 5 {
				memberNames = memberNames[:5]
			}
			for _, cn := range memberNames {
				display := cn
				if len(display) > cw-10 {
					display = display[:cw-13] + "..."
				}
				lines = append(lines, clusterStyle.Render("    " + display))
			}
			if len(members) > 5 {
				lines = append(lines, emptyStyle.Render(fmt.Sprintf("    ... and %d more", len(members)-5)))
			}
			if len(members) == 0 {
				lines = append(lines, emptyStyle.Render("    (empty)"))
			}
		}
	}

	content := strings.Join(lines, "\n")
	return fmt.Sprintf("%s\n%s\n\n%s\n\n%s", title, subtitle, content, help)
}

func (m *ConfigModel) viewAddGroup(cw int, title, subtitle string) string {
	return fmt.Sprintf("%s\n%s\n\n%s\n\n%s", title, subtitle, m.input.View(), theme.HelpStyle.Render(" enter confirm • esc cancel"))
}

func (m *ConfigModel) viewRenameGroup(cw int, title, subtitle string) string {
	return fmt.Sprintf("%s\n%s\n\nRename group \"%s\":\n%s\n\n%s", title, subtitle, m.selectedGroup, m.input.View(), theme.HelpStyle.Render(" enter confirm • esc cancel"))
}

func (m *ConfigModel) viewMoveCluster(cw int, title, subtitle string) string {
	groupStyle := lipgloss.NewStyle().Foreground(theme.HotPink).Bold(true)
	members := m.cfg.Groups[m.selectedGroup]
	if members == nil {
		members = []string{}
	}

	// Add available (not in any group) clusters as "add" option
	inGroup := make(map[string]bool)
	for _, mems := range m.cfg.Groups {
		for _, m := range mems {
			inGroup[m] = true
		}
	}
	var available []string
	for _, c := range m.clusters {
		if !inGroup[c.Name] {
			available = append(available, c.Name)
		}
	}

	maxRows := m.height - 8
	if maxRows < 3 {
		maxRows = 3
	}
	start, end := visibleWindow(m.clusterCursor, len(members)+len(available), maxRows)

	selectedStyle := lipgloss.NewStyle().Foreground(theme.Gold).Bold(true).Background(theme.Purple).Padding(0, 1)
	clusterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0"))
	availableStyle := lipgloss.NewStyle().Foreground(theme.Lime)

	var lines []string
	lines = append(lines, groupStyle.Render(fmt.Sprintf(" %s:", m.selectedGroup)))

	totalItems := len(members) + len(available)
	for i := start; i < end && i < totalItems; i++ {
		if i < len(members) {
			if i == m.clusterCursor {
				lines = append(lines, selectedStyle.Render(fmt.Sprintf("▸ %s  [enter to remove]", members[i])))
			} else {
				lines = append(lines, clusterStyle.Render(fmt.Sprintf("  %s", members[i])))
			}
		} else {
			ai := i - len(members)
			if ai < len(available) {
				if i == m.clusterCursor {
					lines = append(lines, selectedStyle.Render(fmt.Sprintf("▸ + %s  [enter to add]", available[ai])))
				} else {
					lines = append(lines, availableStyle.Render(fmt.Sprintf("  + %s", available[ai])))
				}
			}
		}
	}

	if len(members) == 0 && len(available) == 0 {
		lines = append(lines, theme.HelpStyle.Render("  no clusters available"))
	}

	content := strings.Join(lines, "\n")
	help := theme.HelpStyle.Render(" ↑↓ select • enter remove/add • a done • q back")

	return fmt.Sprintf("%s\n%s\n\n%s\n\n%s", title, subtitle, content, help)
}
