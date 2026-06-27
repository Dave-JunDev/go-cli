package views

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/dave/kube-tui/internal/k8s"
	"github.com/dave/kube-tui/internal/model"
	"github.com/dave/kube-tui/internal/tui/theme"
	"github.com/dave/kube-tui/internal/tui/components"
)

type ResourceSelectedMsg struct {
	Resource model.K8sResource
}

type ResourceModel struct {
	namespace      string
	resourceType   string
	resources      []model.K8sResource
	filtered       []model.K8sResource
	cursor         int
	filter         *components.Filter
	resourceMgr    *k8s.ResourceManager
	loading        bool
	err            error
	statusBar      *components.StatusBar
	cluster        model.Cluster
	resourceTypes  []model.ResourceType
	typeFiltered   []model.ResourceType
	typeCursor     int
	selectingType  bool
	width          int
	height         int
}

func NewResourceModel() *ResourceModel {
	return &ResourceModel{
		filter:        components.NewFilter("Search resources...", nil),
		resourceTypes: model.ResourceTypes,
		typeFiltered:  model.ResourceTypes,
		selectingType: true,
	}
}

func (m *ResourceModel) SetResourceManager(rm *k8s.ResourceManager) {
	m.resourceMgr = rm
}

func (m *ResourceModel) SetStatusBar(sb *components.StatusBar) {
	m.statusBar = sb
}

func (m *ResourceModel) SetCluster(c model.Cluster) {
	m.cluster = c
}

func (m *ResourceModel) SetResourceTypes(types []model.ResourceType) {
	m.resourceTypes = types
	m.typeFiltered = types
}

func (m *ResourceModel) SetNamespace(ns string) {
	m.namespace = ns
}

func (m *ResourceModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.filter.SetWidth(theme.ContentWidth(w) - 4)
}

func (m *ResourceModel) ResetView() {
	m.selectingType = true
	m.resourceType = ""
	m.resources = nil
	m.filtered = nil
	m.cursor = 0
	m.typeFiltered = m.resourceTypes
	m.typeCursor = 0
	m.loading = false
	m.err = nil
}

func (m *ResourceModel) Init() tea.Cmd {
	return nil
}

func (m *ResourceModel) loadResources() tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		resources, err := m.resourceMgr.ListResources(m.namespace, m.resourceType)
		if err != nil {
			return errMsg{err: fmt.Errorf("list %s: %w", m.resourceType, err)}
		}
		return resourcesLoadedMsg{resources: resources}
	}
}

type resourcesLoadedMsg struct {
	resources []model.K8sResource
}

func (m *ResourceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case resourcesLoadedMsg:
		m.resources = msg.resources
		m.filtered = msg.resources
		m.loading = false
		m.cursor = 0
		m.sortFiltered()
		if m.statusBar != nil {
			m.statusBar.SetItems(len(m.resources))
			m.statusBar.SetResource(m.resourceType)
		}
		return m, nil

	case errMsg:
		m.err = msg.err
		m.loading = false
		if m.statusBar != nil {
			m.statusBar.SetError(msg.err.Error())
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.selectingType {
			return m.handleTypeSelection(msg)
		}
		if m.filter.Active() {
			return m.handleFilteredKeyMsg(msg)
		}
		return m.handleResourceList(msg)
	}

	return m, nil
}

