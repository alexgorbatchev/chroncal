package tui

import (
	"context"
	"fmt"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
)

// handleTodosLoaded stores the loaded todos and places them for the current
// time. It drops a response whose range no longer matches the active view.
// A later load for the new range then replaces the todos.
func (m Model) handleTodosLoaded(msg todosLoadedMsg) (tea.Model, tea.Cmd) {
	from, to := m.expectedEventRange()
	if !msg.from.Equal(from) || !msg.to.Equal(to) {
		return m, nil
	}
	if msg.err != nil {
		m.err = fmt.Errorf("load todos: %w", msg.err)
		return m, nil
	}
	m.todos = msg.todos
	m.todoEntries = placeTodos(m.todos, time.Now())
	m = m.refreshCalendarViews()
	return m.refreshDayDialog(), nil
}

// placeTodosAt places the todos again for now. The clock tick calls it, so
// an open todo moves to the new today at midnight and a timed todo turns
// overdue at its DUE time. It refreshes the views only when a placement
// changed.
func (m Model) placeTodosAt(now time.Time) Model {
	entries := placeTodos(m.todos, now)
	if slices.EqualFunc(entries, m.todoEntries, sameTodoEntry) {
		return m
	}
	m.todoEntries = entries
	m = m.refreshCalendarViews()
	return m.refreshDayDialog()
}

// sameTodoEntry reports whether a and b show the same todo in the same way.
func sameTodoEntry(a, b todoEntry) bool {
	return a.todo.ID == b.todo.ID &&
		a.placement.Day.Equal(b.placement.Day) &&
		a.placement.At.Equal(b.placement.At) &&
		a.placement.Overdue == b.placement.Overdue
}

// visibleTodosOn returns the placed todos of visible calendars on day.
func (m Model) visibleTodosOn(day time.Time) []todoEntry {
	return todosOn(filterVisibleTodos(m.todoEntries, m.hiddenCalendars), day)
}

// refreshDayDialog gives the open day dialog the current events and todos
// of its day.
func (m Model) refreshDayDialog() Model {
	if !m.dialogOpen {
		return m
	}
	day := m.dialog.day
	m.dialog = m.dialog.
		SetEvents(eventsOn(filterVisibleEvents(m.events, m.hiddenCalendars), day)).
		SetTodos(m.visibleTodosOn(day))
	return m
}

// handleTodoViewRequested loads the current row of the todo and opens the
// todo view dialog. The requested todo can be an occurrence of a recurring
// todo. The dialog then shows the dates of that occurrence.
func (m Model) handleTodoViewRequested(msg TodoViewRequestedMsg) (tea.Model, tea.Cmd) {
	requested := msg.Todo
	return m, func() tea.Msg {
		ctx := context.Background()
		fresh, err := m.app.Todos.Get(ctx, requested.ID)
		if err != nil {
			return todoViewLoadedMsg{err: fmt.Errorf("load todo %d: %w", requested.ID, err)}
		}
		alarms, err := m.app.Todos.ListAlarms(ctx, requested.ID)
		if err != nil {
			return todoViewLoadedMsg{err: fmt.Errorf("load todo %d alarms: %w", requested.ID, err)}
		}
		fresh.Alarms = alarms
		if fresh.RecurrenceRule != "" && fresh.RecurrenceID == "" {
			fresh.DueDate = requested.DueDate
			fresh.StartDate = requested.StartDate
		}
		return todoViewLoadedMsg{todo: fresh}
	}
}

func (m Model) handleTodoViewLoaded(msg todoViewLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}
	p, _ := placeTodo(msg.todo, time.Now())
	m.todoView = NewTodoViewDialogModel(msg.todo, m.calendars[msg.todo.CalendarID], p.Overdue, m.theme).
		SetSize(m.width, m.height)
	m.todoViewOpen = true
	return m, nil
}
