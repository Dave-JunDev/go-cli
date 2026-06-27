package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dave/kube-tui/internal/k8s"
	"github.com/dave/kube-tui/internal/model"
	"github.com/dave/kube-tui/internal/tui/components"
	"github.com/dave/kube-tui/internal/tui/theme"
)

type DetailModel struct {
	resource    model.K8sResource
	namespace   string
	yamlContent string
	loading     bool
	err         error
	resourceMgr *k8s.ResourceManager
	statusBar   *components.StatusBar
	showYAML    bool
	width       int
	height      int
	viewport    viewport.Model
}

func NewDetailModel() *DetailModel {
	vp := viewport.New(80, 20)
	return &DetailModel{viewport: vp}
}

func (m *DetailModel) SetResourceManager(rm *k8s.ResourceManager) {
	m.resourceMgr = rm
}

func (m *DetailModel) SetStatusBar(sb *components.StatusBar) {
	m.statusBar = sb
}

func (m *DetailModel) SetResource(r model.K8sResource) {
	m.resource = r
	m.namespace = r.Namespace
	m.showYAML = false
}

func (m *DetailModel) SetWidth(w int) {
	m.width = w
}

func (m *DetailModel) SetHeight(h int) {
	m.height = h
}

func (m *DetailModel) Init() tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		yaml, err := m.resourceMgr.GetResourceYAML(m.namespace, m.resource.Type, m.resource.Name)
		if err != nil {
			yaml = fmt.Sprintf("Unable to fetch YAML: %v", err)
		}
		return yamlLoadedMsg{yaml: yaml}
	}
}

type yamlLoadedMsg struct {
	yaml string
}

type ShowYamlMsg struct {
	YAML     string
	Resource model.K8sResource
}

func (m *DetailModel) updateViewportSize() {
	m.viewport.Width = theme.ViewportWidth(m.width)
	m.viewport.Height = theme.ViewportHeight(m.height, 14)
}

func (m *DetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case yamlLoadedMsg:
		m.yamlContent = msg.yaml
		m.loading = false
		if m.statusBar != nil {
			m.statusBar.SetResource(m.resource.Name)
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateViewportSize()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "y":
			if m.yamlContent != "" {
				return m, func() tea.Msg {
					return ShowYamlMsg{
						YAML:     m.yamlContent,
						Resource: m.resource,
					}
				}
			}
		case "l":
			return m, func() tea.Msg {
				return ShowLogMsg{Resource: m.resource}
			}
		case "d":
			return m, func() tea.Msg {
				desc, err := m.resourceMgr.GetResourceDescribe(m.namespace, m.resource.Type, m.resource.Name)
				if err != nil {
					desc = fmt.Sprintf("Unable to describe: %v", err)
				}
				return ShowDescribeMsg{Describe: desc, Resource: m.resource}
			}
		case "backspace", "esc", "q":
			return m, popViewCmd()
		case "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *DetailModel) View() string {
	if m.loading {
		return theme.SpinnerStyle.Render(fmt.Sprintf(" Loading detail for %s...", m.resource.Name))
	}

	cw := theme.ContentWidth(m.width)

	title := theme.TitleStyle.Copy().Width(cw).Render(fmt.Sprintf("ℹ  %s", m.resource.Name))
	subtitle := theme.SubtitleStyle.Copy().Width(cw).Render(fmt.Sprintf(" %s / %s", m.resource.Type, m.namespace))

	contentWidth := cw - 2

	labelStyle := lipgloss.NewStyle().Foreground(theme.ElectricBlue).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0"))

	var sections []string

	addField := func(label, value string) {
		sections = append(sections, labelStyle.Render(label)+"  "+valueStyle.Render(value))
	}

	addField("Name", m.resource.Name)
	addField("Type", m.resource.Type)
	addField("Namespace", m.resource.Namespace)
	addField("Status", theme.ColoredStatus(m.resource.Status))
	addField("Age", m.resource.Age)

	if len(m.resource.Labels) > 0 {
		var labelLines []string
		labelLines = append(labelLines, labelStyle.Render("Labels:"))
		for k, v := range m.resource.Labels {
			line := fmt.Sprintf("  %s=%s", k, v)
			if len(line) > contentWidth-2 {
				line = line[:contentWidth-5] + "…"
			}
			labelLines = append(labelLines, valueStyle.Render(line))
		}
		sections = append(sections, strings.Join(labelLines, "\n"))
	}

	info := lipgloss.NewStyle().Width(contentWidth).Padding(0, 2).Foreground(lipgloss.Color("#E0E0E0")).Render(strings.Join(sections, "\n"))

	help := theme.HelpStyle.Render(" y YAML • d describe • l logs • ← back")

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		subtitle,
		"\n",
		info,
		"\n\n",
		help,
	)
}

func (m *DetailModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.updateViewportSize()
}
