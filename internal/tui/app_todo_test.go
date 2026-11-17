package tui

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/douglasdemoura/chroncal/internal/todo"
)

// loadViews runs the events and todos load of the active view through
// Model.Update.
func loadViews(t *testing.T, m Model) Model {
	t.Helper()
	batch, ok := m.loadEvents()().(tea.BatchMsg)
	require.True(t, ok, "loadEvents must batch the events and the todos queries")
	m, _ = drainBatch(t, m, batch)
	return m
}

func todayLocal() time.Time {
	return dayIn(time.Now(), time.Local)
}

func calendarTodoItems(items []CalendarEvent) []CalendarEvent {
	var out []CalendarEvent
	for _, it := range items {
		if it.Kind == calendarItemTodo {
			out = append(out, it)
		}
	}
	return out
}

func TestTodosLoad_ShowsOpenTodoOnToday(t *testing.T) {
	m, a := newDBBackedModel(t)
	created, err := a.Todos.Create(context.Background(), todo.CreateParams{CalendarID: 1, Summary: "Water plants"})
	require.NoError(t, err)

	m = loadViews(t, m)

	require.Len(t, m.todoEntries, 1)
	require.True(t, m.todoEntries[0].placement.Day.Equal(todayLocal()))
	items := calendarTodoItems(m.calendar.events)
	require.Len(t, items, 1, "the month view must show the todo")
	require.Equal(t, created.ID, items[0].ID)
	require.Equal(t, "Water plants", items[0].Title)
}

func TestTodosLoad_DropsStaleRange(t *testing.T) {
	m, _ := newDBBackedModel(t)
	from, to := m.expectedEventRange()

	updated, _ := m.Update(todosLoadedMsg{
		from:  from.AddDate(0, -1, 0),
		to:    to.AddDate(0, -1, 0),
		todos: []todo.Todo{{ID: 9, CalendarID: 1, Summary: "Stale", Status: "NEEDS-ACTION"}},
	})
	m = updated.(Model)
	require.Empty(t, m.todos, "a response for an old range must not replace the todos")

	updated, _ = m.Update(todosLoadedMsg{
		from:  from,
		to:    to,
		todos: []todo.Todo{{ID: 9, CalendarID: 1, Summary: "Current", Status: "NEEDS-ACTION"}},
	})
	m = updated.(Model)
	require.Len(t, m.todos, 1)
}

func TestTodos_HiddenCalendarHidesTodos(t *testing.T) {
	m, a := newDBBackedModel(t)
	_, err := a.Todos.Create(context.Background(), todo.CreateParams{CalendarID: 1, Summary: "Secret"})
	require.NoError(t, err)
	m = loadViews(t, m)
	require.Len(t, calendarTodoItems(m.calendar.events), 1)

	updated, _ := m.Update(CalendarVisibilityToggledMsg{ID: 1, Hidden: true})
	m = updated.(Model)

	require.Empty(t, calendarTodoItems(m.calendar.events))
	require.Empty(t, m.agenda.todos)
	require.Empty(t, m.visibleTodosOn(todayLocal()))
}

func TestTodos_AgendaEnterOpensTodoView(t *testing.T) {
	m, a := newDBBackedModel(t)
	_, err := a.Todos.Create(context.Background(), todo.CreateParams{CalendarID: 1, Summary: "Call the bank", Priority: 1})
	require.NoError(t, err)
	m, _ = step(t, m, keyPressMsg("a"))
	require.Equal(t, viewAgenda, m.viewMode)
	m = loadViews(t, m)

	idx := -1
	for i, r := range m.agenda.rows {
		if r.todo != nil {
			idx = i
		}
	}
	require.GreaterOrEqual(t, idx, 0, "the agenda must have a todo row")
	m.agenda.selected = idx

	m, next := step(t, m, keyPressMsg("enter"))
	req, ok := next.(TodoViewRequestedMsg)
	require.True(t, ok, "enter on a todo row must request the todo view, got %T", next)
	require.Equal(t, "Call the bank", req.Todo.Summary)

	m, next = step(t, m, req)
	require.IsType(t, todoViewLoadedMsg{}, next)
	m, _ = step(t, m, next)
	require.True(t, m.todoViewOpen)
	require.Equal(t, "Call the bank", m.todoView.Todo().Summary)
	require.Contains(t, m.View().Content, "Call the bank")

	m, next = step(t, m, keyPressMsg("esc"))
	require.IsType(t, TodoViewClosedMsg{}, next)
	m, _ = step(t, m, next)
	require.False(t, m.todoViewOpen)
}

