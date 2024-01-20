package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
)

func NewRm(main model) (tea.Model, error) {
	return rmModel{}, nil
}

type rmModel struct {
}

func (m rmModel) Init() tea.Cmd {
	return nil
}

func (m rmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m rmModel) View() string {
	return ""
}
