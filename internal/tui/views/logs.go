package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/dave/kube-tui/internal/model"
	"github.com/dave/kube-tui/internal/tui/theme"
	"github.com/dave/kube-tui/internal/tui/components"
)

type LogModel struct {
	resource    model.K8sResource
	namespace   string
	clientset   *kubernetes.Clientset
	logContent  string
	follow      bool
	statusBar   *components.StatusBar
	cancelFunc  context.CancelFunc
	containers  []string
	selContainer int
	selectingContainer bool
	width       int
	height      int
}

func NewLogModel() *LogModel {
	return &LogModel{}
}

func (m *LogModel) SetClientset(cs *kubernetes.Clientset) {
	m.clientset = cs
}

func (m *LogModel) SetStatusBar(sb *components.StatusBar) {
	m.statusBar = sb
}

func (m *LogModel) SetResource(r model.K8sResource) {
	m.resource = r
	m.namespace = r.Namespace
}

func (m *LogModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *LogModel) Init() tea.Cmd {
	m.selectingContainer = true
	return func() tea.Msg {
		pod, err := m.clientset.CoreV1().Pods(m.namespace).Get(context.TODO(), m.resource.Name, metav1.GetOptions{})
		if err != nil {
			return errMsg{err: fmt.Errorf("get pod: %w", err)}
		}
		var containers []string
		for _, c := range pod.Spec.Containers {
			containers = append(containers, c.Name)
		}
		if len(containers) == 0 {
			containers = []string{m.resource.Name}
		}
		return containersLoadedMsg{containers: containers}
	}
}

type containersLoadedMsg struct {
	containers []string
}

type ShowLogMsg struct {
	Resource model.K8sResource
}

type logLineMsg struct {
	line string
}

type logDoneMsg struct{}

func (m *LogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case containersLoadedMsg:
		m.containers = msg.containers
		m.selContainer = 0
		return m, nil

	case logLineMsg:
		m.logContent += msg.line + "\n"
		return m, nil

	case logDoneMsg:
		m.follow = false
		if m.statusBar != nil {
			m.statusBar.SetMode("logs (done)")
		}
		return m, nil

	case errMsg:
		m.logContent += fmt.Sprintf("Error: %v\n", msg.err)
		if m.statusBar != nil {
			m.statusBar.SetError(msg.err.Error())
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.selectingContainer {
				m.selectingContainer = false
				container := m.containers[m.selContainer]
				if m.statusBar != nil {
					m.statusBar.SetMode(fmt.Sprintf("logs: %s", container))
				}
				return m, m.startLogStream(container)
			}
			return m, nil

		case "up", "k":
			if m.selectingContainer && m.selContainer > 0 {
				m.selContainer--
			}

		case "down", "j":
			if m.selectingContainer && m.selContainer < len(m.containers)-1 {
				m.selContainer++
			}

		case "f":
			if !m.selectingContainer {
				m.follow = !m.follow
				if m.follow {
					if m.statusBar != nil {
						m.statusBar.SetMode("logs (following)")
					}
				} else {
					if m.statusBar != nil {
						m.statusBar.SetMode("logs (paused)")
					}
				}
			}

		case "backspace", "esc", "q":
			if m.cancelFunc != nil {
				m.cancelFunc()
			}
			return m, popViewCmd()

		case "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *LogModel) startLogStream(container string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.TODO())
		m.cancelFunc = cancel

		podLogOpts := &corev1.PodLogOptions{
			Container: container,
			Follow:    true,
			TailLines: int64Ptr(100),
		}

		req := m.clientset.CoreV1().Pods(m.namespace).GetLogs(m.resource.Name, podLogOpts)
		stream, err := req.Stream(ctx)
		if err != nil {
			return errMsg{err: fmt.Errorf("log stream: %w", err)}
		}
		defer stream.Close()

		buf := make([]byte, 4096)
		for {
			select {
			case <-ctx.Done():
				return logDoneMsg{}
			default:
				n, err := stream.Read(buf)
				if err != nil {
					if err == io.EOF {
						return logDoneMsg{}
					}
					return errMsg{err: fmt.Errorf("log read: %w", err)}
				}
				if n > 0 {
					return logLineMsg{line: string(buf[:n])}
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

func (m *LogModel) View() string {
	if m.selectingContainer {
		return m.viewContainerSelector()
	}

	cw := theme.ContentWidth(m.width)
	title := theme.TitleStyle.Copy().Width(cw).Render(fmt.Sprintf("📋  Logs: %s", m.resource.Name))
	nsInfo := theme.SubtitleStyle.Copy().Width(cw).Render(fmt.Sprintf(" %s / %s", m.namespace, m.resource.Name))

	logStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E0E0E0")).
		Padding(0, 2)

	lines := strings.Split(m.logContent, "\n")
	visibleLines := lines
	if len(visibleLines) > 200 {
		visibleLines = visibleLines[len(visibleLines)-200:]
	}
	logText := logStyle.Render(strings.Join(visibleLines, "\n"))

	followIndicator := ""
	if m.follow {
		followIndicator = lipgloss.NewStyle().Foreground(theme.Lime).Render(" ● FOLLOWING")
	} else {
		followIndicator = lipgloss.NewStyle().Foreground(theme.MutedText).Render(" ○ paused")
	}

	help := theme.HelpStyle.Render(" f toggle follow • ← back")

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		nsInfo+followIndicator,
		"\n",
		logText,
		"\n",
		help,
	)
}

func (m *LogModel) viewContainerSelector() string {
	cw := theme.ContentWidth(m.width)
	title := theme.TitleStyle.Copy().Width(cw).Render("📋  LOGS - CONTAINER SELECTOR")
	prompt := lipgloss.NewStyle().Foreground(theme.Orange).Bold(true).
		Render("Select a container:")

	var entries []string
	for i, c := range m.containers {
		prefix := "  "
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0"))
		if i == m.selContainer {
			prefix = "▸ "
			style = lipgloss.NewStyle().
				Foreground(theme.Gold).
				Bold(true).
				Background(theme.Purple).
				Padding(0, 1)
		}
		entries = append(entries, style.Render(prefix+c))
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"\n",
		prompt,
		"\n",
		lipgloss.JoinVertical(lipgloss.Left, entries...),
		"\n",
		theme.HelpStyle.Render(" ↑↓ navigate • Enter select"),
	)
}

func int64Ptr(i int64) *int64 {
	return &i
}