func (m *ResourceModel) handleFilteredKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.filter.SetWidth(theme.ContentWidth(m.width) - 4)
	cmd := m.filter.Update(msg)
	m.applyFilter()

	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
		if m.cursor < 0 {
			m.cursor = 0
		}
	}

	switch msg.String() {
	case "esc":
		m.filter.Blur()
		m.filter.SetValue("")
		m.applyFilter()
		if m.cursor >= len(m.filtered) {
			m.cursor = len(m.filtered) - 1
			if m.cursor < 0 {
				m.cursor = 0
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
	case "enter":
		if len(m.filtered) > 0 {
			selected := m.filtered[m.cursor]
			return m, func() tea.Msg {
				return ResourceSelectedMsg{Resource: selected}
			}
		}
	}

	return m, cmd
}

func (m *ResourceModel) handleTypeSelection(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if m.filter.Active() {
		m.filter.SetWidth(theme.ContentWidth(m.width) - 4)
		cmd := m.filter.Update(msg)
		m.applyTypeFilter()
		if m.typeCursor >= len(m.typeFiltered) {
			m.typeCursor = len(m.typeFiltered) - 1
			if m.typeCursor < 0 {
				m.typeCursor = 0
			}
		}
		switch keyMsg.String() {
		case "esc":
			m.filter.Blur()
			m.filter.SetValue("")
			m.typeFiltered = m.resourceTypes
			m.typeCursor = 0
		case "enter":
			if len(m.typeFiltered) > 0 {
				selectedType := m.typeFiltered[m.typeCursor]
				m.resourceType = selectedType.Plural
				m.selectingType = false
				m.filter.Blur()
				m.filter.SetValue("")
				return m, m.loadResources()
			}
		case "up", "k":
			if m.typeCursor > 0 {
				m.typeCursor--
			}
		case "down", "j":
			if m.typeCursor < len(m.typeFiltered)-1 {
				m.typeCursor++
			}
		case "pgup":
			m.typeCursor -= 10
			if m.typeCursor < 0 {
				m.typeCursor = 0
			}
		case "pgdown", " ":
			m.typeCursor += 10
			if m.typeCursor >= len(m.typeFiltered) {
				m.typeCursor = len(m.typeFiltered) - 1
			}
		}
		return m, cmd
	}

	switch keyMsg.String() {
	case "up", "k":
		if m.typeCursor > 0 {
			m.typeCursor--
		}

	case "down", "j":
		if m.typeCursor < len(m.typeFiltered)-1 {
			m.typeCursor++
		}

	case "pgup":
		m.typeCursor -= 10
		if m.typeCursor < 0 {
			m.typeCursor = 0
		}

	case "pgdown", " ":
		m.typeCursor += 10
		if m.typeCursor >= len(m.typeFiltered) {
			m.typeCursor = len(m.typeFiltered) - 1
		}

	case "enter":
		if len(m.typeFiltered) > 0 {
			selectedType := m.typeFiltered[m.typeCursor]
			m.resourceType = selectedType.Plural
			m.selectingType = false
			m.filter.SetValue("")
			return m, m.loadResources()
		}

	case "/":
		m.typeFiltered = m.resourceTypes
		m.typeCursor = 0
		return m, m.filter.Focus()

	case "backspace", "q":
		return m, popViewCmd()

	case "ctrl+c":
		return m, tea.Quit
	}

	return m, nil
}

func (m *ResourceModel) applyTypeFilter() {
	query := m.filter.Value()
	if query == "" {
		m.typeFiltered = m.resourceTypes
		return
	}
	targets := make([]string, len(m.resourceTypes))
	for i, rt := range m.resourceTypes {
		targets[i] = rt.Name + " " + rt.Plural + " " + rt.Short
	}
	matches := fuzzy.Find(query, targets)
	var filtered []model.ResourceType
	for _, match := range matches {
		filtered = append(filtered, m.resourceTypes[match.Index])
	}
	m.typeFiltered = filtered
}

func (m *ResourceModel) handleResourceList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if len(m.filtered) == 0 {
			return m, nil
		}
		selected := m.filtered[m.cursor]
		return m, func() tea.Msg {
			return ResourceSelectedMsg{Resource: selected}
		}

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}

	case "pgup":
		m.cursor -= 10
		if m.cursor < 0 {
			m.cursor = 0
		}

	case "pgdown", " ":
		m.cursor += 10
		if m.cursor >= len(m.filtered) {
			m.cursor = len(m.filtered) - 1
		}

	case "/":
		return m, m.filter.Focus()

	case "esc":
		if m.filter.Active() {
			m.filter.Blur()
		}

	case "backspace":
		m.selectingType = true
		m.resourceType = ""
		m.resources = nil
		m.filtered = nil
		m.cursor = 0
		m.typeFiltered = m.resourceTypes
		m.typeCursor = 0
		return m, nil

	case "q":
		return m, popViewCmd()

	case "r":
		return m, m.loadResources()

	case "l":
		if m.resourceType == "pods" && len(m.filtered) > 0 {
			selected := m.filtered[m.cursor]
			return m, func() tea.Msg {
				return ShowLogMsg{Resource: selected}
			}
		}
		return m, nil

	case "t":
		m.selectingType = true
		m.resourceType = ""
		m.resources = nil
		m.filtered = nil
		m.cursor = 0
		m.typeFiltered = m.resourceTypes
		m.typeCursor = 0
		return m, nil

	case "ctrl+c":
		return m, tea.Quit
	}

	return m, nil
}

