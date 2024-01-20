package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
)

func NewPost(main model) (tea.Model, error) {
	return postModel{}, nil
}

type postModel struct {
}

func (m postModel) Init() tea.Cmd {
	return nil
}

func (m postModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m postModel) View() string {
	return ""
}
