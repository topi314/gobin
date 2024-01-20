package cmd

import (
	"log"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type choice struct {
	title       string
	description string
	newModel    func(main model) (tea.Model, error)
}

func (i choice) Title() string       { return i.title }
func (i choice) Description() string { return i.description }
func (i choice) FilterValue() string { return i.title }

func NewMain() tea.Model {
	l := list.New([]list.Item{
		choice{
			title:       "env",
			description: "Let's you edit the environment variables for gobin.",
			newModel:    NewEnv,
		},
		choice{
			title:       "get",
			description: "Downloads a document from a gobin server.",
			newModel:    NewGet,
		},
		choice{
			title:       "import",
			description: "Imports a share url into gobin.",
			newModel:    NewImport,
		},
		choice{
			title:       "post",
			description: "Posts a document to a gobin server.",
			newModel:    NewPost,
		},
		choice{
			title:       "rm",
			description: "Removes a document from a gobin server.",
			newModel:    NewRm,
		},
		choice{
			title:       "share",
			description: "Shares a document from a gobin server.",
			newModel:    NewShare,
		},
	}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "gobin"
	return model{
		keys: defaultKeyMap,
		list: l,
	}
}

type model struct {
	keys   keyMap
	width  int
	height int
	list   list.Model
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		h, v := AppStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		switch msg.String() {

		case "ctrl+c", "q":
			return m, tea.Quit

		case "enter":
			newModel, err := m.list.SelectedItem().(choice).newModel(m)
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
	return AppStyle.Render(m.list.View())
}