func (m *ResourceModel) sortFiltered() {
	sort.Slice(m.filtered, func(i, j int) bool {
		return m.filtered[i].Name < m.filtered[j].Name
	})
}

func (m *ResourceModel) applyFilter() {
	query := m.filter.Value()
	if query == "" {
		m.filtered = m.resources
		m.sortFiltered()
		return
	}

	targets := make([]string, len(m.resources))
	for i, r := range m.resources {
		targets[i] = r.Name + " " + r.Status
	}
	matches := fuzzy.Find(query, targets)
	var filtered []model.K8sResource
	for _, match := range matches {
		filtered = append(filtered, m.resources[match.Index])
	}
	m.filtered = filtered
	m.sortFiltered()
}

func (m *ResourceModel) View() string {
	if m.selectingType {
		return m.viewTypeSelector()
	}

	if m.loading {
		return theme.SpinnerStyle.Render(fmt.Sprintf(" Loading %s in %s...", m.resourceType, m.namespace))
	}

	cw := theme.ContentWidth(m.width)

	title := theme.TitleStyle.Copy().Width(cw).Render(fmt.Sprintf("☰  %s", m.resourceType))
	nsInfo := theme.SubtitleStyle.Copy().Width(cw).Render(fmt.Sprintf(" Namespace: %s", m.namespace))
	countInfo := theme.ResourceCountStyle.Render(fmt.Sprintf(" %d resources", len(m.filtered)))

	colName := cw - 30
	if colName < 20 {
		colName = 20
	}

	var headerCells []string
	headers := []string{"NAME", "STATUS", "AGE"}
	widths := []int{colName, 20, 10}

	headerLipgloss := lipgloss.NewStyle().
		Background(lipgloss.Color("#0D0D1A")).
		Foreground(theme.Gold).
		Bold(true)

	for i, h := range headers {
		headerCells = append(headerCells, lipgloss.NewStyle().Width(widths[i]).Render(h))
	}
	header := headerLipgloss.Render(lipgloss.JoinHorizontal(lipgloss.Top, headerCells...))

	maxRows := m.height - 8
	if maxRows < 3 {
		maxRows = 3
	}
	totalRows := len(m.filtered)
	start, end := visibleWindow(m.cursor, totalRows, maxRows)

	rowStrs := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		r := m.filtered[i]
		var cells []string
		name := theme.Truncate(r.Name, widths[0])
		status := theme.Truncate(r.Status, widths[1])
		age := theme.Truncate(r.Age, widths[2])

		if i == m.cursor {
			cells = append(cells, theme.SelectedRowStyle.Width(widths[0]).Render(name))
			cells = append(cells, theme.SelectedRowStyle.Width(widths[1]).Render(status))
			cells = append(cells, theme.SelectedRowStyle.Width(widths[2]).Render(age))
		} else {
			cells = append(cells, theme.NormalRowStyle.Width(widths[0]).Render(name))
			cells = append(cells, theme.NormalRowStyle.Width(widths[1]).Render(theme.ColoredStatus(status)))
			cells = append(cells, theme.NormalRowStyle.Width(widths[2]).Render(age))
		}

		line := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
		if i == m.cursor {
			line = lipgloss.NewStyle().Foreground(theme.HotPink).Render("▸ ") + line
		} else {
			line = "  " + line
		}
		rowStrs = append(rowStrs, line)
	}

	content := strings.Join(rowStrs, "\n")

	m.filter.SetWidth(cw - 4)
	filterView := m.filter.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		nsInfo,
		countInfo,
		"\n",
		header,
		content,
		filterView,
		"\n",
		theme.HelpStyle.Render(" ↑↓ navigate • Enter detail • / search • r refresh • l logs • t type • q back • ← selector"),
	)
}

