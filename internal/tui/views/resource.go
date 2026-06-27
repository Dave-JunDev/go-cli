package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	vp             viewport.Model
}

func NewResourceModel() *ResourceModel {
	vp := viewport.New(80, 20)
	return &ResourceModel{
		filter:        components.NewFilter("Search resources...", nil),
		resourceTypes: model.ResourceTypes,
		typeFiltered:  model.ResourceTypes,
		selectingType: true,
		vp:            vp,
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

func (m *ResourceModel) SetNamespace(ns string) {
	m.namespace = ns
}

func (m *ResourceModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.filter.SetWidth(theme.ContentWidth(w) - 4)
	m.vp.Width = theme.ViewportWidth(w)
	m.vp.Height = m.vpHeight()
}

func (m *ResourceModel) vpHeight() int {
	// Reserve: title (1) + nsInfo (1) + countInfo (1) + blank (1) + header (1) + filter (1) + blank (1) + help (1) = 8
	return m.height - 8
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
		m.scrollTypeCursor()
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
				m.scrollTypeCursor()
			}
		case "down", "j":
			if m.typeCursor < len(m.typeFiltered)-1 {
				m.typeCursor++
				m.scrollTypeCursor()
			}
		case "pgup":
			m.typeCursor -= m.vp.Height
			if m.typeCursor < 0 {
				m.typeCursor = 0
			}
			m.scrollTypeCursor()
		case "pgdown", " ":
			m.typeCursor += m.vp.Height
			if m.typeCursor >= len(m.typeFiltered) {
				m.typeCursor = len(m.typeFiltered) - 1
			}
			m.scrollTypeCursor()
		}
		return m, cmd
	}

	switch keyMsg.String() {
	case "up", "k":
		if m.typeCursor > 0 {
			m.typeCursor--
			m.scrollTypeCursor()
		}

	case "down", "j":
		if m.typeCursor < len(m.typeFiltered)-1 {
			m.typeCursor++
			m.scrollTypeCursor()
		}

	case "pgup":
		m.typeCursor -= m.vp.Height
		if m.typeCursor < 0 {
			m.typeCursor = 0
		}
		m.scrollTypeCursor()

	case "pgdown", " ":
		m.typeCursor += m.vp.Height
		if m.typeCursor >= len(m.typeFiltered) {
			m.typeCursor = len(m.typeFiltered) - 1
		}
		m.scrollTypeCursor()

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
	var filtered []model.ResourceType
	for _, rt := range m.resourceTypes {
		if caseInsensitiveContains(rt.Name, query) ||
			caseInsensitiveContains(rt.Plural, query) ||
			caseInsensitiveContains(rt.Short, query) {
			filtered = append(filtered, rt)
		}
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
			m.scrollToCursor()
		}

	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			m.scrollToCursor()
		}

	case "pgup":
		m.cursor -= m.vp.Height
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.scrollToCursor()

	case "pgdown", " ":
		m.cursor += m.vp.Height
		if m.cursor >= len(m.filtered) {
			m.cursor = len(m.filtered) - 1
		}
		m.scrollToCursor()

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

func (m *ResourceModel) scrollToCursor() {
	row := m.cursor
	viewportStart := m.vp.YOffset
	viewportEnd := viewportStart + m.vp.Height - 1
	if row < viewportStart {
		m.vp.SetYOffset(row)
	} else if row > viewportEnd {
		m.vp.SetYOffset(row - m.vp.Height + 1)
	}
	if m.vp.YOffset < 0 {
		m.vp.YOffset = 0
	}
}

func (m *ResourceModel) applyFilter() {
	query := m.filter.Value()
	if query == "" {
		m.filtered = m.resources
		return
	}

	var filtered []model.K8sResource
	for _, r := range m.resources {
		if caseInsensitiveContains(r.Name, query) ||
			caseInsensitiveContains(r.Status, query) {
			filtered = append(filtered, r)
		}
	}
	m.filtered = filtered
}

