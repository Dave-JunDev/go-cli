package views

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
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
	logContent  []string
	follow      bool
	statusBar   *components.StatusBar
	cancelFunc  context.CancelFunc
	containers  []string
	selContainer int
	selectingContainer bool
	logOff      int
	width       int
	height      int
	reader      *bufio.Reader
	container   string
	search      *components.Filter
	searches    []int
	searchIdx   int
	saveInput   textinput.Model
	saveMode    bool
}

func NewLogModel() *LogModel {
	si := textinput.New()
	si.Placeholder = "~/k8s-ui/pod.log"
	si.Prompt = "💾 "
	si.Width = 60
	si.CharLimit = 200
	return &LogModel{
		search:    components.NewFilter("Search logs...", nil),
		saveInput: si,
	}
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
	sw := theme.ContentWidth(w)
	if sw < 40 {
		sw = 40
	}
	m.search.SetWidth(sw)
	m.saveInput.Width = sw - 10
}

func (m *LogModel) Init() tea.Cmd {
	m.selectingContainer = true
	m.logContent = nil
	m.logOff = 0
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

type ShowDescribeMsg struct {
	Describe string
	Resource model.K8sResource
}

type logLineMsg struct {
	line string
}

type logDoneMsg struct{}

func (m *LogModel) visibleLogLines() int {
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
	// View: header (2) + \n + content + \n + help = content + 5
	// app.go: content + \n + statusbar = content + 7 total
	// total = height => content = height - 7 - extra
	h := m.height - 7 - extra
	if h < 3 {
		h = 3
	}
	return h
}

func (m *LogModel) wrapW() int {
	w := theme.ContentWidth(m.width) - 4
	if w < 20 {
		w = 20
	}
	return w
}

func (m *LogModel) visualLinesFor(idx int) int {
	return len(wrapLine(m.logContent[idx], m.wrapW()))
}

func (m *LogModel) scrollToBottom() {
	if len(m.logContent) == 0 {
		m.logOff = 0
		return
	}
	visible := m.visibleLogLines()
	wrapW := m.wrapW()
	off := len(m.logContent) - 1
	count := 0
	for off >= 0 {
		wrapped := len(wrapLine(m.logContent[off], wrapW))
		if count+wrapped > visible {
			off++
			break
		}
		count += wrapped
		off--
	}
	if off < 0 {
		off = 0
	}
	if off >= len(m.logContent) {
		off = len(m.logContent) - 1
	}
	m.logOff = off
}

func (m *LogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		sw := theme.ContentWidth(m.width)
		if sw < 40 {
			sw = 40
		}
		m.search.SetWidth(sw)
		m.saveInput.Width = sw - 10

	case containersLoadedMsg:
		m.containers = msg.containers
		m.selContainer = 0
		return m, nil

	case logLineMsg:
		m.logContent = append(m.logContent, msg.line)
		if m.follow {
			m.scrollToBottom()
		}
		return m, m.readNextLogLine()

	case logDoneMsg:
		m.follow = false
		if m.statusBar != nil {
			m.statusBar.SetMode("logs (done)")
		}
		return m, nil

	case errMsg:
		m.logContent = append(m.logContent, fmt.Sprintf("Error: %v", msg.err))
		if m.follow {
			m.scrollToBottom()
		}
		if m.statusBar != nil {
			m.statusBar.SetError(msg.err.Error())
		}
		return m, nil

	case tea.KeyMsg:
		if m.selectingContainer {
			return m.handleContainerKey(msg)
		}
		if m.saveMode {
			return m.handleLogSaveKey(msg)
		}
		if m.search.Active() {
			return m.handleSearchKey(msg)
		}
		return m.handleLogKey(msg)
	}

	return m, nil
}

