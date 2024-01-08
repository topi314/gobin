package main

import (
	"log"
	"os"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbletea"
	"github.com/topi314/gobin/v2/tui/cmd"
)

type choice struct {
	title       string
	description string
	newModel    func() (tea.Model, error)
}

func (i choice) Title() string       { return i.title }
func (i choice) Description() string { return i.description }
func (i choice) FilterValue() string { return i.title }

func initialModel() model {
	list := list.New([]list.Item{
		choice{
			title:       "env",
			description: "Let's you edit the environment variables for gobin.",
			newModel:    cmd.NewEnv,
		},
		choice{
			title:       "get",
			description: "Downloads a document from a gobin server.",
			newModel:    cmd.NewGet,
		},
		choice{
			title:       "import",
			description: "Imports a share url into gobin.",
			newModel:    cmd.NewImport,
		},
		choice{
			title:       "post",
			description: "Posts a document to a gobin server.",
			newModel:    cmd.NewPost,
		},
		choice{
			title:       "rm",
			description: "Removes a document from a gobin server.",
			newModel:    cmd.NewRm,
		},
		choice{
			title:       "share",
			description: "Shares a document from a gobin server.",
			newModel:    cmd.NewShare,
		},
	}, list.NewDefaultDelegate(), 0, 0)
	list.Styles.HelpStyle = cmd.HelpStyle
	return model{
		list: list,
	}
}

type model struct {
	list list.Model
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := cmd.AppStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		switch msg.String() {

		case "ctrl+c", "q":
			return m, tea.Quit

		case "enter":
			newModel, err := m.list.SelectedItem().(choice).newModel()
			if err != nil {
				log.Println(err)
				return m, tea.Quit
			}
			return newModel, nil
		}
	}

	newListModel, tCmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, tCmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	return cmd.AppStyle.Render(m.list.View())
}

func main() {
	os.Setenv("DEBUG", "1")
	if len(os.Getenv("DEBUG")) > 0 {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			log.Println("fatal:", err)
			os.Exit(1)
		}
		defer f.Close()
	}

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
