package views

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/dave/kube-tui/internal/k8s"
	"github.com/dave/kube-tui/internal/model"
	"github.com/dave/kube-tui/internal/tui/components"
	"github.com/dave/kube-tui/internal/tui/theme"
)

type errMsg struct {
	err error
}

type PopViewMsg struct{}

func popViewCmd() tea.Cmd {
	return func() tea.Msg {
		return PopViewMsg{}
	}
}

type NamespaceSelectedMsg struct {
	Namespace      string
	Client         interface{}
}

type NamespaceModel struct {
	namespaces     []string
	filtered       []string
	cursor         int
	filter         *components.Filter
	resourceMgr    *k8s.ResourceManager
	loading        bool
	err            error
	statusBar      *components.StatusBar
	cluster        model.Cluster
	width          int
	height         int
}

func NewNamespaceModel() *NamespaceModel {
	return &NamespaceModel{
		filter: components.NewFilter("Search namespaces...", nil),
	}
}

func (m *NamespaceModel) SetResourceManager(rm *k8s.ResourceManager) {
	m.resourceMgr = rm
}

func (m *NamespaceModel) SetStatusBar(sb *components.StatusBar) {
	m.statusBar = sb
}

func (m *NamespaceModel) SetCluster(c model.Cluster) {
	m.cluster = c
}

func (m *NamespaceModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.filter.SetWidth(theme.ContentWidth(w) - 4)
}

func (m *NamespaceModel) ResetView() {
	m.namespaces = nil
	m.filtered = nil
	m.cursor = 0
	m.loading = false
	m.err = nil
	m.filter.Blur()
	m.filter.SetValue("")
}

func (m *NamespaceModel) Init() tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		namespaces, err := m.resourceMgr.ListNamespaces()
		if err != nil {
			return errMsg{err: fmt.Errorf("list namespaces: %w", err)}
		}
		return namespacesLoadedMsg{namespaces: namespaces}
	}
}

type namespacesLoadedMsg struct {
	namespaces []string
}

func (m *NamespaceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case namespacesLoadedMsg:
		m.namespaces = msg.namespaces
		m.filtered = msg.namespaces
		m.loading = false
		if m.statusBar != nil {
			m.statusBar.SetItems(len(m.namespaces))
		}
		return m, nil

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
			return m, func() tea.Msg {
				return NamespaceSelectedMsg{
					Namespace: selected,
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

		case "backspace", "q":
			if !m.filter.Active() {
				return m, popViewCmd()
			}
		}

	case errMsg:
		m.err = msg.err
		m.loading = false
		if m.statusBar != nil {
			m.statusBar.SetError(msg.err.Error())
		}
	}

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

func (m *NamespaceModel) applyFilter() {
	query := m.filter.Value()
	if query == "" {
		m.filtered = m.namespaces
		return
	}

	matches := fuzzy.Find(query, m.namespaces)
	var filtered []string
	for _, match := range matches {
		filtered = append(filtered, m.namespaces[match.Index])
	}
	m.filtered = filtered
}

func (m *NamespaceModel) View() string {
	if m.loading {
		return theme.SpinnerStyle.Render(" Loading namespaces...")
	}

	cw := theme.ContentWidth(m.width)
	m.filter.SetWidth(cw - 4)

	title := theme.TitleStyle.Copy().Width(cw).Render("■  NAMESPACES")
	clusterInfo := theme.SubtitleStyle.Copy().Width(cw).Render(fmt.Sprintf(" Cluster: %s", m.cluster.Name))
	countInfo := theme.ResourceCountStyle.Render(fmt.Sprintf(" %d namespaces", len(m.filtered)))

	maxRows := m.height - 6
	if maxRows < 3 {
		maxRows = 3
	}
	start, end := visibleWindow(m.cursor, len(m.filtered), maxRows)

	var entries []string
	for i := start; i < end; i++ {
		ns := m.filtered[i]
		prefix := "  "
		style := theme.NamespaceStyle

		if i == m.cursor {
			prefix = "▸ "
			style = lipgloss.NewStyle().
				Foreground(theme.Gold).
				Bold(true).
				Background(theme.Purple).
				Padding(0, 1)
		}

		entries = append(entries, style.Render(prefix+ns))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, entries...)

	filterView := m.filter.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		clusterInfo,
		countInfo,
		"\n",
		filterView,
		content,
		"\n",
		theme.HelpStyle.Render(" ↑↓ navigate • Enter select • / search • ← back"),
	)
}

func (m *NamespaceModel) SelectedNamespace() string {
	if len(m.filtered) == 0 {
		return ""
	}
	return m.filtered[m.cursor]
}
