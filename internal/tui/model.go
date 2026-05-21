package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/danilbrenner/sshelob/internal/config"
)

type Model struct {
	cfg *config.Config
}

func NewModel(cfg *config.Config) *Model {
	return &Model{cfg: cfg}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) View() string {
	return "sshelob\n\nPress any key to quit.\n"
}

func (m *Model) Run() error {
	program := tea.NewProgram(*m)
	_, err := program.Run()
	return err
}
