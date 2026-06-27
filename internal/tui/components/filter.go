package components

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	filterPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#39FF14")).
				Bold(true)

	filterInputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF"))

	filterPlaceholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#555566"))
)

type Filter struct {
	input    textinput.Model
	onChange func(string)
	active   bool
}

func NewFilter(placeholder string, onChange func(string)) *Filter {
	input := textinput.New()
	input.Placeholder = placeholder
	input.Prompt = "🔍 "
	input.PromptStyle = filterPromptStyle
	input.TextStyle = filterInputStyle
	input.PlaceholderStyle = filterPlaceholderStyle
	input.CharLimit = 100
	input.Width = 40

	return &Filter{
		input:    input,
		onChange: onChange,
		active:   false,
	}
}

func (f *Filter) Focus() tea.Cmd {
	f.active = true
	return f.input.Focus()
}

func (f *Filter) Blur() {
	f.active = false
	f.input.Blur()
}

func (f *Filter) SetWidth(w int) {
	if w < 20 {
		w = 20
	}
	f.input.Width = w
}

func (f *Filter) Value() string {
	return f.input.Value()
}

func (f *Filter) SetValue(v string) {
	f.input.SetValue(v)
}

func (f *Filter) Update(msg tea.Msg) tea.Cmd {
	if !f.active {
		return nil
	}

	var cmd tea.Cmd
	f.input, cmd = f.input.Update(msg)

	if f.onChange != nil {
		f.onChange(f.input.Value())
	}

	return cmd
}

func (f *Filter) View() string {
	if !f.active {
		return ""
	}
	return f.input.View()
}

func (f *Filter) Active() bool {
	return f.active
}
