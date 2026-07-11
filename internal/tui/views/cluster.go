package views

import (
	"fmt"
	"sort"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/dave/kube-tui/internal/config"
	"github.com/dave/kube-tui/internal/k8s"
	"github.com/dave/kube-tui/internal/model"
	"github.com/dave/kube-tui/internal/tui/theme"
	"github.com/dave/kube-tui/internal/tui/components"
)

type ClusterSelectedMsg struct {
	Cluster model.Cluster
	Client  interface{}
	Error   error
}

type ClusterModel struct {
	clusters       []model.Cluster
	cursor         int
	filter         *components.Filter
	filtered       []model.Cluster
	cfg            *config.UIConfig
	health         map[string]string // cluster name → "ok" | "unreachable" | "checking"
	err            error
	loading        bool
	statusBar      *components.StatusBar
	width          int
	height         int
}

func NewClusterModel() *ClusterModel {
	return &ClusterModel{
		filter:   components.NewFilter("Search clusters...", nil),
		filtered: []model.Cluster{},
	}
}

func (m *ClusterModel) SetStatusBar(sb *components.StatusBar) {
	m.statusBar = sb
}

func (m *ClusterModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.filter.SetWidth(theme.ContentWidth(w) - 4)
}

func (m *ClusterModel) ResetView() {
	m.loading = false
	m.err = nil
	m.filter.Blur()
	m.filter.SetValue("")
	m.cursor = 0
	m.cfg, _ = config.LoadUIConfig()
	if m.clusters != nil {
		m.filtered = m.clusters
	}
}

func (m *ClusterModel) Init() tea.Cmd {
	return func() tea.Msg {
		entries, err := config.DiscoverKubeconfigs()
		if err != nil {
			return ClusterSelectedMsg{Error: fmt.Errorf("discover kubeconfigs: %w", err)}
		}

		clusters, err := config.LoadClusters(entries)
		if err != nil {
			return ClusterSelectedMsg{Error: fmt.Errorf("load clusters: %w", err)}
		}

		cfg, _ := config.LoadUIConfig()

		return clustersLoadedMsg{clusters: clusters, cfg: cfg}
	}
}

type clustersLoadedMsg struct {
	clusters []model.Cluster
	cfg      *config.UIConfig
}

type healthResultsMsg struct {
	results map[string]string
}

func (m *ClusterModel) checkHealth() tea.Cmd {
	return func() tea.Msg {
		var mu sync.Mutex
		results := make(map[string]string, len(m.clusters))
		var wg sync.WaitGroup

		for _, c := range m.clusters {
			wg.Add(1)
			cluster := c
			go func() {
				defer wg.Done()
				if err := k8s.CheckClusterHealth(cluster); err != nil {
					mu.Lock()
					results[cluster.Name] = "unreachable"
					mu.Unlock()
					return
				}
				mu.Lock()
				results[cluster.Name] = "ok"
				mu.Unlock()
			}()
		}

		wg.Wait()
		return healthResultsMsg{results: results}
	}
}

func (m *ClusterModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case clustersLoadedMsg:
		m.clusters = msg.clusters
		m.filtered = msg.clusters
		m.cfg = msg.cfg
		m.health = make(map[string]string, len(m.clusters))
		for _, c := range m.clusters {
			m.health[c.Name] = "checking"
		}
		m.loading = false
		if m.statusBar != nil {
			m.statusBar.SetItems(len(m.clusters))
		}
		return m, m.checkHealth()

	case healthResultsMsg:
		for name, status := range msg.results {
			m.health[name] = status
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if len(m.filtered) == 0 {
				return m, nil
			}
			selected := m.filtered[m.cursor]
			m.loading = true
			return m, func() tea.Msg {
				return ClusterSelectedMsg{
					Cluster: selected,
				}
			}

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}

		case "/":
			return m, m.filter.Focus()

		case "q":
			if !m.filter.Active() {
				return m, tea.Quit
			}
		case "ctrl+c":
			return m, tea.Quit
		}

	case error:
		m.err = msg
		m.loading = false
		if m.statusBar != nil {
			m.statusBar.SetError(msg.Error())
		}
	}

	// Update filter
	if m.filter.Active() {
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
			m.filter.Blur()
			m.filter.SetValue("")
			m.applyFilter()
			if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
				if m.cursor < 0 {
					m.cursor = 0
				}
			}
			return m, nil
		}

		cmd := m.filter.Update(msg)
		m.applyFilter()
		if m.cursor >= len(m.filtered) {
			m.cursor = len(m.filtered) - 1
			if m.cursor < 0 {
				m.cursor = 0
			}
		}
		return m, cmd
	}

	return m, nil
}