func TestTodos_DayDialogListsTodosAfterEvents(t *testing.T) {
	m, a := newDBBackedModel(t)
	_, err := a.Todos.Create(context.Background(), todo.CreateParams{CalendarID: 1, Summary: "Buy milk"})
	require.NoError(t, err)
	m = loadViews(t, m)

	m, _ = step(t, m, CalendarDaySelectedMsg{Day: todayLocal()})

	require.True(t, m.dialogOpen)
	require.Len(t, m.dialog.todos, 1)
	require.Equal(t, todoGlyph(false)+" Buy milk", m.dialog.eventLabels[len(m.dialog.events)])
	m.dialog.shell = m.dialog.shell.SetSelected(len(m.dialog.events))
	m.dialog = m.dialog.refresh()
	e, ok := m.dialog.selectedTodo()
	require.True(t, ok)
	require.Equal(t, "Buy milk", e.todo.Summary)
	require.Equal(t, "Buy milk", m.dialog.shell.detailTitle, "the detail pane must show the selected todo")
	require.Contains(t, strippedLines(m.dialog.shell.detailLines), "    Status   Needs action")
}

func TestTodos_ClickOnTodoItemOpensTodoView(t *testing.T) {
	m, a := newDBBackedModel(t)
	created, err := a.Todos.Create(context.Background(), todo.CreateParams{CalendarID: 1, Summary: "Clicked"})
	require.NoError(t, err)
	m = loadViews(t, m)

	m.clickedItem = calendarItemRef{Kind: calendarItemTodo, ID: created.ID}
	m, next := step(t, m, CalendarDaySelectedMsg{Day: todayLocal()})

	req, ok := next.(TodoViewRequestedMsg)
	require.True(t, ok, "a click on a todo must request the todo view, got %T", next)
	require.Equal(t, created.ID, req.Todo.ID)
	require.False(t, m.dialogOpen, "a click on a todo must not open the day dialog")
	require.Zero(t, m.clickedItem)
}

func TestTodos_RecurringOccurrenceViewKeepsOccurrenceDates(t *testing.T) {
	m, a := newDBBackedModel(t)
	ctx := context.Background()
	master, err := a.Todos.Create(ctx, todo.CreateParams{
		CalendarID: 1, Summary: "Weekly review", DueDate: "2026-01-05", RecurrenceRule: "FREQ=WEEKLY",
	})
	require.NoError(t, err)
	_, err = a.Todos.UpsertByUID(ctx, todo.UpsertParams{
		UID: master.UID, CalendarID: 1, Summary: "Weekly review",
		DueDate: "2026-01-05", RecurrenceID: "2026-01-05", Status: "COMPLETED",
	})
	require.NoError(t, err)
	m = loadViews(t, m)
	require.Len(t, m.todos, 1)
	require.Equal(t, "2026-01-12", m.todos[0].DueDate, "the current occurrence follows the completed one")

	m, next := step(t, m, TodoViewRequestedMsg{Todo: m.todos[0]})
	loaded, ok := next.(todoViewLoadedMsg)
	require.True(t, ok, "expected todoViewLoadedMsg, got %T", next)
	require.NoError(t, loaded.err)
	require.Equal(t, master.ID, loaded.todo.ID)
	require.Equal(t, "2026-01-12", loaded.todo.DueDate)
	_, _ = step(t, m, next)
}

func TestTodos_ClockTickMovesTodoToNewDay(t *testing.T) {
	m, _ := newDBBackedModel(t)
	m.todos = []todo.Todo{{ID: 1, CalendarID: 1, Summary: "Undated", Status: "NEEDS-ACTION"}}
	day1 := time.Date(2026, 10, 16, 23, 59, 0, 0, time.Local)
	day2 := time.Date(2026, 10, 17, 0, 1, 0, 0, time.Local)

	m = m.placeTodosAt(day1)
	require.True(t, calendarTodoItems(m.calendar.events)[0].Day.Equal(dayIn(day1, time.Local)))

	m = m.placeTodosAt(day2)
	require.True(t, calendarTodoItems(m.calendar.events)[0].Day.Equal(dayIn(day2, time.Local)),
		"an undated todo must move to the new today at midnight")
}
