package components

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dave/kube-tui/internal/tui/theme"
)

var (
	tableBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(theme.Purple).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#0D0D1A")).
			Foreground(theme.Gold).
			Bold(true).
			Padding(0, 1)

	cursorStyle = lipgloss.NewStyle().
			Foreground(theme.HotPink).
			SetString("▸ ")
)

type Column struct {
	Title string
	Width int
}

type Row []string

type Table struct {
	columns  []Column
	rows     []Row
	cursor   int
	offset   int
	viewport viewport.Model
	width    int
	height   int
	title    string
}

func NewTable(title string, columns []Column) *Table {
	vp := viewport.New(80, 20)
	return &Table{
		columns:  columns,
		rows:     []Row{},
		cursor:   0,
		offset:   0,
		viewport: vp,
		title:    title,
	}
}

func (t *Table) SetRows(rows []Row) {
	t.rows = rows
	if t.cursor >= len(rows) {
		t.cursor = len(rows) - 1
		if t.cursor < 0 {
			t.cursor = 0
		}
	}
}

func (t *Table) SetSize(w, h int) {
	t.width = w
	t.height = h
	t.viewport.Width = w - 2
	t.viewport.Height = h - 4
}

func (t *Table) SelectedRow() int {
	return t.cursor
}

func (t *Table) SelectedItem() Row {
	if t.cursor >= 0 && t.cursor < len(t.rows) {
		return t.rows[t.cursor]
	}
	return nil
}

func (t *Table) Rows() []Row {
	return t.rows
}

func (t *Table) MoveUp() {
	if t.cursor > 0 {
		t.cursor--
	}
}

func (t *Table) MoveDown() {
	if t.cursor < len(t.rows)-1 {
		t.cursor++
	}
}

func (t *Table) MoveTop() {
	t.cursor = 0
}

func (t *Table) MoveBottom() {
	t.cursor = len(t.rows) - 1
}

func (t *Table) Update(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			t.MoveUp()
		case "down", "j":
			t.MoveDown()
		case "home", "g":
			t.MoveTop()
		case "end", "G":
			t.MoveBottom()
		}
	}
}

func (t *Table) View() string {
	if len(t.rows) == 0 {
		return lipgloss.NewStyle().
			Foreground(theme.MutedText).
			Italic(true).
			Render("  No resources found")
	}

	headerLipgloss := lipgloss.NewStyle().
		Background(lipgloss.Color("#0D0D1A")).
		Foreground(theme.Gold).
		Bold(true)

	var headerCells []string
	for _, col := range t.columns {
		headerCells = append(headerCells, lipgloss.NewStyle().Width(col.Width).Render(col.Title))
	}
	header := headerLipgloss.Render(lipgloss.JoinHorizontal(lipgloss.Top, headerCells...))

	startIdx := t.offset
	visibleCount := t.height - 6
	if visibleCount < 1 {
		visibleCount = 1
	}

	endIdx := startIdx + visibleCount
	if endIdx > len(t.rows) {
		endIdx = len(t.rows)
	}

	var rows []string
	for i := startIdx; i < endIdx; i++ {
		row := t.rows[i]
		var cells []string
		for j, cell := range row {
			col := t.columns[j]
			rendered := theme.Truncate(cell, col.Width)
			if i == t.cursor {
				rendered = theme.SelectedRowStyle.Width(col.Width).Render(rendered)
			} else {
				rendered = theme.NormalRowStyle.Width(col.Width).Render(rendered)
			}
			cells = append(cells, rendered)
		}

		line := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
		if i == t.cursor {
			line = cursorStyle.Render() + line
		} else {
			line = "  " + line
		}
		rows = append(rows, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)

	return lipgloss.JoinVertical(lipgloss.Left, header, content)
}

func (t *Table) Len() int {
	return len(t.rows)
}
