package cmd

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/topi314/gobin/v2/internal/cfg"
)

func renderEnv(env map[string]string) string {
	var s string
	for k, v := range env {
		s += k + ": " + v + "\n"
	}
	return s
}

func NewEnv() (tea.Model, error) {
	entries, err := cfg.Get()
	if err != nil {
		return nil, err
	}

	vp := viewport.New(60, 20)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		PaddingRight(2)

	vp.SetContent(renderEnv(entries))

	return envModel{
		viewport: vp,
		env:      entries,
	}, nil
}

type envModel struct {
	viewport viewport.Model
	env      map[string]string
	edit     bool
}

func (m envModel) Init() tea.Cmd {
	return nil
}

func (m envModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {

		case "ctrl+c", "q":
			return m, tea.Quit

		}
	}

	vp, cmd := m.viewport.Update(msg)
	m.viewport = vp
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m envModel) View() string {
	return m.viewport.View() + m.helpView()
}

func (m envModel) helpView() string {
	return HelpStyle.Render("\n  ↑/↓: Navigate • q: Quit • Enter: Edit • b: Back\n")
}
