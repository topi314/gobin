package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
)

func NewImport(main model) (tea.Model, error) {
	return importModel{}, nil
}

type importModel struct {
}

func (m importModel) Init() tea.Cmd {
	return nil
}

func (m importModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m importModel) View() string {
	return ""
}