func (m *LogModel) handleContainerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.selectingContainer = false
		container := m.containers[m.selContainer]
		if m.statusBar != nil {
			m.statusBar.SetMode(fmt.Sprintf("logs: %s", container))
		}
		m.logContent = nil
		m.logOff = 0
		m.follow = true
		return m, m.startLogStream(container)

	case "up", "k":
		if m.selContainer > 0 {
			m.selContainer--
		}

	case "down", "j":
		if m.selContainer < len(m.containers)-1 {
			m.selContainer++
		}

	case "backspace", "esc", "q":
		return m, popViewCmd()

	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m *LogModel) visibleBottomBound() int {
	if len(m.logContent) == 0 {
		return 0
	}
	visible := m.visibleLogLines()
	wrapW := m.wrapW()
	count := 0
	off := len(m.logContent) - 1
	for off >= 0 {
		wrapped := len(wrapLine(m.logContent[off], wrapW))
		if count+wrapped > visible {
			off++
			break
		}
		count += wrapped
		off--
	}
	if off < 0 {
		off = 0
	}
	if off >= len(m.logContent) {
		off = len(m.logContent) - 1
	}
	return off
}

func (m *LogModel) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m *LogModel) runSearch() {
	query := strings.ToLower(m.search.Value())
	m.searches = nil
	m.searchIdx = 0

	if query == "" {
		return
	}

	for i, line := range m.logContent {
		if strings.Contains(strings.ToLower(line), query) {
			m.searches = append(m.searches, i)
		}
	}
}

func (m *LogModel) scrollToMatch(idx int) {
	if idx < 0 || idx >= len(m.searches) {
		return
	}
	lineNum := m.searches[idx]
	visible := m.visibleLogLines()
	m.logOff = lineNum - visible/2
	if m.logOff < 0 {
		m.logOff = 0
	}
}

func (m *LogModel) handleLogSaveKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		content := strings.Join(m.logContent, "\n") + "\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
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

func (m *LogModel) defaultLogSavePath() string {
	home, _ := os.UserHomeDir()
	ts := time.Now().Format("20060102_150405")
	name := fmt.Sprintf("logs-%s-%s.log", m.namespace, m.resource.Name)
	return filepath.Join(home, "k8s-ui", ts+"-"+name)
}

func (m *LogModel) handleLogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.logOff > 0 {
			m.logOff--
			m.follow = false
		}

	case "down", "j":
		if m.logOff < m.visibleBottomBound() {
			m.logOff++
		}

	case "pgup":
		visible := m.visibleLogLines()
		m.logOff -= visible
		if m.logOff < 0 {
			m.logOff = 0
		}
		m.follow = false

	case "pgdown":
		visible := m.visibleLogLines()
		m.logOff += visible
		if m.logOff > m.visibleBottomBound() {
			m.logOff = m.visibleBottomBound()
		}

	case "g":
		m.logOff = 0
		m.follow = false

	case "G":
		m.scrollToBottom()

	case "f":
		m.follow = !m.follow
		if m.follow {
			m.scrollToBottom()
			if m.statusBar != nil {
				m.statusBar.SetMode("logs (following)")
			}
		} else {
			if m.statusBar != nil {
				m.statusBar.SetMode("logs (paused)")
			}
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
		m.saveInput.SetValue(m.defaultLogSavePath())
		return m, m.saveInput.Focus()

	case "backspace", "esc", "q":
		if m.cancelFunc != nil {
			m.cancelFunc()
			m.cancelFunc = nil
		}
		m.reader = nil
		return m, popViewCmd()

	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m *LogModel) startLogStream(container string) tea.Cmd {
	m.container = container
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
		m.cancelFunc = nil
		cancel()
		return func() tea.Msg {
			return errMsg{err: fmt.Errorf("log stream: %w", err)}
		}
	}
	m.reader = bufio.NewReader(stream)
	return m.readNextLogLine()
}

func (m *LogModel) readNextLogLine() tea.Cmd {
	return func() tea.Msg {
		if m.reader == nil {
			return logDoneMsg{}
		}
		line, err := m.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if line != "" {
					return logLineMsg{line: strings.TrimRight(line, "\r\n")}
				}
				return logDoneMsg{}
			}
			return errMsg{err: fmt.Errorf("log read: %w", err)}
		}
		return logLineMsg{line: strings.TrimRight(line, "\r\n")}
	}
}

