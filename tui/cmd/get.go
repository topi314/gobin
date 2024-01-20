package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
)

func NewGet(main model) (tea.Model, error) {
	return getModel{}, nil
}

type getModel struct {
	main model
}

func (m getModel) Init() tea.Cmd {
	return nil
}

func (m getModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m getModel) View() string {
	return ""
}
