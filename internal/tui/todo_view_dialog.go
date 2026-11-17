package tui

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/douglasdemoura/chroncal/internal/textsafe"
	"github.com/douglasdemoura/chroncal/internal/todo"
)

// TodoViewRequestedMsg requests the read-only todo view dialog. Todo holds
// the displayed occurrence, so a recurring todo shows the dates of that
// occurrence.
type TodoViewRequestedMsg struct{ Todo todo.Todo }

// TodoViewClosedMsg is emitted when the todo view dialog is dismissed.
type TodoViewClosedMsg struct{}

type todoViewKeyMap struct {
	Close      key.Binding
	ScrollUp   key.Binding
	ScrollDown key.Binding
	PageUp     key.Binding
	PageDown   key.Binding
	Home       key.Binding
	End        key.Binding
}

func defaultTodoViewKeys() todoViewKeyMap {
	return todoViewKeyMap{
		Close:      key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc", "close")),
		ScrollUp:   key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "scroll")),
		ScrollDown: key.NewBinding(key.WithKeys("down", "j")),
		PageUp:     key.NewBinding(key.WithKeys("pgup", "ctrl+b")),
		PageDown:   key.NewBinding(key.WithKeys("pgdown", "ctrl+f")),
		Home:       key.NewBinding(key.WithKeys("home")),
		End:        key.NewBinding(key.WithKeys("end")),
	}
}

// todoViewChromeLines is the count of fixed lines around the body and the
// title: the top and bottom border, the top padding, the faint rule below
// the title, and the help footer with its blank line at the start.
const todoViewChromeLines = 2 + 1 + 1 + 2

// TodoViewDialogModel is a read-only dialog that presents one todo.
type TodoViewDialogModel struct {
	todo     todo.Todo
	calendar CalendarInfo
	overdue  bool
	keys     todoViewKeyMap
	help     help.Model
	dialog   Dialog
	body     viewport.Model
	width    int
	height   int
	title    []string
}

// NewTodoViewDialogModel builds a read-only view of td. overdue marks the
// DUE row (see placeTodo).
func NewTodoViewDialogModel(td todo.Todo, cal CalendarInfo, overdue bool, theme Theme) TodoViewDialogModel {
	dialog := NewDialog("", DefaultDialogStyles())
	dialog.SetWidth(60)
	vp := viewport.New()
	vp.MouseWheelEnabled = true
	return TodoViewDialogModel{
		todo:     td,
		calendar: cal,
		overdue:  overdue,
		keys:     defaultTodoViewKeys(),
		help:     newThemedHelp(theme),
		dialog:   dialog,
		body:     vp,
	}
}

// Todo returns the todo the dialog shows.
func (m TodoViewDialogModel) Todo() todo.Todo { return m.todo }

func (m TodoViewDialogModel) SetSize(w, h int) TodoViewDialogModel {
	m.width = w
	m.height = h
	m.dialog = m.dialog.Update(tea.WindowSizeMsg{Width: w, Height: h})
	cw := m.dialog.ContentWidth()
	m.body.SetWidth(cw)
	if cw <= 0 {
		m.title = nil
		return m
	}
	m.title = m.titleLines(cw)
	lines := m.bodyLines(cw)
	m.body.SetHeight(min(len(lines), m.viewportHeight(len(m.title))))
	m.body.SetContentLines(lines)
	return m
}

func (m TodoViewDialogModel) viewportHeight(titleH int) int {
	return max(m.height-todoViewChromeLines-titleH, 1)
}

func (m TodoViewDialogModel) titleLines(w int) []string {
	avail := m.height - todoViewChromeLines
	return wrapTitleLines(textsafe.Display(m.todo.Summary), w, max(avail/2, 1))
}

// bodyLines returns the rows below the title. A blank row at the start
// keeps a gap below the title rule, as in the event view dialog.
func (m TodoViewDialogModel) bodyLines(w int) []string {
	return append([]string{""}, todoDetailLines(m.todo, m.calendar, m.overdue, w, eventViewLabelWidth, true)...)
}

func (m TodoViewDialogModel) BoxSize() (int, int) {
	if m.width <= 0 || m.height <= 0 {
		return 0, 0
	}
	return lipgloss.Size(m.View())
}

func (m TodoViewDialogModel) Update(msg tea.Msg) (TodoViewDialogModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.SetSize(msg.Width, msg.Height), nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.MouseClickMsg:
		return m.handleMouse(msg)
	case tea.MouseWheelMsg:
		var cmd tea.Cmd
		m.body, cmd = m.body.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m TodoViewDialogModel) handleKey(msg tea.KeyPressMsg) (TodoViewDialogModel, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Close):
		return m, func() tea.Msg { return TodoViewClosedMsg{} }
	case key.Matches(msg, m.keys.ScrollUp):
		m.body.ScrollUp(1)
	case key.Matches(msg, m.keys.ScrollDown):
		m.body.ScrollDown(1)
	case key.Matches(msg, m.keys.PageUp):
		m.body.PageUp()
	case key.Matches(msg, m.keys.PageDown):
		m.body.PageDown()
	case key.Matches(msg, m.keys.Home):
		m.body.GotoTop()
	case key.Matches(msg, m.keys.End):
		m.body.GotoBottom()
	}
	return m, nil
}

// handleMouse opens a clicked link. The dialog has no other click target.
func (m TodoViewDialogModel) handleMouse(msg tea.MouseClickMsg) (TodoViewDialogModel, tea.Cmd) {
	if msg.Button != tea.MouseLeft {
		return m, nil
	}
	bw, bh := m.BoxSize()
	target := mouseResolve(msg.X-(m.width-bw)/2, msg.Y-(m.height-bh)/2)
	if strings.HasPrefix(target, linkZonePrefix) {
		return m, openURLCmd(target[len(linkZonePrefix):])
	}
	return m, nil
}

func (m TodoViewDialogModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	cw := m.dialog.ContentWidth()
	if cw <= 0 {
		return ""
	}
	if m.title == nil {
		m.title = m.titleLines(cw)
	}
	lines := m.bodyLines(cw)
	m.body.SetWidth(cw)
	m.body.SetHeight(min(len(lines), m.viewportHeight(len(m.title))))
	m.body.SetContentLines(lines)

	m.help.SetWidth(cw)
	m.dialog.SetFooter(m.help.ShortHelpView([]key.Binding{m.keys.ScrollUp, m.keys.Close}))
	return mouseSweep(m.dialog.Box(paneTitle(m.title, cw) + "\n" + m.body.View()))
}
