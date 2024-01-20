package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
)

func NewShare(main model) (tea.Model, error) {
	return shareModel{}, nil
}

type shareModel struct {
}

func (m shareModel) Init() tea.Cmd {
	return nil
}

func (m shareModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m shareModel) View() string {
	return ""
}