func (m *ResourceModel) viewTypeSelector() string {
	cw := theme.ContentWidth(m.width)

	title := theme.TitleStyle.Copy().Width(cw).Render("☰  SELECT RESOURCE TYPE")
	clusterInfo := theme.SubtitleStyle.Copy().Width(cw).Render(fmt.Sprintf(" Cluster: %s / Namespace: %s", m.cluster.Name, m.namespace))

	displayTypes := m.typeFiltered
	if len(displayTypes) == 0 {
		displayTypes = m.resourceTypes
	}

	isSearching := m.filter.Active()
	countInfo := theme.ResourceCountStyle.Render(fmt.Sprintf(" %d resource types", len(m.resourceTypes)))
	if isSearching {
		countInfo = theme.ResourceCountStyle.Render(fmt.Sprintf(" %d/%d resource types", len(displayTypes), len(m.resourceTypes)))
	}

	maxVisible := 7
	if isSearching {
		maxVisible = len(displayTypes)
	}

	showAll := len(displayTypes) <= maxVisible || m.typeCursor >= maxVisible || isSearching

	visibleTypes := displayTypes
	if !showAll {
		visibleTypes = displayTypes[:maxVisible]
	}

	colName := cw - 40
	if colName < 15 {
		colName = 15
	}
	colShort := 12
	colPlural := 15
	colGroup := cw - colName - colShort - colPlural
	if colGroup < 10 {
		colGroup = 10
	}

	headerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#0D0D1A")).
		Foreground(theme.Gold).
		Bold(true)

	typeHeaders := []string{"NAME", "SHORT", "PLURAL", "API GROUP"}
	typeWidths := []int{colName, colShort, colPlural, colGroup}
	var headerCells []string
	for i, h := range typeHeaders {
		headerCells = append(headerCells, lipgloss.NewStyle().Width(typeWidths[i]).Render(h))
	}
	typeHeader := headerStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, headerCells...))

	rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0"))

	var entries []string
	for i, rt := range visibleTypes {
		var cells []string
		name := theme.Truncate(rt.Name, typeWidths[0])
		short := theme.Truncate(rt.Short, typeWidths[1])
		plural := theme.Truncate(rt.Plural, typeWidths[2])
		group := theme.Truncate(rt.GroupVersion, typeWidths[3])

		if i == m.typeCursor {
			cells = append(cells, theme.SelectedRowStyle.Width(typeWidths[0]).Render(name))
			cells = append(cells, theme.SelectedRowStyle.Width(typeWidths[1]).Render(short))
			cells = append(cells, theme.SelectedRowStyle.Width(typeWidths[2]).Render(plural))
			cells = append(cells, theme.SelectedRowStyle.Width(typeWidths[3]).Render(group))
		} else {
			cells = append(cells, rowStyle.Width(typeWidths[0]).Render(name))
			cells = append(cells, rowStyle.Width(typeWidths[1]).Render(short))
			cells = append(cells, rowStyle.Width(typeWidths[2]).Render(plural))
			cells = append(cells, rowStyle.Width(typeWidths[3]).Render(group))
		}
		line := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
		if i == m.typeCursor {
			line = lipgloss.NewStyle().Foreground(theme.HotPink).Render("▸ ") + line
		} else {
			line = "  " + line
		}
		entries = append(entries, line)
	}
	if !showAll {
		remaining := len(displayTypes) - maxVisible
		more := lipgloss.NewStyle().Foreground(theme.MutedText).Italic(true).Render(
			fmt.Sprintf("  … and %d more — press / to search", remaining),
		)
		entries = append(entries, more)
	}

	// Limit visible types to available height
	overhead := 7
	if isSearching {
		overhead = 8
	}
	maxRows := m.height - overhead
	if maxRows < 3 {
		maxRows = 3
	}
	start, end := visibleWindow(m.typeCursor, len(entries), maxRows)
	entries = entries[start:end]

	content := lipgloss.JoinVertical(lipgloss.Left, entries...)

	m.filter.SetWidth(cw - 4)
	filterView := m.filter.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		clusterInfo,
		countInfo,
		"\n",
		typeHeader,
		content,
		filterView,
		"\n",
		theme.HelpStyle.Render(" ↑↓ navigate • Enter select • / search • ← back to namespaces"),
	)
}

func visibleWindow(cursor, total, max int) (start, end int) {
	if total <= max {
		return 0, total
	}
	start = cursor - max/2
	if start < 0 {
		start = 0
	}
	end = start + max
	if end > total {
		end = total
		start = end - max
		if start < 0 {
			start = 0
		}
	}
	return
}