func (m *LogModel) View() string {
	if m.selectingContainer {
		return m.viewContainerSelector()
	}

	cw := theme.ContentWidth(m.width)
	title := theme.TitleStyle.Copy().Width(cw).Render(fmt.Sprintf("📋  Logs: %s", m.resource.Name))
	nsInfo := theme.SubtitleStyle.Copy().Width(cw).Render(fmt.Sprintf(" %s / %s", m.namespace, m.resource.Name))

	followIndicator := ""
	if m.follow {
		followIndicator = lipgloss.NewStyle().Foreground(theme.Lime).Render(" ● FOLLOWING")
	} else {
		followIndicator = lipgloss.NewStyle().Foreground(theme.MutedText).Render(" ○ paused")
	}

	query := strings.ToLower(m.search.Value())
	hasSearch := query != "" && len(m.searches) > 0

	visible := m.visibleLogLines()
	wrapW := m.wrapW()
	logStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0")).Padding(0, 2)
	matchStyle := lipgloss.NewStyle().Foreground(theme.Orange).Bold(true).Padding(0, 2)
	currentMatchStyle := lipgloss.NewStyle().Foreground(theme.Gold).Background(theme.Purple).Bold(true).Padding(0, 1)
	var displayLines []string
	idx := m.logOff
	for idx < len(m.logContent) && len(displayLines) < visible {
		currentLine := m.logContent[idx]
		isMatch := hasSearch && m.searchIdx < len(m.searches) && m.searches[m.searchIdx] == idx
		isSearchResult := false
		for _, s := range m.searches {
			if s == idx {
				isSearchResult = true
				break
			}
		}
		for _, wrapped := range wrapLine(currentLine, wrapW) {
			if len(displayLines) >= visible {
				break
			}
			if isMatch {
				displayLines = append(displayLines, currentMatchStyle.Render(wrapped))
			} else if isSearchResult {
				displayLines = append(displayLines, matchStyle.Render(wrapped))
			} else {
				displayLines = append(displayLines, logStyle.Render(wrapped))
			}
		}
		idx++
	}
	logText := strings.Join(displayLines, "\n")

	filterView := m.search.View()

	var searchInfo string
	if hasSearch {
		searchInfo = theme.ResourceCountStyle.Render(fmt.Sprintf(" %d/%d matches", m.searchIdx+1, len(m.searches)))
	}

	help := theme.HelpStyle.Render(" \u2191\u2193 scroll \u2022 f follow \u2022 s save \u2022 / search \u2022 n/N next \u2022 g top \u2022 G bottom \u2022 q back")

	out := fmt.Sprintf("%s%s%s\n%s", title, nsInfo, followIndicator, logText)
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

func (m *LogModel) viewContainerSelector() string {
	cw := theme.ContentWidth(m.width)
	title := theme.TitleStyle.Copy().Width(cw).Render("📋  CONTAINER SELECTOR")
	nsInfo := theme.SubtitleStyle.Copy().Width(cw).Render(fmt.Sprintf(" %s / %s", m.namespace, m.resource.Name))
	countInfo := theme.ResourceCountStyle.Render(fmt.Sprintf(" %d containers", len(m.containers)))

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

	content := lipgloss.JoinVertical(lipgloss.Left, entries...)

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		nsInfo,
		countInfo,
		"\n",
		content,
		"\n",
		theme.HelpStyle.Render(" ↑↓ navigate • Enter select"),
	)
}

func int64Ptr(i int64) *int64 {
	return &i
}

func wrapLine(s string, w int) []string {
	if w < 1 {
		w = 1
	}
	runes := []rune(s)
	if len(runes) <= w {
		return []string{s}
	}
	var chunks []string
	for i := 0; i < len(runes); i += w {
		end := i + w
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	if len(chunks) == 0 {
		return []string{s}
	}
	return chunks
}
