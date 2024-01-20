package cmd

import (
	"log"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/topi314/gobin/v2/internal/cfg"
)

func NewEnv(main model) (tea.Model, error) {
	cfgFile, err := cfg.Get()
	if err != nil {
		return nil, err
	}

	h, v := AppStyle.GetFrameSize()
	vp := viewport.New(main.width-h-3, main.height-v-3)
	vp.SetContent(cfgFile)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		PaddingRight(2)

	ta := textarea.New()
	ta.SetValue(cfgFile)
	ta.FocusedStyle.Base = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		PaddingRight(2)

	return envModel{
		main:     main,
		viewport: vp,
		textarea: ta,
		env:      cfgFile,
	}, nil
}

type envModel struct {
	main     model
	viewport viewport.Model
	textarea textarea.Model
	env      string
	edit     bool
}

func (m envModel) Init() tea.Cmd {
	return nil
}

func (m envModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		log.Println(msg.Width, msg.Height)
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height

		m.textarea.SetWidth(msg.Width)
		m.textarea.SetHeight(msg.Height)
	case tea.KeyMsg:
		switch {

		case key.Matches(msg, m.main.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.main.keys.Back):
			return m.main, nil

		case key.Matches(msg, m.main.keys.Edit) && !m.edit:
			m.edit = true

		case key.Matches(msg, m.main.keys.Cancel) && m.edit:
			m.edit = false

		}
	}

	if m.edit {
		ta, cmd := m.textarea.Update(msg)
		m.textarea = ta
		cmds = append(cmds, cmd)
	} else {
		vp, cmd := m.viewport.Update(msg)
		m.viewport = vp
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m envModel) View() string {
	if m.edit {
		AppStyle.Render(m.textarea.View(), m.editHelpView())
	}

	return AppStyle.Render(m.viewport.View(), m.viewHelpView())
}

func (m envModel) viewHelpView() string {
	return HelpStyle.Render("\n  ↑/↓: Navigate • q: Quit • e: Edit • b: Back\n")
}

func (m envModel) editHelpView() string {
	return HelpStyle.Render("\n  ↑/↓: Navigate • esc: Cancel • Ctrl+S: Save • Ctrl+C: Quit\n")
}