func (m *ClusterModel) applyFilter() {
	query := m.filter.Value()
	if query == "" {
		m.filtered = m.clusters
		return
	}

	targets := make([]string, len(m.clusters))
	for i, c := range m.clusters {
		targets[i] = c.Name + " " + c.Server + " " + c.Context
	}
	matches := fuzzy.Find(query, targets)
	var filtered []model.Cluster
	for _, match := range matches {
		filtered = append(filtered, m.clusters[match.Index])
	}
	m.filtered = filtered
}

func (m *ClusterModel) View() string {
	if m.loading {
		return theme.SpinnerStyle.Render(" Loading clusters...")
	}

	if len(m.clusters) == 0 {
		return theme.ErrorStyle.Render(" No clusters found in ~/.kube/config or ~/.kube/configs/")
	}

	cw := theme.ContentWidth(m.width)
	m.filter.SetWidth(cw - 4)

	title := theme.TitleStyle.Copy().Width(cw).Render("☸  CLUSTER SELECTOR")
	subtitle := theme.SubtitleStyle.Copy().Width(cw).Render(" Select a cluster to connect")
	countInfo := theme.ResourceCountStyle.Render(fmt.Sprintf(" %d clusters available", len(m.clusters)))

	groupStyle := lipgloss.NewStyle().Foreground(theme.HotPink).Bold(true).Underline(true).Padding(0, 1)
	descStyle := lipgloss.NewStyle().Foreground(theme.MutedText)
	selectedDescStyle := lipgloss.NewStyle().Foreground(theme.ElectricBlue)

	okDot := lipgloss.NewStyle().Foreground(theme.Lime).Render("●")
	unreachableDot := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Render("●")
	checkingDot := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("○")

	maxRows := m.height - 6
	if maxRows < 3 {
		maxRows = 3
	}

	// Build config-grouped entries with headers
	cfg := m.cfg
	if cfg == nil {
		cfg = &config.UIConfig{Groups: make(map[string][]string)}
	}
	grouped := config.GroupClusters(m.filtered, cfg)
	var groupNames []string
	for g := range grouped {
		groupNames = append(groupNames, g)
	}
	sort.Strings(groupNames)
	// Move "ungrouped" to end
	for i, g := range groupNames {
		if g == "ungrouped" {
			groupNames = append(groupNames[:i], groupNames[i+1:]...)
			groupNames = append(groupNames, g)
			break
		}
	}

	var allEntries []string
	cursorLine := 0
	totalClusters := 0
	for _, g := range groupNames {
		members := grouped[g]
		allEntries = append(allEntries, groupStyle.Render(" "+g))
		for j, c := range members {
			idx := totalClusters + j
			if idx == m.cursor {
				cursorLine = len(allEntries)
			}
			dot := checkingDot
			if s, ok := m.health[c.Name]; ok {
				switch s {
				case "ok":
					dot = okDot
				case "unreachable":
					dot = unreachableDot
				}
			}
			desc := fmt.Sprintf("%s  %s", c.Context, c.Server)
			if idx == m.cursor {
				line := lipgloss.NewStyle().
					Foreground(theme.Gold).
					Bold(true).
					Background(theme.Purple).
					Padding(0, 1).
					Render(fmt.Sprintf("▸ %s %s", dot, c.Name))
				line += "  " + selectedDescStyle.Render(desc)
				allEntries = append(allEntries, line)
			} else {
				line := lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FFFFFF")).
					Render(fmt.Sprintf("  %s %s", dot, c.Name))
				line += "  " + descStyle.Render(desc)
				allEntries = append(allEntries, line)
			}
		}
		totalClusters += len(members)
	}

	start, end := visibleWindow(cursorLine, len(allEntries), maxRows)
	entries := allEntries[start:end]

	content := lipgloss.JoinVertical(lipgloss.Left, entries...)
	filterView := m.filter.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		subtitle,
		countInfo,
		"\n",
		filterView,
		content,
		"\n",
		theme.HelpStyle.Render(" ↑↓ navigate • Enter select • / search • c config • q quit"),
	)
}

func (m *ClusterModel) Clusters() []model.Cluster {
	return m.clusters
}

func (m *ClusterModel) SelectedCluster() *model.Cluster {
	if len(m.filtered) == 0 {
		return nil
	}
	return &m.filtered[m.cursor]
}


