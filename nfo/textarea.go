package nfo

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

type textAreaModel struct {
	desc     string
	textarea textarea.Model
	err      error
	value    *string
	save     *bool
}

// TextEdit creates a new text area input field.
type textAreaValue struct {
	desc   string
	value  *string
	prompt string
	height int
	width  int
}

// TextArea creates and registers a new text area input field.
// It returns a pointer to the underlying string value, which will be populated
// when the user edits and saves the text area.
func (O *Options) TextArea(desc string, prompt string) *string {
	var value string
	new_var := &textAreaValue{
		desc:   desc,
		value:  &value,
		prompt: prompt,
	}
	O.Register(new_var)
	return &value
}

// TextAreaVar registers a new text area input field using a pre-existing string pointer.
func (O *Options) TextAreaVar(p *string, desc string, prompt string) {
	O.Register(&textAreaValue{
		desc:   desc,
		value:  p,
		prompt: prompt,
	})
	return
}

// Initializes the text area model, returning a command to start blinking the cursor.
func (m textAreaModel) Init() tea.Cmd {
	return textarea.Blink
}

// Presents a text area input to the user and returns true if the user saves the input, false otherwise.
func (t *textAreaValue) Set() bool {
	ti := textarea.New()
	ti.Placeholder = t.prompt
	ti.SetWidth(termWidth())
	ti.SetHeight(20)
	ti.Focus()
	ti.Value()

	model := textAreaModel{
		desc:     t.desc,
		textarea: ti,
		err:      nil,
		value:    t.value,
		save:     new(bool),
	}

	p := tea.NewProgram(model)

	if _, err := p.Run(); err != nil {
		Err(err)
	}

	if *model.save {
		t.value = model.value
		return true
	}

	return false
}

type errMsg error

// Updates the text area model based on the given message.
func (m textAreaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			if m.textarea.Focused() {
				m.textarea.Blur()
			}
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyCtrlS:
			*m.value = m.textarea.Value()
			*m.save = true
			return m, tea.Quit
		default:
			if !m.textarea.Focused() {
				cmd = m.textarea.Focus()
				cmds = append(cmds, cmd)
			}
		}

	// We handle errors just like any other message
	case errMsg:
		m.err = msg
		return m, nil
	}

	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// Returns the string representation of the text area model for rendering.
func (m textAreaModel) View() string {
	return fmt.Sprintf(
		"%s\n\n%s\n\n%s", m.desc,
		m.textarea.View(),
		"(ctrl-s to save, ctrl-c to abort)",
	) + "\n\n"
}

// Returns the value associated with the text area.
func (t *textAreaValue) Get() interface{} {
	return t.value
}

// Returns a string representation of the text area value, indicating whether it is configured or not.
func (t *textAreaValue) String() string {
	if t.value != nil && len(*t.value) > 0 {
		return fmt.Sprintf("%s: \t%s", t.desc, "(Select to Change or Update)")
	} else {
		return fmt.Sprintf("%s: \t%s", t.desc, "*** UNCONFIGURED ***")
	}
}
