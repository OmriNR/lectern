package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type ezItem struct {
	title, desc string
	action      func() error
}

func (i ezItem) Title() string       { return i.title }
func (i ezItem) Description() string { return i.desc }
func (i ezItem) FilterValue() string { return i.title }

type ezModel struct {
	list   list.Model
	chosen *ezItem
}

func (m ezModel) Init() tea.Cmd { return nil }

func (m ezModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "enter":
			if it, ok := m.list.SelectedItem().(ezItem); ok {
				m.chosen = &it
				return m, tea.Quit
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m ezModel) View() string { return m.list.View() }

func runEZ(c *CLI) error {
	for {
		items := []list.Item{
			ezItem{"Connect", "Log in to your Moodle site", func() error {
				c.handleConnect()
				return nil
			}},
			ezItem{"Clone", "Download all your courses", func() error {
				if !c.requireConnected() {
					return nil
				}
				c.handleClone(nil)
				return nil
			}},
			ezItem{"Status", "Check your local workspace", func() error {
				if !c.requireConnected() {
					return nil
				}
				c.handleStatus()
				return nil
			}},
			ezItem{"Quit", "Exit the menu", nil},
		}

		l := list.New(items, list.NewDefaultDelegate(), 40, 16)
		l.Title = "lectern - what would you like to do?"

		m, err := tea.NewProgram(ezModel{list: l}).Run()

		if err != nil {
			return err
		}

		chosen := m.(ezModel).chosen

		if chosen == nil || chosen.action == nil {
			return nil
		}

		if err := chosen.action(); err != nil {
			fmt.Println("ERROR:", err)
		}

		fmt.Println()
		fmt.Print("Press Enter to return to the menu...")
		bufio.NewReader(os.Stdin).ReadString('\n')
	}
}