func (m *ResourceModel) View() string {
	if m.selectingType {
		return m.viewTypeSelector()
	}

	if m.loading {
		return theme.SpinnerStyle.Render(fmt.Sprintf(" Loading %s in %s...", m.resourceType, m.namespace))
	}

	cw := theme.ContentWidth(m.width)
	m.vp.Width = theme.ViewportWidth(m.width)
	m.vp.Height = m.vpHeight()

	title := theme.TitleStyle.Copy().Width(cw).Render(fmt.Sprintf("☰  %s", m.resourceType))
	nsInfo := theme.SubtitleStyle.Copy().Width(cw).Render(fmt.Sprintf(" Namespace: %s", m.namespace))
	countInfo := theme.ResourceCountStyle.Render(fmt.Sprintf(" %d resources", len(m.filtered)))

	sort.Slice(m.filtered, func(i, j int) bool {
		return m.filtered[i].Name < m.filtered[j].Name
	})

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

	rows := make([]string, len(m.filtered))
	for i, r := range m.filtered {
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
		rows[i] = line
	}

	m.vp.SetContent(strings.Join(rows, "\n"))

	m.filter.SetWidth(cw - 4)
	filterView := m.filter.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		nsInfo,
		countInfo,
		"\n",
		header,
		m.vp.View(),
		filterView,
		"\n",
		theme.HelpStyle.Render(" ↑↓ navigate • Enter detail • / search • r refresh • l logs • t type • q back • ← selector"),
	)
}

func (m *ResourceModel) viewTypeSelector() string {
	cw := theme.ContentWidth(m.width)

	title := theme.TitleStyle.Copy().Width(cw).Render("☰  RESOURCE TYPE SELECTOR")
	nsInfo := theme.SubtitleStyle.Copy().Width(cw).Render(fmt.Sprintf(" Namespace: %s", m.namespace))

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

	var entries []string
	for i, rt := range visibleTypes {
		prefix := "  "
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0"))
		if i == m.typeCursor {
			prefix = "▸ "
			style = lipgloss.NewStyle().
				Foreground(theme.Gold).
				Bold(true).
				Background(theme.Purple).
				Padding(0, 1)
		}
		shortInfo := ""
		if rt.Short != "" {
			shortInfo = lipgloss.NewStyle().Foreground(theme.MutedText).Render(fmt.Sprintf(" (%s)", rt.Short))
		}
		entries = append(entries, style.Render(prefix+rt.Name)+"  "+shortInfo)
	}
	if !showAll {
		remaining := len(displayTypes) - maxVisible
		more := lipgloss.NewStyle().Foreground(theme.MutedText).Italic(true).Render(
			fmt.Sprintf("  … and %d more — press / to search", remaining),
		)
		entries = append(entries, more)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, entries...)

	overhead := 6
	if isSearching {
		overhead = 7
	}
	m.vp.Width = theme.ViewportWidth(m.width)
	m.vp.Height = m.height - overhead
	if m.vp.Height < 3 {
		m.vp.Height = 3
	}
	m.vp.SetContent(content)
	m.scrollTypeCursor()

	m.filter.SetWidth(cw - 4)
	filterView := m.filter.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		nsInfo,
		countInfo,
		filterView,
		"\n",
		m.vp.View(),
		"\n",
		theme.HelpStyle.Render(" ↑↓ navigate • Enter select • / search • ← back to namespaces"),
	)
}

func (m *ResourceModel) scrollTypeCursor() {
	row := m.typeCursor
	vpStart := m.vp.YOffset
	vpEnd := vpStart + m.vp.Height - 1
	if row < vpStart {
		m.vp.SetYOffset(row)
	} else if row > vpEnd {
		m.vp.SetYOffset(row - m.vp.Height + 1)
	}
	if m.vp.YOffset < 0 {
		m.vp.YOffset = 0
	}
}

